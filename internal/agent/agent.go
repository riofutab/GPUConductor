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
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
		config: config,
		nodeID: node.ID,
		node:   node,
		ctx:    ctx,
		cancel: cancel,
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

// getRedisConfig 从服务器获取Redis配置
func (a *Agent) getRedisConfig() (string, error) {
	resp, err := http.Get(a.config.ServerURL() + "/api/v1/config/redis")
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
	// 简单实现，实际可能需要更复杂的逻辑
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
	a.updateTaskStatus(task, "running")

	go func() {
		if err := a.runTaskContainer(task); err != nil {
			a.reportTaskError(task, err.Error())
			return
		}

		if err := a.uploadModelArtifacts(task); err != nil {
			a.reportTaskError(task, fmt.Sprintf("模型上传失败: %v", err))
			return
		}

		exitCode := 0
		task.ExitCode = &exitCode
		a.updateTaskStatus(task, "completed")
		log.Printf("任务完成: %s", task.Name)
	}()
}

func (a *Agent) runTaskContainer(task *models.Task) error {
	if a.dockerMgr == nil {
		return fmt.Errorf("Docker未初始化")
	}

	spec := cloneTask(task)

	env := copyEnv(spec.Environment)
	if spec.DatasetPath != "" {
		if err := ensureDir(spec.DatasetPath); err != nil {
			return fmt.Errorf("数据集目录无效: %w", err)
		}
		env["DATASET_PATH"] = "/workspace/dataset"
		spec.Volumes = append(spec.Volumes, fmt.Sprintf("%s:/workspace/dataset:ro", spec.DatasetPath))
	}

	if spec.ModelOutputPath != "" {
		if err := os.MkdirAll(spec.ModelOutputPath, 0o755); err != nil {
			return fmt.Errorf("创建模型输出目录失败: %w", err)
		}
		env["MODEL_OUTPUT_PATH"] = "/workspace/output"
		spec.Volumes = append(spec.Volumes, fmt.Sprintf("%s:/workspace/output", spec.ModelOutputPath))
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

	spec.Environment = env

	devices, err := a.dockerMgr.GetGPUDevices()
	if err != nil {
		return err
	}

	if spec.GPUCount > len(devices) {
		return fmt.Errorf("节点可用GPU不足，要求: %d, 可用: %d", spec.GPUCount, len(devices))
	}

	allocated := devices
	if spec.GPUCount > 0 && spec.GPUCount < len(devices) {
		allocated = devices[:spec.GPUCount]
	}

	containerID, err := a.dockerMgr.StartContainer(&spec, allocated)
	if err != nil {
		return err
	}
	task.ContainerID = containerID

	exitCode, err := a.dockerMgr.WaitForContainer(containerID)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		logs, _ := a.dockerMgr.GetContainerLogs(containerID)
		return fmt.Errorf("容器退出码 %d: %s", exitCode, logs)
	}

	return nil
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
func (a *Agent) reportTaskError(task *models.Task, errorMsg string) {
	task.Status = "failed"
	task.ErrorMessage = errorMsg
	now := time.Now()
	task.CompletedAt = &now

	a.updateTaskStatus(task, "failed")
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
