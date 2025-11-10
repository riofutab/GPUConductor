package api

import (
	"GPUConductor/internal/auth"
	"GPUConductor/internal/logger"
	"GPUConductor/internal/models"
	"GPUConductor/internal/scheduler"
	"GPUConductor/internal/security"
	"GPUConductor/internal/utils"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Handler struct {
	db             *gorm.DB
	scheduler      *scheduler.Scheduler
	redisAddr      string
	heartbeatGrace time.Duration
	logger         *zap.Logger
}

func NewHandler(db *gorm.DB, sched *scheduler.Scheduler, redisAddr string, heartbeatGrace time.Duration) *Handler {
	err := logger.InitLogger(false)
	if err != nil {
		panic(fmt.Sprintf("初始化日志失败: %v", err))
	}
	return &Handler{
		db:             db,
		scheduler:      sched,
		redisAddr:      redisAddr,
		heartbeatGrace: heartbeatGrace,
		logger:         logger.Logger,
	}
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// Login 用户登录
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("登录请求参数验证失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}

	h.logger.Info("用户登录请求", zap.String("mobile", req.Mobile))

	var user models.User
	now := time.Now()

	if auth.IsConfigured() {
		ldapUser, err := auth.AuthenticateUser(req.Mobile, req.Password)
		if err != nil {
			if errors.Is(err, auth.ErrLDAPNotConfigured) {
				h.logger.Warn("LDAP未配置，使用本地认证", zap.String("mobile", req.Mobile))
			} else {
				h.logger.Warn("LDAP认证失败", zap.String("mobile", req.Mobile), zap.Error(err))
				c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "认证失败"})
				return
			}
		} else {
			h.logger.Info("LDAP认证成功", zap.String("mobile", req.Mobile))
			result := h.db.Where("username = ?", req.Mobile).First(&user)
			if result.Error == gorm.ErrRecordNotFound {
				mobile := ldapUser.Mobile
				if mobile == "" {
					mobile = req.Mobile
				}

				user = models.User{
					Username:    req.Mobile,
					Mobile:      mobile,
					Email:       ldapUser.Email,
					FullName:    ldapUser.FullName,
					DisplayName: ldapUser.FullName,
					Role:        "user",
					IsActive:    true,
					LastLogin:   utils.Ptr(now),
					LastLoginAt: utils.Ptr(now),
				}
				if err := h.db.Create(&user).Error; err != nil {
					h.logger.Error("创建LDAP用户失败", zap.String("mobile", req.Mobile), zap.Error(err))
					c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
					return
				}
				h.logger.Info("创建新LDAP用户成功", zap.String("mobile", req.Mobile))
			} else if result.Error != nil {
				h.logger.Error("查询LDAP用户失败", zap.String("mobile", req.Mobile), zap.Error(result.Error))
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
				return
			} else {
				user.LastLogin = utils.Ptr(now)
				user.LastLoginAt = utils.Ptr(now)
				h.db.Save(&user)
			}

			token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
			if err != nil {
				h.logger.Error("生成JWT token失败", zap.String("mobile", req.Mobile), zap.Error(err))
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
				return
			}

			h.logger.Info("用户登录成功", zap.String("mobile", req.Mobile))

			c.JSON(http.StatusOK, LoginResponse{
				Token: token,
				User:  &user,
			})
			return
		}
	}

	// 本地用户认证
	result := h.db.Where("username = ?", req.Mobile).First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		h.logger.Warn("本地用户不存在", zap.String("mobile", req.Mobile))
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "用户名或密码错误"})
		return
	} else if result.Error != nil {
		h.logger.Error("查询用户失败", zap.String("mobile", req.Mobile), zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	if !security.CheckPassword(user.Password, req.Password) {
		h.logger.Warn("密码验证失败", zap.String("mobile", req.Mobile))
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "用户名或密码错误"})
		return
	}

	user.LastLogin = utils.Ptr(now)
	user.LastLoginAt = utils.Ptr(now)
	h.db.Save(&user)

	// 生成JWT token
	token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		h.logger.Error("生成JWT token失败", zap.String("mobile", req.Mobile), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	h.logger.Info("用户登录成功", zap.String("mobile", req.Mobile))

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  &user,
	})
}

