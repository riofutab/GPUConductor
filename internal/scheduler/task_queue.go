package scheduler

import (
	"GPUConductor/internal/models"
	"container/heap"
	"sync"
	"time"
)

// TaskQueue 任务队列实现
type TaskQueue struct {
	mu    sync.RWMutex
	tasks []*models.Task
}

// 实现heap.Interface接口
func (q *TaskQueue) Len() int { 
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks) 
}

func (q *TaskQueue) Less(i, j int) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	// 优先级高的排在前面，相同优先级按提交时间排序
	if q.tasks[i].Priority != q.tasks[j].Priority {
		return q.tasks[i].Priority > q.tasks[j].Priority
	}
	return q.tasks[i].SubmittedAt.Before(q.tasks[j].SubmittedAt)
}

func (q *TaskQueue) Swap(i, j int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks[i], q.tasks[j] = q.tasks[j], q.tasks[i]
}

func (q *TaskQueue) Push(x interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	task := x.(*models.Task)
	task.QueuePosition = len(q.tasks) + 1
	q.tasks = append(q.tasks, task)
}

func (q *TaskQueue) Pop() interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.tasks) == 0 {
		return nil
	}
	task := q.tasks[len(q.tasks)-1]
	q.tasks = q.tasks[:len(q.tasks)-1]
	q.updateQueuePositions()
	return task
}

// NewTaskQueue 创建任务队列
func NewTaskQueue() *TaskQueue {
	return &TaskQueue{
		tasks: make([]*models.Task, 0),
	}
}

// PushTask 添加任务到队列（对外接口）
func (q *TaskQueue) PushTask(task *models.Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.QueuePosition = len(q.tasks) + 1
	q.tasks = append(q.tasks, task)
	heap.Push(q, task)
}

// PopTask 从队列中取出最高优先级的任务（对外接口）
func (q *TaskQueue) PopTask() *models.Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return nil
	}

	task := heap.Pop(q).(*models.Task)
	q.updateQueuePositions()
	return task
}

// Peek 查看队列头部的任务
func (q *TaskQueue) Peek() *models.Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.tasks) == 0 {
		return nil
	}
	return q.tasks[0]
}

// Remove 从队列中移除任务
func (q *TaskQueue) Remove(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, task := range q.tasks {
		if task.ID == taskID {
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			q.updateQueuePositions()
			return true
		}
	}
	return false
}

// GetTasks 获取队列中的所有任务
func (q *TaskQueue) GetTasks() []*models.Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*models.Task, len(q.tasks))
	copy(tasks, q.tasks)
	return tasks
}



// sortTasks 根据优先级排序任务
func (q *TaskQueue) sortTasks() {
	heap.Init(q)
}

// updateQueuePositions 更新队列位置
func (q *TaskQueue) updateQueuePositions() {
	for i, task := range q.tasks {
		task.QueuePosition = i + 1
	}
}



// TaskManager 任务管理器
type TaskManager struct {
	queue     *TaskQueue
	running   map[string]*models.Task
	completed map[string]*models.Task
	mu        sync.RWMutex
}

// NewTaskManager 创建任务管理器
func NewTaskManager() *TaskManager {
	return &TaskManager{
		queue:     NewTaskQueue(),
		running:   make(map[string]*models.Task),
		completed: make(map[string]*models.Task),
	}
}

// SubmitTask 提交任务
func (m *TaskManager) SubmitTask(task *models.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task.Status = "pending"
	task.SubmittedAt = time.Now()
	m.queue.PushTask(task)
}

// StartTask 开始执行任务
func (m *TaskManager) StartTask(task *models.Task) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查任务是否还在队列中
	if !m.queue.Remove(task.ID) {
		return false
	}

	task.Status = "running"
	startTime := time.Now()
	task.StartedAt = &startTime
	m.running[task.ID] = task
	return true
}

// CompleteTask 完成任务
func (m *TaskManager) CompleteTask(taskID string, status string, errorMessage string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.running[taskID]; exists {
		task.Status = status
		completedTime := time.Now()
		task.CompletedAt = &completedTime
		task.ErrorMessage = errorMessage
		m.completed[taskID] = task
		delete(m.running, taskID)
	}
}

// CancelTask 取消任务
func (m *TaskManager) CancelTask(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 尝试从队列中取消
	if m.queue.Remove(taskID) {
		return true
	}

	// 尝试从运行中取消
	if task, exists := m.running[taskID]; exists {
		task.Status = "cancelled"
		cancelledTime := time.Now()
		task.CompletedAt = &cancelledTime
		m.completed[taskID] = task
		delete(m.running, taskID)
		return true
	}

	return false
}

// GetPendingTasks 获取等待中的任务
func (m *TaskManager) GetPendingTasks() []*models.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queue.GetTasks()
}

// GetRunningTasks 获取运行中的任务
func (m *TaskManager) GetRunningTasks() []*models.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*models.Task, 0, len(m.running))
	for _, task := range m.running {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetCompletedTasks 获取已完成的任务
func (m *TaskManager) GetCompletedTasks() []*models.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*models.Task, 0, len(m.completed))
	for _, task := range m.completed {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetTaskByID 根据ID获取任务
func (m *TaskManager) GetTaskByID(taskID string) *models.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查运行中的任务
	if task, exists := m.running[taskID]; exists {
		return task
	}

	// 检查已完成的任务
	if task, exists := m.completed[taskID]; exists {
		return task
	}

	// 检查队列中的任务
	for _, task := range m.queue.GetTasks() {
		if task.ID == taskID {
			return task
		}
	}

	return nil
}

// CleanupExpiredTasks 清理过期任务
func (m *TaskManager) CleanupExpiredTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expiredTime := now.Add(-24 * time.Hour) // 保留24小时内的任务

	// 清理过期的已完成任务
	for taskID, task := range m.completed {
		if task.CompletedAt != nil && task.CompletedAt.Before(expiredTime) {
			delete(m.completed, taskID)
		}
	}
}