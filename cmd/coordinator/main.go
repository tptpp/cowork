package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	sloggin "github.com/samber/slog-gin"
	"github.com/tp/cowork/internal/coordinator/agent"
	"github.com/tp/cowork/internal/coordinator/approval"
	"github.com/tp/cowork/internal/coordinator/handler"
	"github.com/tp/cowork/internal/coordinator/message"
	"github.com/tp/cowork/internal/coordinator/middleware"
	"github.com/tp/cowork/internal/coordinator/node"
	"github.com/tp/cowork/internal/coordinator/scheduler"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/coordinator/ws"
	"github.com/tp/cowork/internal/shared/config"
	"github.com/tp/cowork/internal/shared/logger"
	"github.com/tp/cowork/internal/shared/utils"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该限制
	},
}

// CoordinatorConfig 协调者配置
type CoordinatorConfig struct {
	AIBaseURL string `json:"ai_base_url"`
	AIModel   string `json:"ai_model"`
	AIAPIKey  string `json:"ai_api_key"`

	// 调度器配置
	Scheduler SchedulerConfig `json:"scheduler"`
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	PollInterval     string `json:"poll_interval"`      // 轮询间隔
	WorkerTimeout    string `json:"worker_timeout"`     // Worker 超时时间
	MaxRetryAttempts int    `json:"max_retry_attempts"` // 最大重试次数
	TaskTimeout      string `json:"task_timeout"`       // 任务超时时间
}

// SettingFile 配置文件结构
type SettingFile struct {
	Coordinator CoordinatorConfig `json:"coordinator"`
}

// loadCoordinatorConfig 从配置文件加载 Coordinator 配置
func loadCoordinatorConfig(configPath string) CoordinatorConfig {
	var cfg CoordinatorConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read config file", "path", configPath, "error", err)
		}
		return cfg
	}

	var settings SettingFile
	if err := json.Unmarshal(data, &settings); err != nil {
		slog.Warn("Failed to parse config file", "path", configPath, "error", err)
		return cfg
	}

	cfg = settings.Coordinator
	if cfg.AIAPIKey != "" {
		cfg.AIAPIKey = utils.ExpandEnvVars(cfg.AIAPIKey)
	}

	slog.Info("Loaded coordinator config from file", "ai_model", cfg.AIModel)
	return cfg
}

