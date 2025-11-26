package scheduler

import (
	"GPUConductor/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type Scheduler struct {
	db          *gorm.DB
	redisClient *redis.Client
	ctx         context.Context
	cancel      context.CancelFunc
}

func New(db *gorm.DB, redisOpts *redis.Options) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	if redisOpts == nil {
		redisOpts = &redis.Options{Addr: "localhost:6379"}
	}
	rdb := redis.NewClient(redisOpts)

	return &Scheduler{
		db:          db,
		redisClient: rdb,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (s *Scheduler) Start() {
	log.Println("任务调度器启动")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			log.Println("任务调度器停止")
			return
		case <-ticker.C:
			s.scheduleTasks()
			s.checkRunningTasks()
		}
	}
}

func (s *Scheduler) Stop() {
	s.cancel()
}

// SubmitTask 提交新任务
func (s *Scheduler) SubmitTask(task *models.Task) error {
	task.Status = "pending"
	if err := s.db.Create(task).Error; err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	// 发布任务事件
	s.publishTaskEvent(task, "submitted")

	log.Printf("任务已提交: %s (%s)", task.Name, task.ID)
	return nil
}

// CancelTask 取消任务
func (s *Scheduler) CancelTask(taskID string) error {
	var task models.Task
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status == "running" {
		// 停止正在运行的容器
		if err := s.stopTaskContainer(&task); err != nil {
			log.Printf("停止任务容器失败: %v", err)
		}
	}

	task.Status = "cancelled"
	now := time.Now()
	task.CompletedAt = &now

	if err := s.db.Save(&task).Error; err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	s.publishTaskEvent(&task, "cancelled")
	log.Printf("任务已取消: %s (%s)", task.Name, task.ID)
	return nil
}

// scheduleTasks 调度待执行任务
func (s *Scheduler) scheduleTasks() {
	var pendingTasks []models.Task
	if err := s.db.Where("status = ?", "pending").
		Order("priority DESC, submitted_at ASC").
		Find(&pendingTasks).Error; err != nil {
		log.Printf("查询待执行任务失败: %v", err)
		return
	}

	for _, task := range pendingTasks {
		if node := s.findAvailableNode(&task); node != nil {
			if err := s.assignTaskToNode(&task, node); err != nil {
				log.Printf("分配任务到节点失败: %v", err)
				continue
			}
		}
	}
}

// findAvailableNode 查找可用节点
func (s *Scheduler) findAvailableNode(task *models.Task) *models.Node {
	query := s.db.Where("status = ?", "online")

	// 如果指定了节点ID
	if task.NodeID != "" {
		query = query.Where("id = ?", task.NodeID)
	}

	var nodes []models.Node
	if err := query.Preload("GPUs").Find(&nodes).Error; err != nil {
		log.Printf("查询节点失败: %v", err)
		return nil
	}

	for _, node := range nodes {
		if len(task.NodeTags) > 0 && !nodeHasTags(node.Tags, task.NodeTags) {
			continue
		}

		if s.hasEnoughGPUResources(&node, task) {
			return &node
		}
	}

	return nil
}

// hasEnoughGPUResources 检查节点是否有足够的GPU资源
func (s *Scheduler) hasEnoughGPUResources(node *models.Node, task *models.Task) bool {
	availableGPUs := 0

	for _, gpu := range node.GPUs {
		// 检查GPU使用率和内存
		if gpu.Utilization < 80 && // 使用率小于80%
			(task.GPUMemory == 0 || gpu.Memory-gpu.MemoryUsed >= task.GPUMemory) {
			availableGPUs++
		}
	}

	return availableGPUs >= task.GPUCount
}

// assignTaskToNode 将任务分配给节点
func (s *Scheduler) assignTaskToNode(task *models.Task, node *models.Node) error {
	task.Status = "running"
	task.AssignedNodeID = node.Address
	now := time.Now()
	task.StartedAt = &now

	if err := s.db.Save(task).Error; err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 发送任务到节点执行
	if err := s.sendTaskToNode(task, node); err != nil {
		// 如果发送失败，回滚任务状态
		task.Status = "pending"
		task.AssignedNodeID = ""
		task.StartedAt = nil
		s.db.Save(task)
		return fmt.Errorf("发送任务到节点失败: %w", err)
	}

	s.publishTaskEvent(task, "started")
	log.Printf("任务已分配: %s -> %s", task.Name, node.Name)
	return nil
}

// sendTaskToNode 发送任务到节点执行
func (s *Scheduler) sendTaskToNode(task *models.Task, node *models.Node) error {
	taskData, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// 通过Redis发布任务
	channel := fmt.Sprintf("node:%s:tasks", node.ID)
	return s.redisClient.Publish(s.ctx, channel, taskData).Err()
}

// checkRunningTasks 检查运行中的任务
func (s *Scheduler) checkRunningTasks() {
	var runningTasks []models.Task
	if err := s.db.Where("status = ?", "running").Find(&runningTasks).Error; err != nil {
		log.Printf("查询运行中任务失败: %v", err)
		return
	}

	for _, task := range runningTasks {
		// 检查任务是否超时
		if task.MaxDuration > 0 && task.StartedAt != nil {
			elapsed := time.Since(*task.StartedAt)
			if elapsed > time.Duration(task.MaxDuration)*time.Minute {
				log.Printf("任务超时，正在取消: %s", task.Name)
				s.CancelTask(task.ID)
			}
		}
	}
}

// stopTaskContainer 停止任务容器
func (s *Scheduler) stopTaskContainer(task *models.Task) error {
	if task.ContainerID == "" {
		return nil
	}

	// 发送停止命令到节点
	stopCmd := map[string]interface{}{
		"action":       "stop",
		"container_id": task.ContainerID,
	}

	cmdData, err := json.Marshal(stopCmd)
	if err != nil {
		return err
	}

	channel := fmt.Sprintf("node:%s:commands", task.AssignedNodeID)
	return s.redisClient.Publish(s.ctx, channel, cmdData).Err()
}

// publishTaskEvent 发布任务事件
func (s *Scheduler) publishTaskEvent(task *models.Task, event string) {
	eventData := map[string]interface{}{
		"event": event,
		"task":  task,
	}

	data, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("序列化任务事件失败: %v", err)
		return
	}

	s.redisClient.Publish(s.ctx, "task_events", data)
}

func nodeHasTags(nodeTags []string, required []string) bool {
	if len(required) == 0 {
		return true
	}

	tagSet := make(map[string]struct{}, len(nodeTags))
	for _, tag := range nodeTags {
		tagSet[strings.TrimSpace(tag)] = struct{}{}
	}

	for _, tag := range required {
		if _, ok := tagSet[strings.TrimSpace(tag)]; !ok {
			return false
		}
	}

	return true
}
