package agent

import (
	"GPUConductor/internal/config"
	"GPUConductor/internal/docker"
	"GPUConductor/internal/models"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/host"
)

type Config struct {
	ServerURL string
	NodeName  string
	Tags      []string
	GPU       GPUConfig
}

type GPUConfig struct {
	NvidiaSMIPath        string
	MonitorInterval      int
	UtilizationThreshold int
}

type Agent struct {
	config      *config.Config
	nodeID      string
	node        *models.Node
	redisClient *redis.Client
	dockerMgr   *docker.DockerManager
	ctx         context.Context
	cancel      context.CancelFunc
	gpuMu       sync.Mutex
	gpuInUse    map[string]bool
}

const defaultS3Region = "us-east-1"

func New(config *config.Config) *Agent {
	ctx, cancel := context.WithCancel(context.Background())

	nodeID := loadOrCreateNodeID()
	node := &models.Node{
		ID:   nodeID,
		Name: config.Node.NodeName,
	}

	return &Agent{
		config:   config,
		nodeID:   node.ID,
		node:     node,
		ctx:      ctx,
		cancel:   cancel,
		gpuInUse: make(map[string]bool),
	}
}

func (a *Agent) Start() error {
	if a.config.GPU.MonitorInterval <= 0 {
		a.config.GPU.MonitorInterval = 5
	}
	if a.config.GPU.NvidiaSMIPath == "" {
		a.config.GPU.NvidiaSMIPath = "nvidia-smi"
	}
	if a.config.GPU.UtilizationThreshold <= 0 {
		a.config.GPU.UtilizationThreshold = 80
	}

	dm, err := docker.NewDockerManager()
	if err != nil {
		return fmt.Errorf("初始化Docker失败: %w", err)
	}
	a.dockerMgr = dm

	// 连接Redis (从服务器获取配置)
	redisAddr, redisPass, redisDB, err := a.getRedisConfig()
	if err != nil {
		log.Printf("获取Redis配置失败，使用默认配置: %v", err)
		redisAddr = "localhost:6379"
	}

	if a.redisClient == nil {
		a.redisClient = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: redisPass,
			DB:       redisDB,
		})
	}

	// 启动各个组件
	go a.heartbeatLoop()
	go a.gpuMonitorLoop()
	go a.taskListener()
	go a.cleanupLoop()

	log.Printf("Agent节点启动成功: %s", a.config.NodeName())

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

func (a *Agent) cleanupLoop() {
	if a.dockerMgr == nil {
		return
	}

	// 先执行一次，避免遗留容器
	if err := a.dockerMgr.CleanupContainers(); err != nil {
		log.Printf("清理容器失败: %v", err)
	}

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.dockerMgr.CleanupContainers(); err != nil {
				log.Printf("定时清理容器失败: %v", err)
			}
		}
	}
}

