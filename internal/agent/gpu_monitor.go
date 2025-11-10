package agent

import (
	"GPUConductor/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GPUInfo 从nvidia-smi获取的GPU信息
type GPUInfo struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	Memory      int64   `json:"memory"`      // 总内存 MB
	MemoryUsed  int64   `json:"memory_used"` // 已使用内存 MB
	Utilization float64 `json:"utilization"` // 使用率 0-100
	Temperature int     `json:"temperature"` // 温度
	PowerUsage  int     `json:"power_usage"` // 功耗 W
}

// GPUMonitor GPU监控器
type GPUMonitor struct {
	agent *Agent
}

// NewGPUMonitor 创建GPU监控器
func NewGPUMonitor(agent *Agent) *GPUMonitor {
	return &GPUMonitor{
		agent: agent,
	}
}

// Start 启动GPU监控
func (m *GPUMonitor) Start() {
	log.Println("GPU监控器启动")

	interval := m.agent.config.GPU.MonitorInterval
	if interval <= 0 {
		log.Println("GPU监控器未配置监控周期，退出")
		return
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.agent.ctx.Done():
			log.Println("GPU监控器停止")
			return
		case <-ticker.C:
			m.updateGPUStats()
		}
	}
}

// updateGPUStats 更新GPU统计信息
func (m *GPUMonitor) updateGPUStats() {
	gpus, err := m.getGPUInfo()
	if err != nil {
		log.Printf("获取GPU信息失败: %v", err)
		return
	}

	// 更新节点状态
	m.agent.node.Status = "online"
	m.agent.node.LastSeen = time.Now()

	// 更新GPU信息
	var gpuModels []models.GPU
	for _, gpu := range gpus {
		gpuModels = append(gpuModels, models.GPU{
			Index:       gpu.Index,
			Name:        gpu.Name,
			Memory:      gpu.Memory,
			MemoryUsed:  gpu.MemoryUsed,
			Utilization: gpu.Utilization,
			Temperature: gpu.Temperature,
			PowerUsage:  gpu.PowerUsage,
			UpdatedAt:   time.Now(),
		})
	}

	m.agent.node.GPUs = gpuModels

	// 发布GPU状态更新
	m.publishGPUStats()
}

// getGPUInfo 获取GPU信息
func (m *GPUMonitor) getGPUInfo() ([]GPUInfo, error) {
	cmd := exec.Command(m.agent.config.GPU.NvidiaSMIPath,
		"--query-gpu=index,name,memory.total,memory.used,utilization.gpu,temperature.gpu,power.draw",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行nvidia-smi失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var gpus []GPUInfo

	for _, line := range lines {
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

		gpus = append(gpus, GPUInfo{
			Index:       index,
			Name:        name,
			Memory:      memoryTotal,
			MemoryUsed:  memoryUsed,
			Utilization: utilization,
			Temperature: temperature,
			PowerUsage:  powerUsage,
		})
	}

	return gpus, nil
}

// publishGPUStats 发布GPU状态更新
func (m *GPUMonitor) publishGPUStats() {
	stats := map[string]interface{}{
		"node_id":   m.agent.node.ID,
		"gpus":      m.agent.node.GPUs,
		"status":    m.agent.node.Status,
		"timestamp": time.Now().Unix(),
	}

	data, err := json.Marshal(stats)
	if err != nil {
		log.Printf("序列化GPU状态失败: %v", err)
		return
	}

	// 发布到Redis
	m.agent.redisClient.Publish(m.agent.ctx, "gpu_stats", data)
}

// CheckGPUResources 检查GPU资源是否充足
func (m *GPUMonitor) CheckGPUResources(gpuCount int, gpuMemory int64) bool {
	availableGPUs := 0

	for _, gpu := range m.agent.node.GPUs {
		// 检查GPU使用率和内存
		if gpu.Utilization < float64(m.agent.config.GPU.UtilizationThreshold) &&
			(gpuMemory == 0 || (gpu.Memory-gpu.MemoryUsed) >= gpuMemory) {
			availableGPUs++
		}
	}

	return availableGPUs >= gpuCount
}

// GetAvailableGPUs 获取可用GPU列表
func (m *GPUMonitor) GetAvailableGPUs() []models.GPU {
	var availableGPUs []models.GPU

	for _, gpu := range m.agent.node.GPUs {
		if gpu.Utilization < float64(m.agent.config.GPU.UtilizationThreshold) {
			availableGPUs = append(availableGPUs, gpu)
		}
	}

	return availableGPUs
}
