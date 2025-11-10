/**
 * @Author: carlo carlo@paeony.org
 * @Date: 2025-09-29 18:41:49
 * @LastEditors: carlo carlo@paeony.org
 * @LastEditTime: 2025-09-29 22:07:37
 * @FilePath: internal/server/server.go
 * @Description: 这是默认设置,可以在设置》工具》File Description中进行配置
 */
package server

import (
	"GPUConductor/internal/api"
	"GPUConductor/internal/auth"
	"GPUConductor/internal/logger"
	"GPUConductor/internal/models"
	"GPUConductor/internal/scheduler"
	"GPUConductor/internal/security"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type            string `mapstructure:"type"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Name            string `mapstructure:"name"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	SSLMode         string `mapstructure:"sslmode"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type Config struct {
	Port                  string
	Database              *DatabaseConfig
	Redis                 *RedisConfig
	LDAP                  *auth.LDAPConfig
	JWT                   *auth.JWTConfig
	HeartbeatGraceSeconds int
}

type Server struct {
	config    *Config
	db        *gorm.DB
	scheduler *scheduler.Scheduler
	router    *gin.Engine
	redisAddr string
}

func New(config *Config) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Start() error {
	// 初始化日志系统
	if err := logger.InitLogger(false); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	defer logger.Sync()

	logger.Info("开始启动GPUConductor服务器",
		zap.String("port", s.config.Port),
		zap.String("database_host", s.config.Database.Host),
		zap.Int("database_port", s.config.Database.Port),
	)

	// 初始化数据库
	if err := s.initDatabase(); err != nil {
		logger.Warn("数据库初始化失败，使用内存模式", zap.Error(err))
		// 在内存模式下继续运行，但某些功能可能受限
	} else {
		logger.Info("数据库连接成功")
	}

	// 初始化LDAP认证
	if err := s.initLDAP(); err != nil {
		logger.Error("LDAP初始化失败", zap.Error(err))
		return fmt.Errorf("LDAP初始化失败: %w", err)
	}
	logger.Info("LDAP认证初始化完成")

	// 初始化JWT配置
	s.initJWT()

	// 初始化调度器
	redisAddr := fmt.Sprintf("%s:%d", s.config.Redis.Host, s.config.Redis.Port)
	s.redisAddr = redisAddr
	s.scheduler = scheduler.New(s.db, redisAddr)
	go s.scheduler.Start()
	logger.Info("任务调度器已启动")

	// 初始化路由
	s.initRoutes()

	// 启动服务器
	logger.Info("GPUConductor服务器启动完成", zap.String("port", s.config.Port))
	log.Printf("GPUConductor服务器启动在端口 %s", s.config.Port)
	return http.ListenAndServe(":"+s.config.Port, s.router)
}

func (s *Server) initDatabase() error {

	// 构建数据库连接字符串
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		s.config.Database.Host,
		s.config.Database.Username,
		s.config.Database.Password,
		s.config.Database.Name,
		s.config.Database.Port,
		s.config.Database.SSLMode,
	)

	logger.Debug("连接数据库",
		zap.String("host", s.config.Database.Host),
		zap.Int("port", s.config.Database.Port),
		zap.String("database", s.config.Database.Name),
	)

	var err error
	s.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("数据库连接失败", zap.Error(err))
		return err
	}

	// 配置连接池
	sqlDB, err := s.db.DB()
	if err != nil {
		logger.Error("获取数据库连接池失败", zap.Error(err))
		return err
	}

	// 设置连接池参数
	if s.config.Database.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(s.config.Database.MaxOpenConns)
	}
	if s.config.Database.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(s.config.Database.MaxIdleConns)
	}
	if s.config.Database.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(s.config.Database.ConnMaxLifetime) * time.Second)
	}

	logger.Debug("数据库连接池配置完成",
		zap.Int("max_open_conns", s.config.Database.MaxOpenConns),
		zap.Int("max_idle_conns", s.config.Database.MaxIdleConns),
		zap.Int("conn_max_lifetime", s.config.Database.ConnMaxLifetime),
	)

	// 自动迁移数据库表
	logger.Info("开始数据库表迁移")
	err = s.db.AutoMigrate(
		&models.User{},
		&models.Node{},
		&models.GPU{},
		&models.Task{},
		&models.TaskLog{},
	)
	if err != nil {
		logger.Error("数据库表迁移失败", zap.Error(err))
		return err
	}

	if err := s.ensureDefaultAdmin(); err != nil {
		logger.Warn("创建默认管理员失败", zap.Error(err))
	}

	logger.Info("数据库表迁移完成")
	return nil
}

func (s *Server) initLDAP() error {
	if s.config.LDAP == nil || s.config.LDAP.Host == "" {
		logger.Warn("LDAP未配置，将使用本地认证")
		return nil
	}

	logger.Info("LDAP配置初始化完成",
		zap.String("host", s.config.LDAP.Host),
		zap.Int("port", s.config.LDAP.Port),
		zap.String("base_dn", s.config.LDAP.BaseDN),
		zap.String("bind_dn", s.config.LDAP.BindDN),
	)

	auth.InitLDAP(s.config.LDAP)
	return nil
}

func (s *Server) initRoutes() {
	gin.SetMode(gin.ReleaseMode)
	s.router = gin.New()
	s.router.Use(gin.Logger(), gin.Recovery())

	// 根路径返回 API 信息
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":      "GPUConductor",
			"version":   "api-only",
			"endpoints": []string{"/api/v1/health", "/api/v1/tasks", "/api/v1/nodes"},
		})
	})

	s.router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
	})

	// API路由
	graceSeconds := s.config.HeartbeatGraceSeconds
	if graceSeconds <= 0 {
		graceSeconds = 60
	}
	apiHandler := api.NewHandler(s.db, s.scheduler, s.redisAddr, time.Duration(graceSeconds)*time.Second)

	// 公开路由
	publicGroup := s.router.Group("/api/v1")
	{
		publicGroup.POST("/auth/login", apiHandler.Login)
		publicGroup.GET("/health", apiHandler.HealthCheck)
		publicGroup.GET("/config/redis", apiHandler.GetRedisConfig)
		publicGroup.POST("/nodes/:id/heartbeat", apiHandler.NodeHeartbeat)
		publicGroup.PUT("/tasks/:id", apiHandler.UpdateTaskStatus)
	}

	// 需要认证的路由
	apiGroup := s.router.Group("/api/v1")
	apiGroup.Use(apiHandler.AuthMiddleware())
	{
		// 用户管理
		apiGroup.GET("/users/profile", apiHandler.GetUserProfile)

		// 节点管理
		apiGroup.GET("/nodes", apiHandler.GetNodes)

		// 任务管理
		apiGroup.GET("/tasks", apiHandler.GetTasks)
		apiGroup.GET("/tasks/:id", apiHandler.GetTask)
		apiGroup.POST("/tasks", apiHandler.CreateTask)
		apiGroup.POST("/tasks/:id/cancel", apiHandler.CancelTask)
		apiGroup.GET("/tasks/:id/logs", apiHandler.GetTaskLogs)

		// 系统统计
		apiGroup.GET("/stats", apiHandler.GetStats)
		apiGroup.GET("/stats/gpus", apiHandler.GetGPUStats)
		apiGroup.GET("/gpu/stats", apiHandler.GetGPUStats)

		// 节点管理
		apiGroup.PUT("/nodes/:id/status", apiHandler.UpdateNodeStatus)
	}

	// WebSocket连接
	// s.router.GET("/ws", apiHandler.HandleWebSocket) // 暂时注释掉，api包中没有此方法
}

func (s *Server) ensureDefaultAdmin() error {
	var count int64
	if err := s.db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	password, err := security.HashPassword("admin123")
	if err != nil {
		return err
	}

	now := time.Now()
	admin := models.User{
		Username:    "admin",
		Mobile:      "13800000000",
		Password:    password,
		Email:       "admin@gcond.local",
		FullName:    "Administrator",
		DisplayName: "Administrator",
		Role:        "admin",
		Status:      "active",
		IsActive:    true,
		LastLogin:   &now,
		LastLoginAt: &now,
	}

	return s.db.Create(&admin).Error
}

func (s *Server) initJWT() {
	cfg := s.config.JWT
	if cfg == nil || cfg.Secret == "" {
		cfg = &auth.JWTConfig{
			Secret:                "change-me-in-production",
			ExpirationHours:       24,
			RefreshExpirationDays: 7,
		}
	}
	if cfg.ExpirationHours == 0 {
		cfg.ExpirationHours = 24
	}
	if cfg.RefreshExpirationDays == 0 {
		cfg.RefreshExpirationDays = 7
	}

	auth.InitJWT(cfg)
}
