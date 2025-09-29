package agent

import (
	"GPUConductor/internal/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/host"
)

type Config struct {
	ServerURL string
	NodeName  string
	Tags      []string
}

type Agent struct {
	config      *Config
	nodeID      string
	redisClient *redis.Client
	ctx         context.Context
	cancel      context.CancelFunc
}

func New(config *Config) *Agent {
	ctx, cancel := context.WithCancel(context.Background())

	return &Agent{
		config: config,
		nodeID: uuid.New().String(),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (a *Agent) Start() error {
	// 连接Redis (从服务器获取配置)
	redisAddr, err := a.getRedisConfig()
	if err != nil {
		log.Printf("获取Redis配置失败，使用默认配置: %v", err)
		redisAddr = "localhost:6379"
	}

	a.redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 启动各个组件
	go a.heartbeatLoop()
	go a.gpuMonitorLoop()
	go a.taskListener()

	log.Printf("Agent节点启动成功: %s", a.config.NodeName)

	// 阻塞主线程
	<-a.ctx.Done()
	return nil
}

func (a *Agent) Stop() {
	a.cancel()
	if a.redisClient != nil {
		a.redisClient.Close()
	}
}

// getRedisConfig 从服务器获取Redis配置
func (a *Agent) getRedisConfig() (string, error) {
	resp, err := http.Get(a.config.ServerURL + "/api/v1/config/redis")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var config struct {
		Redis string `json:"redis"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return "", err
	}

	return config.Redis, nil
}

// heartbeatLoop 心跳循环
func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat() {
	hostInfo, _ := host.Info()

	heartbeat := map[string]interface{}{
		"name":    a.config.NodeName,
		"address": a.getLocalIP(),
		"tags":    a.config.Tags,
		"status":  "online",
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"host":    hostInfo.Hostname,
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		log.Printf("序列化心跳数据失败: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/nodes/%s/heartbeat", a.config.ServerURL, a.nodeID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("发送心跳失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("心跳响应错误: %d", resp.StatusCode)
	}
}

// getLocalIP 获取本地IP地址
func (a *Agent) getLocalIP() string {
	// 简单实现，实际可能需要更复杂的逻辑
	return "127.0.0.1"
}

// gpuMonitorLoop GPU监控循环
func (a *Agent) gpuMonitorLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.updateGPUInfo()
		}
	}
}

// updateGPUInfo 更新GPU信息
func (a *Agent) updateGPUInfo() {
	gpus, err := a.getGPUInfo()
	if err != nil {
		log.Printf("获取GPU信息失败: %v", err)
		return
	}

	data, err := json.Marshal(gpus)
	if err != nil {
		log.Printf("序列化GPU数据失败: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/nodes/%s/gpus", a.config.ServerURL, a.nodeID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("创建GPU更新请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("更新GPU信息失败: %v", err)
		return
	}
	defer resp.Body.Close()
}

// getGPUInfo 获取GPU信息
func (a *Agent) getGPUInfo() ([]models.GPU, error) {
	// 使用nvidia-smi获取GPU信息
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,name,memory.total,memory.used,utilization.gpu,temperature.gpu,power.draw", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行nvidia-smi失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	gpus := make([]models.GPU, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Split(line, ", ")
		if len(fields) < 7 {
			continue
		}

		index, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		name := strings.TrimSpace(fields[1])
		memoryTotal, _ := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		memoryUsed, _ := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64)
		utilization, _ := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
		temperature, _ := strconv.Atoi(strings.TrimSpace(fields[5]))
		powerUsage, _ := strconv.Atoi(strings.TrimSpace(fields[6]))

		gpu := models.GPU{
			Index:       index,
			Name:        name,
			Memory:      memoryTotal,
			MemoryUsed:  memoryUsed,
			Utilization: utilization,
			Temperature: temperature,
			PowerUsage:  powerUsage,
			UpdatedAt:   time.Now(),
		}

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}

// taskListener 任务监听器
func (a *Agent) taskListener() {
	channel := fmt.Sprintf("node:%s:tasks", a.nodeID)
	pubsub := a.redisClient.Subscribe(a.ctx, channel)
	defer pubsub.Close()

	for {
		select {
		case <-a.ctx.Done():
			return
		case msg := <-pubsub.Channel():
			var task models.Task
			if err := json.Unmarshal([]byte(msg.Payload), &task); err != nil {
				log.Printf("解析任务数据失败: %v", err)
				continue
			}

			go a.executeTask(&task)
		}
	}
}

// executeTask 执行任务
func (a *Agent) executeTask(task *models.Task) {
	log.Printf("开始执行任务: %s", task.Name)

	// 暂时使用简化的任务执行（模拟）
	a.updateTaskStatus(task, "running")

	// 模拟任务执行
	go func() {
		// 模拟执行时间
		time.Sleep(10 * time.Second)

		// 模拟成功完成
		exitCode := 0
		task.ExitCode = &exitCode
		a.updateTaskStatus(task, "completed")
		log.Printf("任务完成: %s", task.Name)
	}()
}

// createContainer 创建容器 (暂时禁用)
func (a *Agent) createContainer(task *models.Task) (string, error) {
	// 暂时返回模拟的容器ID
	return "mock-container-id", nil
}

// buildEnvVars 构建环境变量
func (a *Agent) buildEnvVars(env map[string]string) []string {
	vars := make([]string, 0, len(env))
	for k, v := range env {
		vars = append(vars, fmt.Sprintf("%s=%s", k, v))
	}
	return vars
}

// updateTaskStatus 更新任务状态
func (a *Agent) updateTaskStatus(task *models.Task, status string) {
	task.Status = status
	now := time.Now()

	if status == "completed" || status == "failed" {
		task.CompletedAt = &now
	}

	data, err := json.Marshal(task)
	if err != nil {
		log.Printf("序列化任务状态失败: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/tasks/%s", a.config.ServerURL, task.ID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("创建任务更新请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("更新任务状态失败: %v", err)
		return
	}
	defer resp.Body.Close()
}

// reportTaskError 报告任务错误
func (a *Agent) reportTaskError(task *models.Task, errorMsg string) {
	task.Status = "failed"
	task.ErrorMessage = errorMsg
	now := time.Now()
	task.CompletedAt = &now

	a.updateTaskStatus(task, "failed")
	log.Printf("任务错误: %s - %s", task.Name, errorMsg)
}
