package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	GPU      GPUConfig      `yaml:"gpu"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Docker   DockerConfig   `yaml:"docker"`
	Node     NodeConfig     `yaml:"node"`
	Security SecurityConfig `yaml:"security"`
	Log      LogConfig      `yaml:"log"`
	Web      WebConfig      `yaml:"web"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port"`
	Mode        string   `yaml:"mode"`
	CORSOrigins []string `yaml:"cors_origins"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// GPUConfig GPU配置
type GPUConfig struct {
	NvidiaSMIPath       string `yaml:"nvidia_smi_path"`
	MonitorInterval     int    `yaml:"monitor_interval"`
	UtilizationThreshold int  `yaml:"utilization_threshold"`
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	MaxConcurrentTasks int `yaml:"max_concurrent_tasks"`
	TaskTimeout        int `yaml:"task_timeout"`
	CleanupInterval    int `yaml:"cleanup_interval"`
	MaxRetries         int `yaml:"max_retries"`
}

// DockerConfig Docker配置
type DockerConfig struct {
	SocketPath     string `yaml:"socket_path"`
	NetworkMode    string `yaml:"network_mode"`
	DataVolumePath string `yaml:"data_volume_path"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	Name             string   `yaml:"name"`
	NodeName         string   `yaml:"node_name"`
	Tags             []string `yaml:"tags"`
	MaxGPUCount      int      `yaml:"max_gpu_count"`
	HeartbeatInterval int    `yaml:"heartbeat_interval"`
	ServerURL        string   `yaml:"server_url"`
}

// 添加便捷方法
func (c *Config) NodeName() string {
	return c.Node.NodeName
}

func (c *Config) ServerURL() string {
	return c.Node.ServerURL
}

func (c *Config) Tags() []string {
	return c.Node.Tags
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	JWTSecret   string `yaml:"jwt_secret"`
	TokenExpiry int    `yaml:"token_expiry"`
	EnableAuth  bool   `yaml:"enable_auth"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
}

// WebConfig Web界面配置
type WebConfig struct {
	Enable bool   `yaml:"enable"`
	Title  string `yaml:"title"`
	Theme  string `yaml:"theme"`
}

// LoadConfig 加载配置文件
func LoadConfig(filePath string) (*Config, error) {
	if filePath == "" {
		return nil, fmt.Errorf("config file path is required")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 设置默认值
	setDefaults(&config)

	return &config, nil
}

// setDefaults 设置配置默认值
func setDefaults(config *Config) {
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Mode == "" {
		config.Server.Mode = "release"
	}
	if len(config.Server.CORSOrigins) == 0 {
		config.Server.CORSOrigins = []string{"*"}
	}

	if config.Database.Driver == "" {
		config.Database.Driver = "sqlite"
	}
	if config.Database.DSN == "" {
		config.Database.DSN = "gcond.db"
	}

	if config.Redis.Host == "" {
		config.Redis.Host = "localhost"
	}
	if config.Redis.Port == 0 {
		config.Redis.Port = 6379
	}

	if config.GPU.NvidiaSMIPath == "" {
		config.GPU.NvidiaSMIPath = "nvidia-smi"
	}
	if config.GPU.MonitorInterval == 0 {
		config.GPU.MonitorInterval = 5
	}
	if config.GPU.UtilizationThreshold == 0 {
		config.GPU.UtilizationThreshold = 20
	}

	if config.Scheduler.MaxConcurrentTasks == 0 {
		config.Scheduler.MaxConcurrentTasks = 4
	}
	if config.Scheduler.TaskTimeout == 0 {
		config.Scheduler.TaskTimeout = 3600
	}
	if config.Scheduler.CleanupInterval == 0 {
		config.Scheduler.CleanupInterval = 300
	}
	if config.Scheduler.MaxRetries == 0 {
		config.Scheduler.MaxRetries = 3
	}

	if config.Docker.SocketPath == "" {
		config.Docker.SocketPath = "/var/run/docker.sock"
	}
	if config.Docker.NetworkMode == "" {
		config.Docker.NetworkMode = "bridge"
	}
	if config.Docker.DataVolumePath == "" {
		config.Docker.DataVolumePath = "/data"
	}

	if config.Node.Name == "" {
		config.Node.Name = "node-1"
	}
	if config.Node.MaxGPUCount == 0 {
		config.Node.MaxGPUCount = 4
	}
	if config.Node.HeartbeatInterval == 0 {
		config.Node.HeartbeatInterval = 30
	}

	if config.Security.JWTSecret == "" {
		config.Security.JWTSecret = "your-secret-key-change-in-production"
	}
	if config.Security.TokenExpiry == 0 {
		config.Security.TokenExpiry = 24
	}

	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.FilePath == "" {
		config.Log.FilePath = "gcond.log"
	}
	if config.Log.MaxSize == 0 {
		config.Log.MaxSize = 100
	}
	if config.Log.MaxBackups == 0 {
		config.Log.MaxBackups = 10
	}
	if config.Log.MaxAge == 0 {
		config.Log.MaxAge = 30
	}

	if config.Web.Title == "" {
		config.Web.Title = "GPUConductor"
	}
	if config.Web.Theme == "" {
		config.Web.Theme = "light"
	}
}

// GetDSN 获取数据库连接字符串
func (c *Config) GetDSN() string {
	return c.Database.DSN
}

// GetRedisAddr 获取Redis地址
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// IsDebugMode 是否调试模式
func (c *Config) IsDebugMode() bool {
	return c.Server.Mode == "debug"
}
