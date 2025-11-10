package docker

import (
	"GPUConductor/internal/models"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerManager Docker容器管理器
type DockerManager struct {
	cli *client.Client
}

// NewDockerManager 创建Docker管理器
func NewDockerManager() (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("创建Docker客户端失败: %w", err)
	}

	return &DockerManager{cli: cli}, nil
}

// StartContainer 启动训练容器
func (dm *DockerManager) StartContainer(task *models.Task, gpuDevices []string) (string, error) {
	ctx := context.Background()

	// 检查镜像是否存在，不存在则拉取
	if err := dm.pullImage(ctx, task.Image); err != nil {
		return "", fmt.Errorf("拉取镜像失败: %w", err)
	}

	// 创建容器配置
	config := &container.Config{
		Image: task.Image,
		Cmd:   []string{"sh", "-c", task.Command},
		Env:   buildEnvVars(task.Environment, gpuDevices),
		Tty:   false,
	}

	// 创建主机配置
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "no",
		},
		AutoRemove: false,
		Mounts:     buildMounts(task.Volumes),
	}

	if len(gpuDevices) > 0 {
		hostConfig.Resources = container.Resources{
			DeviceRequests: []container.DeviceRequest{
				{
					Driver:       "nvidia",
					Count:        len(gpuDevices),
					DeviceIDs:    gpuDevices,
					Capabilities: [][]string{{"gpu"}},
				},
			},
		}
	}

	// 创建容器
	resp, err := dm.cli.ContainerCreate(ctx, config, hostConfig, nil, nil,
		fmt.Sprintf("gcond-task-%s", task.ID))
	if err != nil {
		return "", fmt.Errorf("创建容器失败: %w", err)
	}

	// 启动容器
	if err := dm.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("启动容器失败: %w", err)
	}

	log.Printf("任务 %s 容器已启动: %s", task.ID, resp.ID)
	return resp.ID, nil
}

// WaitForContainer 等待容器退出
func (dm *DockerManager) WaitForContainer(containerID string) (int64, error) {
	ctx := context.Background()
	statusCh, errCh := dm.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return -1, fmt.Errorf("等待容器失败: %w", err)
		}
	case status := <-statusCh:
		return status.StatusCode, nil
	}
	return -1, nil
}

// StopContainer 停止容器
func (dm *DockerManager) StopContainer(containerID string) error {
	ctx := context.Background()

	timeout := 30 * time.Second
	if err := dm.cli.ContainerStop(ctx, containerID, &timeout); err != nil {
		return fmt.Errorf("停止容器失败: %w", err)
	}

	return nil
}

// RemoveContainer 移除容器
func (dm *DockerManager) RemoveContainer(containerID string) error {
	ctx := context.Background()

	if err := dm.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("移除容器失败: %w", err)
	}

	return nil
}

// GetContainerLogs 获取容器日志
func (dm *DockerManager) GetContainerLogs(containerID string) (string, error) {
	ctx := context.Background()

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	}

	reader, err := dm.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("获取容器日志失败: %w", err)
	}
	defer reader.Close()

	// 读取日志输出
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	if err != nil {
		return "", fmt.Errorf("读取容器日志失败: %w", err)
	}

	logs := stdout.String()
	if stderr.Len() > 0 {
		logs += "\nSTDERR:\n" + stderr.String()
	}

	return logs, nil
}

// GetContainerStatus 获取容器状态
func (dm *DockerManager) GetContainerStatus(containerID string) (string, error) {
	ctx := context.Background()

	containerJSON, err := dm.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("检查容器状态失败: %w", err)
	}

	return containerJSON.State.Status, nil
}

// ListContainers 列出所有容器
func (dm *DockerManager) ListContainers() ([]types.Container, error) {
	ctx := context.Background()

	containers, err := dm.cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("列出容器失败: %w", err)
	}

	return containers, nil
}

// CleanupContainers 清理容器
func (dm *DockerManager) CleanupContainers() error {
	ctx := context.Background()

	containers, err := dm.ListContainers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		// 清理已停止的gcond任务容器
		if strings.HasPrefix(container.Names[0], "/gcond-task-") &&
			container.State == "exited" {
			if err := dm.RemoveContainer(container.ID); err != nil {
				log.Printf("清理容器 %s 失败: %v", container.ID, err)
			} else {
				log.Printf("已清理容器: %s", container.ID)
			}
		}
	}

	return nil
}

// pullImage 拉取镜像
func (dm *DockerManager) pullImage(ctx context.Context, image string) error {
	// 检查镜像是否已存在
	images, err := dm.cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return err
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == image {
				return nil // 镜像已存在
			}
		}
	}

	// 拉取镜像
	log.Printf("正在拉取镜像: %s", image)
	reader, err := dm.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}
	defer reader.Close()

	// 等待拉取完成
	buf := make([]byte, 1024)
	for {
		_, err := reader.Read(buf)
		if err != nil {
			break
		}
	}

	log.Printf("镜像拉取完成: %s", image)
	return nil
}

// HealthCheck 健康检查
func (dm *DockerManager) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := dm.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("Docker守护进程不可用: %w", err)
	}

	return nil
}

// GetGPUDevices 获取GPU设备列表
func (dm *DockerManager) GetGPUDevices() ([]string, error) {
	// 这里可以扩展为从系统获取实际的GPU设备列表
	// 目前返回模拟的设备列表
	return []string{"0", "1", "2", "3"}, nil
}

func buildMounts(volumes []string) []mount.Mount {
	if len(volumes) == 0 {
		return nil
	}

	mounts := make([]mount.Mount, 0, len(volumes))
	for _, vol := range volumes {
		parts := strings.Split(vol, ":")
		if len(parts) < 2 {
			continue
		}

		m := mount.Mount{
			Type:   mount.TypeBind,
			Source: strings.TrimSpace(parts[0]),
			Target: strings.TrimSpace(parts[1]),
		}

		if len(parts) >= 3 {
			mode := strings.TrimSpace(parts[2])
			if strings.EqualFold(mode, "ro") {
				m.ReadOnly = true
			}
		}

		mounts = append(mounts, m)
	}

	return mounts
}

func buildEnvVars(env map[string]string, gpuDevices []string) []string {
	envVars := make([]string, 0, len(env)+2)

	if len(gpuDevices) > 0 {
		joined := strings.Join(gpuDevices, ",")
		envVars = append(envVars,
			fmt.Sprintf("NVIDIA_VISIBLE_DEVICES=%s", joined),
			"CUDA_VISIBLE_DEVICES="+joined,
		)
	}

	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", strings.TrimSpace(k), v))
	}

	return envVars
}
