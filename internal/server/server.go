package server

import (
	"GPUConductor/internal/api"
	"GPUConductor/internal/auth"
	"GPUConductor/internal/middleware"
	"GPUConductor/internal/models"
	"GPUConductor/internal/scheduler"
	"GPUConductor/web"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	Port     string
	Database string
	Redis    string
	LDAP     *auth.LDAPConfig
	JWT      *auth.JWTConfig
}

type Server struct {
	config    *Config
	db        *gorm.DB
	scheduler *scheduler.Scheduler
	router    *gin.Engine
}

func New(config *Config) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Start() error {
	// 初始化数据库
	if err := s.initDatabase(); err != nil {
		return fmt.Errorf("数据库初始化失败: %w", err)
	}

	// 初始化调度器
	s.scheduler = scheduler.New(s.db, s.config.Redis)
	go s.scheduler.Start()

	// 初始化路由
	s.initRoutes()

	// 启动服务器
	log.Printf("GPUConductor服务器启动在端口 %s", s.config.Port)
	return http.ListenAndServe(":"+s.config.Port, s.router)
}

func (s *Server) initDatabase() error {
	var err error
	s.db, err = gorm.Open(postgres.Open(s.config.Database), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移数据库表
	return s.db.AutoMigrate(
		&models.User{},
		&models.Node{},
		&models.GPU{},
		&models.Task{},
		&models.TaskLog{},
	)
}

func (s *Server) initRoutes() {
	gin.SetMode(gin.ReleaseMode)
	s.router = gin.New()
	s.router.Use(gin.Logger(), gin.Recovery())

	// 静态文件服务 (内嵌前端)
	s.router.StaticFS("/static", http.FS(web.StaticFiles))
	s.router.GET("/", func(c *gin.Context) {
		data, err := web.IndexHTML.ReadFile("dist/index.html")
		if err != nil {
			c.String(500, "无法加载前端页面")
			return
		}
		c.Data(200, "text/html; charset=utf-8", data)
	})

	// 初始化认证
	auth.InitLDAP(s.config.LDAP)
	auth.InitJWT(s.config.JWT)

	// API路由
	apiHandler := api.New(s.db, s.scheduler)

	// 公开路由
	publicGroup := s.router.Group("/api/v1")
	{
		publicGroup.POST("/auth/login", apiHandler.Login)
		publicGroup.POST("/auth/refresh", apiHandler.RefreshToken)
	}

	// 需要认证的路由
	apiGroup := s.router.Group("/api/v1")
	apiGroup.Use(middleware.AuthMiddleware())
	{
		// 用户管理
		apiGroup.GET("/users/profile", apiHandler.GetProfile)
		apiGroup.PUT("/users/profile", apiHandler.UpdateProfile)

		// 节点管理
		apiGroup.GET("/nodes", apiHandler.GetNodes)
		apiGroup.GET("/nodes/:id", apiHandler.GetNode)
		apiGroup.POST("/nodes/:id/heartbeat", apiHandler.NodeHeartbeat)
		apiGroup.PUT("/nodes/:id/gpus", apiHandler.UpdateNodeGPUs)

		// 任务管理
		apiGroup.GET("/tasks", apiHandler.GetTasks)
		apiGroup.GET("/tasks/:id", apiHandler.GetTask)
		apiGroup.POST("/tasks", apiHandler.CreateTask)
		apiGroup.PUT("/tasks/:id", apiHandler.UpdateTask)
		apiGroup.DELETE("/tasks/:id", apiHandler.DeleteTask)
		apiGroup.POST("/tasks/:id/cancel", apiHandler.CancelTask)
		apiGroup.GET("/tasks/:id/logs", apiHandler.GetTaskLogs)

		// 统计信息
		apiGroup.GET("/stats", apiHandler.GetStats)
	}

	// WebSocket连接
	s.router.GET("/ws", apiHandler.HandleWebSocket)
}
