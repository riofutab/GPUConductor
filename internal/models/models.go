package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// User 用户模型
type User struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Username    string     `json:"username" gorm:"uniqueIndex"`
	Mobile      string     `json:"mobile" gorm:"uniqueIndex"` // 手机号作为登录标识
	Password    string     `json:"-"`                         // 密码不序列化到JSON
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	DisplayName string     `json:"display_name"`
	Role        string     `json:"role" gorm:"default:user"`     // admin, user
	Status      string     `json:"status" gorm:"default:active"` // active, inactive
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	LastLogin   *time.Time `json:"last_login"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Node 代表一个GPU节点
type Node struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex"`
	Address   string    `json:"address"`
	Status    string    `json:"status"` // online, offline, busy
	Tags      []string  `json:"tags" gorm:"serializer:json"`
	Priority  int       `json:"priority" gorm:"default:1"`
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// GPU信息
	GPUs []GPU `json:"gpus" gorm:"foreignKey:NodeID"`
}

// GPU 代表一个GPU设备
type GPU struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	NodeID      string    `json:"node_id"`
	Index       int       `json:"index"`
	Name        string    `json:"name"`
	Memory      int64     `json:"memory"`      // 总内存 MB
	MemoryUsed  int64     `json:"memory_used"` // 已使用内存 MB
	Utilization float64   `json:"utilization"` // 使用率 0-100
	Temperature int       `json:"temperature"` // 温度
	PowerUsage  int       `json:"power_usage"` // 功耗 W
	UpdatedAt   time.Time `json:"updated_at"`
}

// Task 代表一个训练任务
type Task struct {
	ID            string `json:"id" gorm:"primaryKey"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Status        string `json:"status"` // pending, running, completed, failed, cancelled
	Priority      int    `json:"priority" gorm:"default:1"`
	QueuePosition int    `json:"queue_position" gorm:"-"` // 队列位置（不持久化）

	// 用户信息
	CreatedBy string `json:"created_by"` // 创建用户

	// 执行配置
	Image       string            `json:"image"`                              // Docker镜像
	Command     string            `json:"command"`                            // 执行命令
	Environment map[string]string `json:"environment" gorm:"serializer:json"` // 环境变量
	Volumes     []string          `json:"volumes" gorm:"serializer:json"`     // 挂载卷

	// 资源要求
	GPUCount  int      `json:"gpu_count" gorm:"default:1"`
	GPUMemory int64    `json:"gpu_memory"`                       // 所需GPU内存 MB
	NodeTags  []string `json:"node_tags" gorm:"serializer:json"` // 节点标签要求
	NodeID    string   `json:"node_id"`                          // 指定节点ID (可选)

	// 时间控制
	MaxDuration int        `json:"max_duration"` // 最大执行时间 (分钟)
	SubmittedAt time.Time  `json:"submitted_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// 执行信息
	AssignedNodeID string `json:"assigned_node_id"`
	ContainerID    string `json:"container_id"`
	ExitCode       *int   `json:"exit_code"`
	ErrorMessage   string `json:"error_message"`

	// 业务扩展
	DatasetPath          string             `json:"dataset_path"`
	MinioEndpoint        string             `json:"minio_endpoint"`
	MinioBucket          string             `json:"minio_bucket"`
	MinioAccessKey       string             `json:"minio_access_key"`
	MinioSecretKey       string             `json:"minio_secret_key"`
	ModelOutputPath      string             `json:"model_output_path"`
	ModelOutputContainer string             `json:"model_output_container"`
	ScriptPath           string             `json:"script_path"`
	CodeRepo             string             `json:"code_repo"`
	Iterations           int                `json:"iterations"`
	CurrentIteration     int                `json:"current_iteration"`
	Metrics              map[string]float64 `json:"metrics" gorm:"serializer:json"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskLog 任务日志
type TaskLog struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	TaskID    string    `json:"task_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Setting 简单的键值配置
type Setting struct {
	Key       string    `json:"key" gorm:"primaryKey"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate 在创建前生成UUID
func (n *Node) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}

func (g *GPU) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.SubmittedAt.IsZero() {
		t.SubmittedAt = time.Now()
	}
	return nil
}

func (tl *TaskLog) BeforeCreate(tx *gorm.DB) error {
	if tl.ID == "" {
		tl.ID = uuid.New().String()
	}
	if tl.Timestamp.IsZero() {
		tl.Timestamp = time.Now()
	}
	return nil
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}
