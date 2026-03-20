package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/tp/cowork/internal/coordinator/handler"
	"github.com/tp/cowork/internal/coordinator/middleware"
	"github.com/tp/cowork/internal/coordinator/scheduler"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该限制
	},
}

func main() {
	// 初始化数据库
	dbPath := os.Getenv("COWORK_DB_PATH")
	if dbPath == "" {
		dbPath = "./cowork.db"
	}

	s, err := store.New(store.Config{Path: dbPath})
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer s.Close()

	log.Printf("Database initialized: %s", dbPath)

	// 初始化 WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// 初始化任务调度器
	taskScheduler := scheduler.New(
		scheduler.DefaultConfig(),
		store.NewTaskStore(s.DB()),
		store.NewWorkerStore(s.DB()),
		hub,
	)
	taskScheduler.Start()
	defer taskScheduler.Stop()

	// 初始化处理器
	h := handler.NewHandler(
		store.NewTaskStore(s.DB()),
		store.NewWorkerStore(s.DB()),
		store.NewTaskLogStore(s.DB()),
		store.NewNotificationStore(s.DB()),
		store.NewUserLayoutStore(s.DB()),
		store.NewAgentSessionStore(s.DB()),
		hub,
		taskScheduler,
	)

	// 创建 Gin 路由
	r := gin.Default()

	// CORS 配置
	corsOrigins := os.Getenv("COWORK_CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "*" // 默认允许所有来源
	}

	corsConfig := middleware.DefaultCORSConfig()
	corsConfig.AllowedOrigins = middleware.ParseOrigins(corsOrigins)
	log.Printf("CORS allowed origins: %v", corsConfig.AllowedOrigins)

	// CORS 中间件
	r.Use(middleware.CORS(corsConfig))

	// API 认证配置
	authConfig := middleware.DefaultAuthConfig()
	if len(authConfig.APIKeys) > 0 {
		log.Printf("API Key authentication enabled with %d keys", len(authConfig.APIKeys))
	}
	if authConfig.JWTSecret != "" {
		log.Println("JWT authentication enabled")
	}

	// 健康检查（无需认证）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// WebSocket 路由（无需认证）
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		client := ws.NewClient(hub, conn)
		client.Register()

		// 启动读写协程
		go client.WritePump()
		go client.ReadPump()
	})

	// API 路由
	api := r.Group("/api")
	api.Use(middleware.Auth(authConfig))
	{
		// 系统 API
		api.GET("/system/stats", h.GetSystemStats)

		// 任务 API
		api.GET("/tasks", h.GetTasks)
		api.POST("/tasks", h.CreateTask)
		api.GET("/tasks/:id", h.GetTask)
		api.PUT("/tasks/:id/status", h.UpdateTaskStatus)
		api.POST("/tasks/:id/logs", h.CreateTaskLog)
		api.DELETE("/tasks/:id", h.CancelTask) // 统一使用 DELETE 方法取消任务
		api.GET("/tasks/:id/logs", h.GetTaskLogs)

		// Worker API
		api.GET("/workers", h.GetWorkers)
		api.POST("/workers/register", h.RegisterWorker)
		api.POST("/workers/:id/heartbeat", h.WorkerHeartbeat)
		api.GET("/workers/:id", h.GetWorker)
		api.DELETE("/workers/:id", h.UnregisterWorker)

		// User Layout API
		api.GET("/user/layout", h.GetLayout)
		api.PUT("/user/layout", h.SaveLayout)

		// Agent API
		api.GET("/agent/sessions", h.GetAgentSessions)
		api.POST("/agent/sessions", h.CreateAgentSession)
		api.GET("/agent/sessions/:id", h.GetAgentSession)
		api.DELETE("/agent/sessions/:id", h.DeleteAgentSession)
		api.POST("/agent/sessions/:id/messages", h.SendAgentMessage)
		api.GET("/agent/sessions/:id/messages", h.GetAgentMessages)

		// Notification API
		api.GET("/notifications", h.GetNotifications)
		api.PUT("/notifications/read", h.MarkNotificationsRead)
		api.PUT("/notifications/read-all", h.MarkAllNotificationsRead)
	}

	// 启动 HTTP 服务器
	addr := os.Getenv("COWORK_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// 优雅关闭
	go func() {
		log.Printf("Coordinator starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Coordinator...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Coordinator stopped")
}