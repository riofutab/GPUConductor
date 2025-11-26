package api

import (
	"GPUConductor/internal/auth"
	"GPUConductor/internal/logger"
	"GPUConductor/internal/models"
	"GPUConductor/internal/scheduler"
	"GPUConductor/internal/security"
	"GPUConductor/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Handler struct {
	db             *gorm.DB
	scheduler      *scheduler.Scheduler
	redisAddr      string
	redisPassword  string
	redisDB        int
	heartbeatGrace time.Duration
	logger         *zap.Logger
	gitlabBaseURL  string
	gitlabToken    string
	ldapConfig     *auth.LDAPConfig
	minioDefaults  MinioDefaults
}

type MinioDefaults struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	Bucket     string
}

func NewHandler(db *gorm.DB, sched *scheduler.Scheduler, redisAddr string, redisPassword string, redisDB int, heartbeatGrace time.Duration) *Handler {
	err := logger.InitLogger(false)
	if err != nil {
		panic(fmt.Sprintf("初始化日志失败: %v", err))
	}
	h := &Handler{
		db:             db,
		scheduler:      sched,
		redisAddr:      redisAddr,
		redisPassword:  redisPassword,
		redisDB:        redisDB,
		heartbeatGrace: heartbeatGrace,
		logger:         logger.Logger,
		gitlabBaseURL:  defaultGitlabBase(),
		gitlabToken:    os.Getenv("GITLAB_TOKEN"),
		ldapConfig:     nil,
	}
	if db != nil {
		h.loadGitlabConfig()
		h.loadLDAPConfig()
	}
	h.minioDefaults = MinioDefaults{
		Endpoint:  os.Getenv("MINIO_ENDPOINT"),
		AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		SecretKey: os.Getenv("MINIO_SECRET_KEY"),
		Bucket:    os.Getenv("MINIO_BUCKET"),
	}
	return h
}

func defaultGitlabBase() string {
	base := os.Getenv("GITLAB_BASE_URL")
	if strings.TrimSpace(base) == "" {
		return "https://gitlab.com"
	}
	return base
}

func (h *Handler) loadGitlabConfig() {
	var base models.Setting
	if err := h.db.Where("key = ?", "gitlab_base_url").First(&base).Error; err == nil && strings.TrimSpace(base.Value) != "" {
		h.gitlabBaseURL = base.Value
	}
	var token models.Setting
	if err := h.db.Where("key = ?", "gitlab_token").First(&token).Error; err == nil && strings.TrimSpace(token.Value) != "" {
		h.gitlabToken = token.Value
	}
}

func (h *Handler) loadLDAPConfig() {
	var host, baseDN, userDN, bindDN, bindPass, userFilter models.Setting
	_ = h.db.Where("key = ?", "ldap_host").First(&host).Error
	_ = h.db.Where("key = ?", "ldap_base_dn").First(&baseDN).Error
	_ = h.db.Where("key = ?", "ldap_user_dn").First(&userDN).Error
	_ = h.db.Where("key = ?", "ldap_bind_dn").First(&bindDN).Error
	_ = h.db.Where("key = ?", "ldap_bind_pass").First(&bindPass).Error
	_ = h.db.Where("key = ?", "ldap_user_filter").First(&userFilter).Error

	var portSetting models.Setting
	_ = h.db.Where("key = ?", "ldap_port").First(&portSetting).Error
	port, _ := strconv.Atoi(strings.TrimSpace(portSetting.Value))
	if port == 0 {
		port = 389
	}

	if strings.TrimSpace(host.Value) == "" {
		return
	}

	cfg := &auth.LDAPConfig{
		Host:       strings.TrimSpace(host.Value),
		Port:       port,
		BaseDN:     strings.TrimSpace(baseDN.Value),
		UserDN:     strings.TrimSpace(userDN.Value),
		BindDN:     strings.TrimSpace(bindDN.Value),
		BindPass:   strings.TrimSpace(bindPass.Value),
		UserFilter: strings.TrimSpace(userFilter.Value),
	}
	h.ldapConfig = cfg
	auth.InitLDAP(cfg)
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

// GetGitlabConfig 管理员查看GitLab配置（不返回真实token）
func (h *Handler) GetGitlabConfig(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	h.loadGitlabConfig()
	c.JSON(http.StatusOK, gin.H{
		"base_url":  h.gitlabBaseURL,
		"token_set": h.gitlabToken != "",
	})
}

// UpdateGitlabConfig 管理员更新GitLab配置
func (h *Handler) UpdateGitlabConfig(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	var req struct {
		BaseURL string `json:"base_url"`
		Token   string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}

	if strings.TrimSpace(req.BaseURL) == "" {
		req.BaseURL = defaultGitlabBase()
	}

	settings := []models.Setting{
		{Key: "gitlab_base_url", Value: strings.TrimSpace(req.BaseURL)},
	}
	if strings.TrimSpace(req.Token) != "" {
		settings = append(settings, models.Setting{Key: "gitlab_token", Value: strings.TrimSpace(req.Token)})
	}

	for _, s := range settings {
		if err := h.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&s).Error; err != nil {
			h.logger.Error("保存GitLab配置失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存失败"})
			return
		}
	}

	h.gitlabBaseURL = settings[0].Value
	if len(settings) > 1 {
		h.gitlabToken = settings[1].Value
	}

	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// GetUsers 管理员获取用户列表