// GetUserProfile 获取用户信息
func (h *Handler) GetUserProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "未授权"})
		return
	}

	var user models.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		h.logger.Error("查询用户信息失败", zap.String("userID", userID.(string)), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetNodes 获取节点列表
func (h *Handler) GetNodes(c *gin.Context) {
	var nodes []models.Node
	if err := h.db.Preload("GPUs").Find(&nodes).Error; err != nil {
		h.logger.Error("查询节点列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, nodes)
}

// GetTasks 获取任务列表
func (h *Handler) GetTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := c.Query("status")

	offset := (page - 1) * pageSize

	var tasks []models.Task
	query := h.db.Offset(offset).Limit(pageSize).Order("created_at DESC")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&tasks).Error; err != nil {
		h.logger.Error("查询任务列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	var total int64
	h.db.Model(&models.Task{}).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// CreateTask 创建任务
func (h *Handler) CreateTask(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "未授权"})
		return
	}

	var task models.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		h.logger.Warn("创建任务参数验证失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}

	task.CreatedBy = userID.(string)
	task.Status = "pending"

	if h.scheduler != nil {
		if err := h.scheduler.SubmitTask(&task); err != nil {
			h.logger.Error("调度器提交任务失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "提交任务失败"})
			return
		}
	} else if err := h.db.Create(&task).Error; err != nil {
		h.logger.Error("创建任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	h.logger.Info("创建任务成功", zap.String("taskID", task.ID))

	c.JSON(http.StatusCreated, task)
}

// CancelTask 取消任务
func (h *Handler) CancelTask(c *gin.Context) {
	taskID := c.Param("id")

	if h.scheduler != nil {
		if err := h.scheduler.CancelTask(taskID); err != nil {
			h.logger.Error("取消任务失败", zap.String("taskID", taskID), zap.Error(err))
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
	} else {
		var task models.Task
		if err := h.db.Where("id = ?", taskID).First(&task).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "任务不存在"})
				return
			}
			h.logger.Error("查询任务失败", zap.String("taskID", taskID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
			return
		}

		task.Status = "cancelled"
		now := time.Now()
		task.CompletedAt = &now
		if err := h.db.Save(&task).Error; err != nil {
			h.logger.Error("更新任务状态失败", zap.String("taskID", taskID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// GetTask 获取任务详情
func (h *Handler) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	var task models.Task
	if err := h.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "任务不存在"})
			return
		}
		h.logger.Error("查询任务失败", zap.String("taskID", taskID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetTaskLogs 获取任务日志
func (h *Handler) GetTaskLogs(c *gin.Context) {
	taskID := c.Param("id")

	var logs []models.TaskLog
	if err := h.db.Where("task_id = ?", taskID).Order("timestamp DESC").Find(&logs).Error; err != nil {
		h.logger.Error("查询任务日志失败", zap.String("taskID", taskID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// GetStats 获取系统统计信息
func (h *Handler) GetStats(c *gin.Context) {
	// 更新超时节点
	if h.heartbeatGrace > 0 {
		cutoff := time.Now().Add(-h.heartbeatGrace)
		h.db.Model(&models.Node{}).
			Where("last_seen IS NOT NULL AND last_seen < ? AND status = ?", cutoff, "online").
			Update("status", "offline")
	}

	// 获取节点统计
	var totalNodes int64
	var onlineNodes int64
	h.db.Model(&models.Node{}).Count(&totalNodes)
	h.db.Model(&models.Node{}).Where("status = ?", "online").Count(&onlineNodes)

	// 获取任务统计
	var totalTasks int64
	var pendingTasks int64
	var runningTasks int64
	h.db.Model(&models.Task{}).Count(&totalTasks)
	h.db.Model(&models.Task{}).Where("status = ?", "pending").Count(&pendingTasks)
	h.db.Model(&models.Task{}).Where("status = ?", "running").Count(&runningTasks)

	// 获取GPU使用率统计
	var gpus []models.GPU
	if err := h.db.Find(&gpus).Error; err != nil {
		h.logger.Error("查询GPU统计失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	totalGPUs := len(gpus)
	var totalUtilization float64
	for _, gpu := range gpus {
		totalUtilization += gpu.Utilization
	}
	avgUtilization := 0.0
	if totalGPUs > 0 {
		avgUtilization = totalUtilization / float64(totalGPUs)
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": gin.H{
			"total":   totalNodes,
			"online":  onlineNodes,
			"offline": totalNodes - onlineNodes,
		},
		"tasks": gin.H{
			"total":     totalTasks,
			"pending":   pendingTasks,
			"running":   runningTasks,
			"completed": totalTasks - pendingTasks - runningTasks,
		},
		"gpus": gin.H{
			"total":             totalGPUs,
			"avg_utilization":   avgUtilization,
			"total_utilization": totalUtilization,
		},
		"timestamp": time.Now().Unix(),
	})
}

// GetGPUStats 获取GPU详细统计信息
func (h *Handler) GetGPUStats(c *gin.Context) {
	var gpus []models.GPU
	if err := h.db.Find(&gpus).Error; err != nil {
		h.logger.Error("查询GPU统计失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, gpus)
}

// UpdateNodeStatus 更新节点状态
func (h *Handler) UpdateNodeStatus(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("更新节点状态参数验证失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}

	var node models.Node
	if err := h.db.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "节点不存在"})
			return
		}
		h.logger.Error("查询节点失败", zap.String("nodeID", nodeID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	node.Status = req.Status
	node.LastSeen = time.Now()

	if err := h.db.Save(&node).Error; err != nil {
		h.logger.Error("更新节点状态失败", zap.String("nodeID", nodeID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	h.logger.Info("更新节点状态成功", zap.String("nodeID", nodeID), zap.String("status", req.Status))
	c.JSON(http.StatusOK, node)
}

// GetRedisConfig 返回Redis配置
func (h *Handler) GetRedisConfig(c *gin.Context) {
	if h.redisAddr == "" {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "Redis未配置"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"redis": h.redisAddr})
}

// NodeHeartbeat 处理节点心跳
func (h *Handler) NodeHeartbeat(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Name    string       `json:"name"`
		Address string       `json:"address"`
		Tags    []string     `json:"tags"`
		Status  string       `json:"status"`
		GPUs    []models.GPU `json:"gpus"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("心跳数据解析失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}

	if nodeID == "" {
		nodeID = req.Name
	}
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "节点ID缺失"})
		return
	}

	var node models.Node
	err := h.db.Where("id = ?", nodeID).First(&node).Error
	if err == gorm.ErrRecordNotFound {
		node = models.Node{ID: nodeID}
	} else if err != nil {
		h.logger.Error("查询节点失败", zap.String("nodeID", nodeID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	if req.Name != "" {
		existing := models.Node{}
		if err := h.db.Where("name = ?", req.Name).First(&existing).Error; err == nil && existing.ID != node.ID {
			req.Name = fmt.Sprintf("%s-%s", req.Name, nodeID[:8])
		}
		node.Name = req.Name
	}
	node.Address = req.Address
	node.Tags = req.Tags
	if req.Status != "" {
		node.Status = req.Status
	} else {
		node.Status = "online"
	}
	node.LastSeen = time.Now()

	if err := h.db.Save(&node).Error; err != nil {
		h.logger.Error("保存节点信息失败", zap.String("nodeID", nodeID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	if len(req.GPUs) > 0 {
		if err := h.db.Where("node_id = ?", node.ID).Delete(&models.GPU{}).Error; err != nil {
			h.logger.Error("清理旧GPU信息失败", zap.String("nodeID", nodeID), zap.Error(err))
		} else {
			for _, gpu := range req.GPUs {
				gpu.NodeID = node.ID
				if err := h.db.Create(&gpu).Error; err != nil {
					h.logger.Warn("保存GPU信息失败", zap.String("nodeID", nodeID), zap.Error(err))
				}
			}
		}
	}

	c.JSON(http.StatusOK, node)
}

// UpdateTaskStatus 由Agent回传任务状态
func (h *Handler) UpdateTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	var req struct {
		Status         string            `json:"status"`
		AssignedNodeID string            `json:"assigned_node_id"`
		ContainerID    string            `json:"container_id"`
		ExitCode       *int              `json:"exit_code"`
		ErrorMessage   string            `json:"error_message"`
		Environment    map[string]string `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("任务状态更新参数错误", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}

	var task models.Task
	if err := h.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "任务不存在"})
			return
		}
		h.logger.Error("查询任务失败", zap.String("taskID", taskID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	if req.Status != "" {
		task.Status = req.Status
		if req.Status == "completed" || req.Status == "failed" {
			now := time.Now()
			task.CompletedAt = &now
		}
	}

	if req.AssignedNodeID != "" {
		task.AssignedNodeID = req.AssignedNodeID
	}
	if req.ContainerID != "" {
		task.ContainerID = req.ContainerID
	}
	if req.ExitCode != nil {
		task.ExitCode = req.ExitCode
	}
	if req.ErrorMessage != "" {
		task.ErrorMessage = req.ErrorMessage
	}
	if req.Environment != nil {
		task.Environment = req.Environment
	}

	if err := h.db.Save(&task).Error; err != nil {
		h.logger.Error("更新任务失败", zap.String("taskID", taskID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// HealthCheck 健康检查
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// AuthMiddleware JWT认证中间件
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "未授权"})
			c.Abort()
			return
		}

		// 去掉Bearer前缀
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		claims, err := auth.ValidateToken(token)
		if err != nil {
			h.logger.Warn("JWT验证失败", zap.Error(err))
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "令牌无效"})
			c.Abort()
			return
		}

		// 设置用户信息到上下文
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}