// getRedisConfig 从服务器获取Redis配置
func (a *Agent) getRedisConfig() (string, string, int, error) {
	resp, err := http.Get(a.config.ServerURL() + "/api/v1/config/redis")
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	var redisInfo struct {
		Redis    string `json:"redis"`
		Password string `json:"password"`
		DB       int    `json:"db"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&redisInfo); err != nil {
		return "", "", 0, err
	}

	return redisInfo.Redis, redisInfo.Password, redisInfo.DB, nil
}

// heartbeatLoop 心跳循环
func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(10 * time.Second)
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
		"name":    a.config.Node.NodeName,
		"address": a.getLocalIP(),
		"tags":    a.config.Node.Tags,
		"status":  "online",
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"host":    hostInfo.Hostname,
		"gpus":    a.node.GPUs,
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		log.Printf("序列化心跳数据失败: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/nodes/%s/heartbeat", a.config.ServerURL(), a.nodeID)
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
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			return ip.String()
		}
	}
	return "127.0.0.1"
}

// gpuMonitorLoop GPU监控循环
func (a *Agent) gpuMonitorLoop() {
	interval := a.config.GPU.MonitorInterval
	if interval <= 0 {
		log.Println("GPU监控间隔未配置，跳过gpuMonitorLoop")
		return
	}

	gpuMonitor := NewGPUMonitor(a)
	gpuMonitor.Start()
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
	a.updateTaskStatus(task, "running", "")

	go func() {
		logs, err := a.runTaskContainer(task)
		if err != nil {
			a.reportTaskError(task, err.Error(), logs)
			return
		}

		if err := a.uploadModelArtifacts(task); err != nil {
			a.reportTaskError(task, fmt.Sprintf("模型上传失败: %v", err), logs)
			return
		}

		exitCode := 0
		task.ExitCode = &exitCode
		a.updateTaskStatus(task, "completed", logs)
		log.Printf("任务完成: %s", task.Name)
	}()
}

func (a *Agent) runTaskContainer(task *models.Task) (string, error) {
	if a.dockerMgr == nil {
		return "", fmt.Errorf("Docker未初始化")
	}

	spec := cloneTask(task)

	env := copyEnv(spec.Environment)
	if spec.DatasetPath != "" {
		// 数据集路径作为远端/MinIO路径传给容器，由训练脚本自行下载
		env["DATASET_PATH"] = spec.DatasetPath
	}

	if spec.ModelOutputPath != "" {
		if err := os.MkdirAll(spec.ModelOutputPath, 0o755); err != nil {
			return "", fmt.Errorf("创建模型输出目录失败: %w", err)
		}
	}
	if spec.ModelOutputContainer != "" {
		env["MODEL_OUTPUT_PATH"] = spec.ModelOutputContainer
	} else {
		env["MODEL_OUTPUT_PATH"] = "/workspace/output"
	}
	if spec.ModelOutputPath != "" {
		spec.Volumes = append(spec.Volumes, fmt.Sprintf("%s:%s", spec.ModelOutputPath, env["MODEL_OUTPUT_PATH"]))
	}

	if spec.Iterations > 0 {
		env["TRAINING_ITERATIONS"] = strconv.Itoa(spec.Iterations)
	}

	if spec.ScriptPath != "" {
		env["TRAINING_SCRIPT"] = spec.ScriptPath
	}

	if spec.MinioEndpoint != "" {
		env["MINIO_ENDPOINT"] = spec.MinioEndpoint
		env["MINIO_BUCKET"] = spec.MinioBucket
		env["MINIO_ACCESS_KEY"] = spec.MinioAccessKey
		env["MINIO_SECRET_KEY"] = spec.MinioSecretKey
	}
	if spec.CodeRepo != "" {
		env["CODE_REPO"] = spec.CodeRepo
	}

	spec.Environment = env
	// 如果提供了脚本路径，则优先执行该脚本，避免与启动命令冲突
	cmd := strings.TrimSpace(spec.Command)
	if spec.ScriptPath != "" {
		cmd = fmt.Sprintf("bash %s", spec.ScriptPath)
	}
	if cmd == "" {
		return "", fmt.Errorf("启动命令不能为空")
	}
	spec.Command = cmd

	devices, err := a.dockerMgr.GetGPUDevices()
	if err != nil {
		return "", err
	}

	allocated, err := a.allocateGPUs(devices, spec.GPUCount)
	if err != nil {
		return "", err
	}
	defer a.releaseGPUs(allocated)

	containerID, err := a.dockerMgr.StartContainer(&spec, allocated)
	if err != nil {
		return "", err
	}
	task.ContainerID = containerID
	a.updateTaskStatus(task, "running", "")

	exitCode, err := a.dockerMgr.WaitForContainer(containerID)
	logs := a.collectContainerLogs(containerID)
	if err := a.dockerMgr.RemoveContainer(containerID); err != nil {
		log.Printf("移除容器失败: %v", err)
	}

	if err != nil {
		return logs, err
	}

	code := int(exitCode)
	task.ExitCode = &code

	if exitCode != 0 {
		return logs, fmt.Errorf("容器退出码 %d", exitCode)
	}

	return logs, nil
}

func (a *Agent) collectContainerLogs(containerID string) string {
	if a.dockerMgr == nil || containerID == "" {
		return ""
	}

	logs, err := a.dockerMgr.GetContainerLogs(containerID)
	if err != nil {
		log.Printf("获取容器日志失败: %v", err)
		return ""
	}

	return truncateLogs(logs)
}

func truncateLogs(logs string) string {
	const maxLogSize = 20000
	if len(logs) > maxLogSize {
		return logs[len(logs)-maxLogSize:]
	}
	return logs
}

// GPU 申请与释放，防止同一节点重复分配
func (a *Agent) allocateGPUs(all []string, count int) ([]string, error) {
	if count <= 0 {
		return nil, nil
	}
	a.gpuMu.Lock()
	defer a.gpuMu.Unlock()

	free := make([]string, 0, len(all))
	for _, id := range all {
		if !a.gpuInUse[id] {
			free = append(free, id)
		}
	}
	if len(free) < count {
		return nil, fmt.Errorf("节点可用GPU不足，要求: %d, 可用: %d", count, len(free))
	}
	allocated := free[:count]
	for _, id := range allocated {
		a.gpuInUse[id] = true
	}
	return allocated, nil
}

func (a *Agent) releaseGPUs(ids []string) {
	if len(ids) == 0 {
		return
	}
	a.gpuMu.Lock()
	defer a.gpuMu.Unlock()
	for _, id := range ids {
		delete(a.gpuInUse, id)
	}
}

func cloneTask(task *models.Task) models.Task {
	spec := *task
	if task.Volumes != nil {
		spec.Volumes = append([]string{}, task.Volumes...)
	}
	if task.Environment != nil {
		spec.Environment = copyEnv(task.Environment)
	} else {
		spec.Environment = map[string]string{}
	}
	return spec
}

func copyEnv(env map[string]string) map[string]string {
	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = v
	}
	return result
}

func ensureDir(path string) error {
	if path == "" {
		return fmt.Errorf("目录为空")
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("目录不存在: %s", path)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("路径不是目录: %s", path)
	}
	return nil
}

// updateTaskStatus 更新任务状态
func (a *Agent) updateTaskStatus(task *models.Task, status string, logs string) {
	task.Status = status

	payload := map[string]interface{}{
		"status":           status,
		"assigned_node_id": task.AssignedNodeID,
		"container_id":     task.ContainerID,
	}

	if task.ExitCode != nil {
		payload["exit_code"] = task.ExitCode
	}
	if task.ErrorMessage != "" {
		payload["error_message"] = task.ErrorMessage
	}
	if len(task.Environment) > 0 {
		payload["environment"] = task.Environment
	}
	if strings.TrimSpace(logs) != "" {
		payload["logs"] = logs
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("序列化任务状态失败: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/tasks/%s", a.config.ServerURL(), task.ID)
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
func (a *Agent) reportTaskError(task *models.Task, errorMsg string, logs string) {
	task.Status = "failed"
	task.ErrorMessage = errorMsg

	a.updateTaskStatus(task, "failed", logs)
	log.Printf("任务错误: %s - %s", task.Name, errorMsg)
}

func (a *Agent) uploadModelArtifacts(task *models.Task) error {
	if task.MinioEndpoint == "" || task.MinioBucket == "" || task.ModelOutputPath == "" {
		return nil
	}

	info, err := os.Stat(task.ModelOutputPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("模型输出路径不是目录: %s", task.ModelOutputPath)
	}

	base := filepath.Clean(task.ModelOutputPath)
	return filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		objectKey := filepath.ToSlash(rel)
		return putObjectToMinio(task, objectKey, path)
	})
}

func putObjectToMinio(task *models.Task, objectKey, filePath string) error {
	if task.MinioAccessKey == "" || task.MinioSecretKey == "" {
		return fmt.Errorf("MinIO凭证未配置")
	}

	baseURL, err := normalizeEndpoint(task.MinioEndpoint)
	if err != nil {
		return err
	}

	payload, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	objectPath := buildObjectPath(task.MinioBucket, objectKey)
	fullURL := *baseURL
	fullURL.Path = objectPath

	payloadHash := sha256.Sum256(payload)
	payloadHex := hex.EncodeToString(payloadHash[:])

	req, err := http.NewRequest("PUT", fullURL.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Length", strconv.Itoa(len(payload)))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-content-sha256", payloadHex)

	amzDate := time.Now().UTC().Format("20060102T150405Z")
	dateStamp := amzDate[:8]
	req.Header.Set("x-amz-date", amzDate)

	canonicalURI := objectPath
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", baseURL.Host, payloadHex, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	hashCanonical := sha256.Sum256([]byte(strings.Join([]string{
		"PUT",
		canonicalURI,
		"",
		canonicalHeaders,
		signedHeaders,
		payloadHex,
	}, "\n")))
	canonicalHashHex := hex.EncodeToString(hashCanonical[:])

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, defaultS3Region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		canonicalHashHex,
	}, "\n")

	signingKey := getSigningKey(task.MinioSecretKey, dateStamp, defaultS3Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		task.MinioAccessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("上传到MinIO失败: %s", string(body))
	}

	return nil
}

func normalizeEndpoint(ep string) (*url.URL, error) {
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "https://" + ep
	}
	u, err := url.Parse(ep)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	u.Path = ""
	return u, nil
}

func buildObjectPath(bucket, objectKey string) string {
	parts := strings.Split(objectKey, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return fmt.Sprintf("/%s/%s", url.PathEscape(bucket), strings.Join(parts, "/"))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func getSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func loadOrCreateNodeID() string {
	if data, err := os.ReadFile(".node_id"); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}
	id := uuid.New().String()
	if err := os.WriteFile(".node_id", []byte(id), 0600); err != nil {
		log.Printf("写入.node_id失败: %v", err)
	}
	return id
}