func (h *Handler) GetUsers(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	var users []models.User
	if err := h.db.Select("id", "username", "display_name", "mobile", "email", "role", "status", "last_login_at").Find(&users).Error; err != nil {
		h.logger.Error("查询用户列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// UpdateUserRole 管理员更新用户角色
func (h *Handler) UpdateUserRole(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	userID := c.Param("id")
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != "admin" && role != "user" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "角色必须为 admin 或 user"})
		return
	}

	if err := h.db.Model(&models.User{}).Where("id = ?", userID).Update("role", role).Error; err != nil {
		h.logger.Error("更新用户角色失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "系统错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// GetLDAPConfig 管理员查看LDAP配置（密码不回显）
func (h *Handler) GetLDAPConfig(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	h.loadLDAPConfig()
	cfg := h.ldapConfig
	if cfg == nil {
		cfg = &auth.LDAPConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"host":         cfg.Host,
		"port":         cfg.Port,
		"base_dn":      cfg.BaseDN,
		"user_dn":      cfg.UserDN,
		"bind_dn":      cfg.BindDN,
		"user_filter":  cfg.UserFilter,
		"password_set": cfg.BindPass != "",
	})
}

// UpdateLDAPConfig 管理员更新LDAP配置
func (h *Handler) UpdateLDAPConfig(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	var req struct {
		Host       string `json:"host"`
		Port       int    `json:"port"`
		BaseDN     string `json:"base_dn"`
		UserDN     string `json:"user_dn"`
		BindDN     string `json:"bind_dn"`
		BindPass   string `json:"bind_pass"`
		UserFilter string `json:"user_filter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数无效"})
		return
	}
	if strings.TrimSpace(req.Host) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "LDAP Host 不能为空"})
		return
	}
	if req.Port == 0 {
		req.Port = 389
	}

	settings := []models.Setting{
		{Key: "ldap_host", Value: strings.TrimSpace(req.Host)},
		{Key: "ldap_port", Value: strconv.Itoa(req.Port)},
		{Key: "ldap_base_dn", Value: strings.TrimSpace(req.BaseDN)},
		{Key: "ldap_user_dn", Value: strings.TrimSpace(req.UserDN)},
		{Key: "ldap_bind_dn", Value: strings.TrimSpace(req.BindDN)},
		{Key: "ldap_user_filter", Value: strings.TrimSpace(req.UserFilter)},
	}
	if strings.TrimSpace(req.BindPass) != "" {
		settings = append(settings, models.Setting{Key: "ldap_bind_pass", Value: req.BindPass})
	}

	for _, s := range settings {
		if err := h.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&s).Error; err != nil {
			h.logger.Error("保存LDAP配置失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存失败"})
			return
		}
	}

	cfg := &auth.LDAPConfig{
		Host:       strings.TrimSpace(req.Host),
		Port:       req.Port,
		BaseDN:     strings.TrimSpace(req.BaseDN),
		UserDN:     strings.TrimSpace(req.UserDN),
		BindDN:     strings.TrimSpace(req.BindDN),
		BindPass:   strings.TrimSpace(req.BindPass),
		UserFilter: strings.TrimSpace(req.UserFilter),
	}
	h.ldapConfig = cfg
	auth.InitLDAP(cfg)

	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
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
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	task.CreatedBy = userID.(string)
	task.Status = "pending"

	// 补全 MinIO 默认值，避免在表单中输入敏感信息
	if task.MinioEndpoint == "" && h.minioDefaults.Endpoint != "" {
		task.MinioEndpoint = h.minioDefaults.Endpoint
	}
	if task.MinioAccessKey == "" && h.minioDefaults.AccessKey != "" {
		task.MinioAccessKey = h.minioDefaults.AccessKey
	}
	if task.MinioSecretKey == "" && h.minioDefaults.SecretKey != "" {
		task.MinioSecretKey = h.minioDefaults.SecretKey
	}
	if task.MinioBucket == "" && h.minioDefaults.Bucket != "" {
		task.MinioBucket = h.minioDefaults.Bucket
	}

	if h.scheduler != nil {
		if err := h.scheduler.SubmitTask(&task); err != nil {
			h.logger.Error("调度器提交任务失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "提交任务失败"})
			return
		}
	} else if err := h.db.Create(&task).Error; err != nil {
		h.logger.Error("创建任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
	if err := h.db.Where("task_id = ?", taskID).
		Order("timestamp ASC").
		Find(&logs).Error; err != nil {
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

// GetGitlabRepos 获取GitLab仓库列表（管理员）
func (h *Handler) GetGitlabRepos(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	if h.gitlabToken == "" {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "GitLab 未配置令牌"})
		return
	}

	groupID := strings.TrimSpace(c.Query("group_id"))
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/api/v4/projects?membership=true&simple=true&per_page=50", strings.TrimRight(h.gitlabBaseURL, "/"))
	if groupID != "" {
		url = fmt.Sprintf("%s/api/v4/groups/%s/projects?simple=true&per_page=50", strings.TrimRight(h.gitlabBaseURL, "/"), groupID)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "创建请求失败"})
		return
	}
	req.Header.Set("PRIVATE-TOKEN", h.gitlabToken)

	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("请求GitLab失败", zap.Error(err))
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "GitLab 请求失败"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Warn("GitLab返回错误", zap.Int("status", resp.StatusCode), zap.ByteString("body", body))
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "GitLab 返回错误"})
		return
	}

	var projects []struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNs    string `json:"path_with_namespace"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		Visibility    string `json:"visibility"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		h.logger.Error("解析GitLab返回失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "解析GitLab返回失败"})
		return
	}

	result := make([]gin.H, 0, len(projects))
	for _, p := range projects {
		result = append(result, gin.H{
			"id":             p.ID,
			"name":           p.Name,
			"path":           p.PathWithNs,
			"http_url":       p.HTTPURL,
			"ssh_url":        p.SSHURL,
			"visibility":     p.Visibility,
			"default_branch": p.DefaultBranch,
		})
	}

	c.JSON(http.StatusOK, result)
}

// GetGitlabGroups 获取GitLab分组（管理员）
func (h *Handler) GetGitlabGroups(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	if h.gitlabToken == "" {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "GitLab 未配置令牌"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v4/groups?all_available=false&per_page=100", strings.TrimRight(h.gitlabBaseURL, "/")), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "创建请求失败"})
		return
	}
	req.Header.Set("PRIVATE-TOKEN", h.gitlabToken)

	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("请求GitLab分组失败", zap.Error(err))
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "GitLab 请求失败"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Warn("GitLab分组返回错误", zap.Int("status", resp.StatusCode), zap.ByteString("body", body))
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "GitLab 返回错误"})
		return
	}

	var groups []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		FullPath string `json:"full_path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		h.logger.Error("解析GitLab分组失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "解析GitLab返回失败"})
		return
	}

	result := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		result = append(result, gin.H{
			"id":        g.ID,
			"name":      g.Name,
			"full_path": g.FullPath,
		})
	}

	c.JSON(http.StatusOK, result)
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

	c.JSON(http.StatusOK, gin.H{
		"redis":    h.redisAddr,
		"password": h.redisPassword,
		"db":       h.redisDB,
	})
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
		Status         string             `json:"status"`
		AssignedNodeID string             `json:"assigned_node_id"`
		ContainerID    string             `json:"container_id"`
		ExitCode       *int               `json:"exit_code"`
		ErrorMessage   string             `json:"error_message"`
		CurrentIter    *int               `json:"current_iteration"`
		Metrics        map[string]float64 `json:"metrics"`
		Environment    map[string]string  `json:"environment"`
		Logs           string             `json:"logs"`
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
	if req.CurrentIter != nil {
		task.CurrentIteration = *req.CurrentIter
	}
	if req.Metrics != nil {
		task.Metrics = req.Metrics
	}

	// 追加容器日志
	if strings.TrimSpace(req.Logs) != "" {
		logEntry := models.TaskLog{
			TaskID:  task.ID,
			Content: req.Logs,
		}
		if err := h.db.Create(&logEntry).Error; err != nil {
			h.logger.Warn("写入任务日志失败", zap.String("taskID", taskID), zap.Error(err))
		}
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

func (h *Handler) requireAdmin(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role == "admin" {
		return true
	}
	c.JSON(http.StatusForbidden, ErrorResponse{Error: "需要管理员权限"})
	return false
}
