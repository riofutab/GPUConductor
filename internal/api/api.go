package api

import (
	"GPUConductor/internal/auth"
	"GPUConductor/internal/models"
	"GPUConductor/internal/scheduler"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type Handler struct {
	db        *gorm.DB
	scheduler *scheduler.Scheduler
	upgrader  websocket.Upgrader
}

func New(db *gorm.DB, scheduler *scheduler.Scheduler) *Handler {
	return &Handler{
		db:        db,
		scheduler: scheduler,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许跨域
			},
		},
	}
}

// GetNodes 获取所有节点
func (h *Handler) GetNodes(c *gin.Context) {
	var nodes []models.Node
	if err := h.db.Preload("GPUs").Find(&nodes).Error; err != nil {
		c.JSON(500, gin.H{"error": "查询节点失败"})
		return
	}
	c.JSON(200, nodes)
}

// GetNode 获取单个节点
func (h *Handler) GetNode(c *gin.Context) {
	id := c.Param("id")
	var node models.Node
	if err := h.db.Preload("GPUs").First(&node, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "节点不存在"})
		return
	}
	c.JSON(200, node)
}

// NodeHeartbeat 节点心跳
func (h *Handler) NodeHeartbeat(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name    string   `json:"name"`
		Address string   `json:"address"`
		Tags    []string `json:"tags"`
		Status  string   `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请求格式错误"})
		return
	}

	var node models.Node
	result := h.db.First(&node, "id = ?", id)

	if result.Error == gorm.ErrRecordNotFound {
		// 创建新节点
		node = models.Node{
			ID:       id,
			Name:     req.Name,
			Address:  req.Address,
			Tags:     req.Tags,
			Status:   req.Status,
			LastSeen: time.Now(),
		}
		if err := h.db.Create(&node).Error; err != nil {
			c.JSON(500, gin.H{"error": "创建节点失败"})
			return
		}
	} else if result.Error != nil {
		c.JSON(500, gin.H{"error": "查询节点失败"})
		return
	} else {
		// 更新现有节点
		node.Name = req.Name
		node.Address = req.Address
		node.Tags = req.Tags
		node.Status = req.Status
		node.LastSeen = time.Now()
		if err := h.db.Save(&node).Error; err != nil {
			c.JSON(500, gin.H{"error": "更新节点失败"})
			return
		}
	}

	c.JSON(200, node)
}

// UpdateNodeGPUs 更新节点GPU信息
func (h *Handler) UpdateNodeGPUs(c *gin.Context) {
	nodeID := c.Param("id")

	var gpus []models.GPU
	if err := c.ShouldBindJSON(&gpus); err != nil {
		c.JSON(400, gin.H{"error": "请求格式错误"})
		return
	}

	// 删除旧的GPU记录
	h.db.Where("node_id = ?", nodeID).Delete(&models.GPU{})

	// 创建新的GPU记录
	for i := range gpus {
		gpus[i].NodeID = nodeID
		gpus[i].UpdatedAt = time.Now()
	}

	if err := h.db.Create(&gpus).Error; err != nil {
		c.JSON(500, gin.H{"error": "更新GPU信息失败"})
		return
	}

	c.JSON(200, gin.H{"message": "GPU信息更新成功"})
}

// GetTasks 获取任务列表
func (h *Handler) GetTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")

	offset := (page - 1) * limit

	query := h.db.Model(&models.Task{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var tasks []models.Task
	if err := query.Order("created_at DESC").
		Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		c.JSON(500, gin.H{"error": "查询任务失败"})
		return
	}

	c.JSON(200, gin.H{
		"tasks": tasks,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetTask 获取单个任务
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	var task models.Task
	if err := h.db.First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(200, task)
}

// CreateTask 创建任务
func (h *Handler) CreateTask(c *gin.Context) {
	var task models.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(400, gin.H{"error": "请求格式错误"})
		return
	}

	if err := h.scheduler.SubmitTask(&task); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, task)
}

// UpdateTask 更新任务
func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")

	var task models.Task
	if err := h.db.First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	var updates models.Task
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(400, gin.H{"error": "请求格式错误"})
		return
	}

	// 只允许更新某些字段
	task.Name = updates.Name
	task.Description = updates.Description
	task.Priority = updates.Priority

	if err := h.db.Save(&task).Error; err != nil {
		c.JSON(500, gin.H{"error": "更新任务失败"})
		return
	}

	c.JSON(200, task)
}

// DeleteTask 删除任务
func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")

	var task models.Task
	if err := h.db.First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status == "running" {
		c.JSON(400, gin.H{"error": "无法删除正在运行的任务"})
		return
	}

	if err := h.db.Delete(&task).Error; err != nil {
		c.JSON(500, gin.H{"error": "删除任务失败"})
		return
	}

	c.JSON(200, gin.H{"message": "任务删除成功"})
}

// CancelTask 取消任务
func (h *Handler) CancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.CancelTask(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "任务取消成功"})
}

// GetTaskLogs 获取任务日志
func (h *Handler) GetTaskLogs(c *gin.Context) {
	taskID := c.Param("id")

	var logs []models.TaskLog
	if err := h.db.Where("task_id = ?", taskID).
		Order("timestamp ASC").Find(&logs).Error; err != nil {
		c.JSON(500, gin.H{"error": "查询日志失败"})
		return
	}

	c.JSON(200, logs)
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	var stats struct {
		TotalNodes     int64 `json:"total_nodes"`
		OnlineNodes    int64 `json:"online_nodes"`
		TotalGPUs      int64 `json:"total_gpus"`
		TotalTasks     int64 `json:"total_tasks"`
		PendingTasks   int64 `json:"pending_tasks"`
		RunningTasks   int64 `json:"running_tasks"`
		CompletedTasks int64 `json:"completed_tasks"`
	}

	h.db.Model(&models.Node{}).Count(&stats.TotalNodes)
	h.db.Model(&models.Node{}).Where("status = ?", "online").Count(&stats.OnlineNodes)
	h.db.Model(&models.GPU{}).Count(&stats.TotalGPUs)
	h.db.Model(&models.Task{}).Count(&stats.TotalTasks)
	h.db.Model(&models.Task{}).Where("status = ?", "pending").Count(&stats.PendingTasks)
	h.db.Model(&models.Task{}).Where("status = ?", "running").Count(&stats.RunningTasks)
	h.db.Model(&models.Task{}).Where("status = ?", "completed").Count(&stats.CompletedTasks)

	c.JSON(200, stats)
}

// 认证相关API
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token"`
	User         models.User `json:"user"`
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// LDAP认证
	ldapUser, err := auth.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失败"})
		return
	}

	// 查找或创建用户
	var user models.User
	result := h.db.Where("username = ?", ldapUser.Username).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// 创建新用户
			user = models.User{
				Username:    ldapUser.Username,
				Email:       ldapUser.Email,
				FullName:    ldapUser.FullName,
				DisplayName: ldapUser.FullName,
				Role:        "user",
				IsActive:    true,
			}
			if err := h.db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
			return
		}
	}

	// 生成JWT令牌
	token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成刷新令牌失败"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (h *Handler) RefreshToken(c *gin.Context) {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims, err := auth.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的刷新令牌"})
		return
	}

	var user models.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}

func (h *Handler) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	type UpdateProfileRequest struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.FullName = req.FullName
	user.DisplayName = req.FullName
	user.Email = req.Email

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// HandleWebSocket 处理WebSocket连接
func (h *Handler) HandleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 这里可以实现实时数据推送
	for {
		// 发送实时统计数据
		var stats struct {
			Nodes []models.Node `json:"nodes"`
			Tasks []models.Task `json:"tasks"`
		}

		h.db.Preload("GPUs").Find(&stats.Nodes)
		h.db.Where("status IN ?", []string{"pending", "running"}).Find(&stats.Tasks)

		if err := conn.WriteJSON(stats); err != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}
}