func main() {
	// 初始化日志
	logLevel := os.Getenv("COWORK_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := os.Getenv("COWORK_LOG_FORMAT")
	if logFormat == "" {
		logFormat = "text"
	}
	logger.Init(logLevel, logFormat)

	// 加载配置文件
	configPath := config.GetSettingFilePath()
	coordCfg := loadCoordinatorConfig(configPath)

	// 初始化数据库
	dbPath := os.Getenv("COWORK_DB_PATH")
	if dbPath == "" {
		// 使用 ~/.cowork/coordinator/cowork.db 作为默认数据库路径
		dbPath = config.GetCoordinatorDBPath()
	}

	// 确保数据库目录存在
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		slog.Error("Failed to create database directory", "error", err)
		os.Exit(1)
	}

	s, err := store.New(store.Config{Path: dbPath})
	if err != nil {
		slog.Error("Failed to initialize store", "error", err)
		os.Exit(1)
	}
	defer s.Close()

	slog.Info("Database initialized", "path", dbPath)

	// 初始化 WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// 构建调度器配置
	schedCfg := scheduler.DefaultConfig()
	if coordCfg.Scheduler.PollInterval != "" {
		if d, err := time.ParseDuration(coordCfg.Scheduler.PollInterval); err == nil {
			schedCfg.PollInterval = d
		}
	}
	if coordCfg.Scheduler.WorkerTimeout != "" {
		if d, err := time.ParseDuration(coordCfg.Scheduler.WorkerTimeout); err == nil {
			schedCfg.WorkerTimeout = d
		}
	}
	if coordCfg.Scheduler.MaxRetryAttempts > 0 {
		schedCfg.MaxRetryAttempts = coordCfg.Scheduler.MaxRetryAttempts
	}
	if coordCfg.Scheduler.TaskTimeout != "" {
		if d, err := time.ParseDuration(coordCfg.Scheduler.TaskTimeout); err == nil {
			schedCfg.TaskTimeout = d
		}
	}

	// 初始化任务调度器
	taskScheduler := scheduler.New(
		schedCfg,
		store.NewTaskStore(s.DB()),
		store.NewWorkerStore(s.DB()),
		hub,
	)
	taskScheduler.Start()
	defer taskScheduler.Stop()

	// 初始化工具注册中心
	toolRegistry := tools.NewRegistry(store.NewToolDefinitionStore(s.DB()))
	if err := toolRegistry.Initialize(); err != nil {
		slog.Error("Failed to initialize tool registry", "error", err)
		os.Exit(1)
	}
	slog.Info("Tool registry initialized", "count", len(toolRegistry.GetBuiltinTools()))

	// ========== 新增服务初始化 ==========

	// 初始化 Node Registry 和 Scheduler
	nodeRegistry := node.NewRegistry(store.NewNodeStore(s.DB()))
	nodeScheduler := node.NewScheduler(
		nodeRegistry,
		store.NewNodeStore(s.DB()),
		store.NewTaskStore(s.DB()),
		store.NewTaskDependencyStore(s.DB()),
	)
	slog.Info("Node registry and scheduler initialized")

	// 初始化 Message Router
	msgRouter := message.NewRouter(
		store.NewMessageStore(s.DB()),
		store.NewTaskStore(s.DB()),
		s.DB(),
	)
	slog.Info("Message router initialized")

	// 初始化 Agent Template Manager
	templateManager := agent.NewTemplateManager(store.NewAgentTemplateStore(s.DB()))
	if err := templateManager.InitSystemTemplates(context.Background()); err != nil {
		slog.Warn("Failed to initialize system templates", "error", err)
	} else {
		slog.Info("System agent templates initialized")
	}

	// 初始化 Approval Service
	approvalService := approval.NewService(
		store.NewApprovalRequestStore(s.DB()),
		store.NewApprovalPolicyStore(s.DB()),
	)
	slog.Info("Approval service initialized")

	// 初始化 Approval Handler
	approvalHandler := handler.NewApprovalHandler(approvalService)

	// ========== 新增 Handler ==========

	// 初始化 Message Handler
	messageHandler := handler.NewMessageHandler(msgRouter, store.NewMessageStore(s.DB()))

	// 初始化 Node Handler
	nodeHandler := handler.NewNodeHandler(nodeRegistry, nodeScheduler)

	// 初始化处理器
	h := handler.NewHandler(
		store.NewTaskStore(s.DB()),
		store.NewWorkerStore(s.DB()),
		store.NewTaskLogStore(s.DB()),
		store.NewNotificationStore(s.DB()),
		store.NewUserLayoutStore(s.DB()),
		store.NewAgentSessionStore(s.DB()),
		store.NewToolExecutionStore(s.DB()),
		store.NewTaskFileStore(s.DB()),
		hub,
		taskScheduler,
		toolRegistry,
	)

	// 从配置文件加载 AI 配置
	if coordCfg.AIBaseURL != "" && coordCfg.AIModel != "" && coordCfg.AIAPIKey != "" {
		// 根据配置推断模型类型
		modelType := "default"
		if strings.Contains(coordCfg.AIModel, "gpt") || strings.Contains(coordCfg.AIBaseURL, "openai") {
			modelType = "openai"
		} else if strings.Contains(coordCfg.AIModel, "claude") || strings.Contains(coordCfg.AIBaseURL, "anthropic") {
			modelType = "anthropic"
		} else if strings.Contains(coordCfg.AIModel, "glm") || strings.Contains(coordCfg.AIBaseURL, "bigmodel") || strings.Contains(coordCfg.AIBaseURL, "dashscope") {
			modelType = "glm"
		}
		h.SetAIConfig(modelType, coordCfg.AIBaseURL, coordCfg.AIModel, coordCfg.AIAPIKey)
		slog.Info("AI config loaded from file", "type", modelType, "model", coordCfg.AIModel)

		// 初始化 ConversationCoordinator（支持 Function Calling + 任务拆解）
		engine := agent.NewFunctionCallingEngine(
			toolRegistry,
			store.NewToolExecutionStore(s.DB()),
			store.NewTaskStore(s.DB()),
			agent.FunctionCallingConfig{
				MaxToolRounds: 10,
				Timeout:       60 * time.Second,
			},
		)

		// 创建 Agent 工具调度器（用于 Function Calling 的工具执行）
		agentToolScheduler := agent.NewToolScheduler(
			toolRegistry,
			store.NewToolExecutionStore(s.DB()),
			store.NewTaskStore(s.DB()),
			nil, // taskCreator - 暂时不支持远程工具执行
			10,  // maxConcurrent
		)

		// 启动工具调度器
		agentToolScheduler.Start(context.Background())
		slog.Info("Tool scheduler started")

		// 使用带任务拆解功能的 Coordinator
		coordinator := agent.NewConversationCoordinatorWithDecomposer(
			engine,
			agentToolScheduler,
			store.NewAgentSessionStore(s.DB()),
			store.NewToolExecutionStore(s.DB()),
			toolRegistry,
			store.NewAgentTemplateStore(s.DB()),
			store.NewTaskStore(s.DB()),
			store.NewTaskGroupStore(s.DB()),
			store.NewTaskDependencyStore(s.DB()),
			agent.CoordinatorConfig{
				MaxToolRounds: 10,
			},
		)
		h.SetAgentCoordinator(coordinator)
		slog.Info("Agent coordinator initialized with Function Calling + Task Decomposition support")
	}

	// 创建 Gin 路由
	r := gin.New()
	r.Use(sloggin.New(slog.Default()))
	r.Use(gin.Recovery())

	// CORS 配置
	corsOrigins := os.Getenv("COWORK_CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "*" // 默认允许所有来源
	}

	corsConfig := middleware.DefaultCORSConfig()
	corsConfig.AllowedOrigins = middleware.ParseOrigins(corsOrigins)
	slog.Info("CORS allowed origins", "origins", corsConfig.AllowedOrigins)

	// CORS 中间件
	r.Use(middleware.CORS(corsConfig))

	// API 认证配置
	authConfig := middleware.DefaultAuthConfig()
	if len(authConfig.APIKeys) > 0 {
		slog.Info("API Key authentication enabled", "count", len(authConfig.APIKeys))
	}
	if authConfig.JWTSecret != "" {
		slog.Info("JWT authentication enabled")
	}

	// 健康检查（无需认证）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// WebSocket 路由（无需认证）
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Warn("WebSocket upgrade error", "error", err)
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
		api.GET("/tasks/:id/files", h.GetTaskFiles)       // 任务文件列表
		api.POST("/tasks/:id/files", h.UploadTaskFile)    // 上传任务文件

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
		// Agent Function Calling API
		api.POST("/agent/sessions/:id/messages/tools", h.SendAgentMessageWithTools)
		api.GET("/agent/sessions/:id/tools/executions", h.GetAgentToolExecutions)
		api.POST("/agent/sessions/:id/tools/execute", h.ExecuteToolCall)
		api.GET("/agent/tools", h.GetAvailableTools)
		api.GET("/agent/tools/:name", h.GetToolDefinition)
		api.POST("/agent/sessions/:id/files", h.UploadAgentFile) // Agent 会话文件上传

		// Tool API
		api.GET("/tools", h.GetTools)
		api.GET("/tools/:name", h.GetTool)
		api.POST("/tools", h.CreateTool)
		api.PUT("/tools/:name", h.UpdateTool)
		api.DELETE("/tools/:name", h.DeleteTool)
		api.POST("/tools/:name/enable", h.EnableTool)
		api.POST("/tools/:name/disable", h.DisableTool)

		// Notification API
		api.GET("/notifications", h.GetNotifications)
		api.PUT("/notifications/read", h.MarkNotificationsRead)
		api.PUT("/notifications/read-all", h.MarkAllNotificationsRead)

		// File API
		api.POST("/files/upload", h.UploadFile)
		api.GET("/files/:id", h.DownloadFile)
		api.DELETE("/files/:id", h.DeleteFile)

		// ========== 新增 API ==========

		// Message API (Agent间通信)
		messageHandler.RegisterRoutes(api)

		// Node API (节点管理)
		nodeHandler.RegisterRoutes(api)

		// Approval API (审批)
		approvalHandler.RegisterRoutes(api)

		// Agent Template API
		api.GET("/agent/templates", func(c *gin.Context) {
			templates, err := templateManager.List(c.Request.Context())
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, templates)
		})
		api.GET("/agent/templates/:id", func(c *gin.Context) {
			id := c.Param("id")
			template, err := templateManager.Get(c.Request.Context(), id)
			if err != nil {
				c.JSON(404, gin.H{"error": "template not found"})
				return
			}
			c.JSON(200, template)
		})
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
		slog.Info("Coordinator starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down Coordinator...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("Server shutdown error", "error", err)
	}

	slog.Info("Coordinator stopped")
}