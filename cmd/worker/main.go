package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tp/cowork/internal/shared/config"
	"github.com/tp/cowork/internal/shared/logger"
	"github.com/tp/cowork/internal/shared/models"
	"github.com/tp/cowork/internal/shared/utils"
	"github.com/tp/cowork/internal/worker/ai"
	"github.com/tp/cowork/internal/worker/executor"
)

// WorkerConfig Worker 配置
type WorkerConfig struct {
	Name           string
	Tags           []string
	CoordinatorURL string
	MaxConcurrent  int
	WorkDir        string
	DockerEnabled  bool
	DockerImage    string

	// AI 配置
	AIBaseURL     string
	AIModel       string
	AIAPIKey      string
	SystemPrompt  string
	AIMaxTokens   int
	AITemperature float64
	Description   string

	// 工具配置
	EnabledTools  []string
	AllowedPaths  []string
	ShellTimeout  int
}

// CoordinatorClient Coordinator 客户端
type CoordinatorClient struct {
	baseURL    string
	httpClient *http.Client
	workerID   string
}

// NewCoordinatorClient 创建 Coordinator 客户端
func NewCoordinatorClient(baseURL string) *CoordinatorClient {
	return &CoordinatorClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	Success           bool   `json:"success"`
	ID                string `json:"id"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	HeartbeatInterval int    `json:"heartbeat_interval"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success        bool     `json:"success"`
	AssignedTasks  []string `json:"assigned_tasks"`
	CancelledTasks []string `json:"cancelled_tasks"`
	Commands       []string `json:"commands"`
}

// TaskResponse 任务响应
type TaskResponse struct {
	Success bool        `json:"success"`
	Data    models.Task `json:"data"`
}

// TaskUpdateRequest 任务更新请求
type TaskUpdateRequest struct {
	Status   models.TaskStatus `json:"status"`
	Progress int               `json:"progress"`
	Output   models.JSON       `json:"output"`
	Error    string            `json:"error"`
}

// TaskLogRequest 任务日志请求
type TaskLogRequest struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// ApprovalRequest 审批请求
type ApprovalRequest struct {
	TaskID    string                 `json:"task_id"`
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	RiskLevel string                 `json:"risk_level"`
}

// ApprovalResponse 审批响应
type ApprovalResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"` // pending, approved, rejected
}

// Register 注册 Worker
func (c *CoordinatorClient) Register(name string, tags []string, maxConcurrent int, description string, capabilities map[string]interface{}) (*RegisterResponse, error) {
	payload := map[string]interface{}{
		"name":           name,
		"tags":           tags,
		"max_concurrent": maxConcurrent,
	}

	// 添加描述和能力信息
	if description != "" {
		payload["description"] = description
	}
	if capabilities != nil {
		payload["capabilities"] = capabilities
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/workers/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool             `json:"success"`
		Data    RegisterResponse `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("register failed: %s", result.Error.Message)
	}

	c.workerID = result.Data.ID

	return &result.Data, nil
}

// SendHeartbeat 发送心跳
func (c *CoordinatorClient) SendHeartbeat(status string, currentTasks []string, progress map[string]int) (*HeartbeatResponse, error) {
	payload := map[string]interface{}{
		"status":        status,
		"current_tasks": currentTasks,
		"progress":      progress,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/workers/%s/heartbeat", c.baseURL, c.workerID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool              `json:"success"`
		Data    HeartbeatResponse `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("heartbeat failed: %s", result.Error.Message)
	}

	return &result.Data, nil
}

// FetchTask 获取任务详情
func (c *CoordinatorClient) FetchTask(taskID string) (*models.Task, error) {
	url := fmt.Sprintf("%s/api/tasks/%s", c.baseURL, taskID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("failed to fetch task")
	}

	return &result.Data, nil
}

// UpdateTask 更新任务状态
func (c *CoordinatorClient) UpdateTask(taskID string, update TaskUpdateRequest) error {
	body, err := json.Marshal(update)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/tasks/%s/status", c.baseURL, taskID)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// SendTaskLog 发送任务日志
func (c *CoordinatorClient) SendTaskLog(taskID string, level, message string) error {
	payload := TaskLogRequest{
		Level:   level,
		Message: message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/tasks/%s/logs", c.baseURL, taskID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// RequestApproval 请求审批
func (c *CoordinatorClient) RequestApproval(taskID string, toolName string, args map[string]interface{}) (*ApprovalResponse, error) {
	req := ApprovalRequest{
		TaskID:    taskID,
		ToolName:  toolName,
		Arguments: args,
		RiskLevel: "high",
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", c.baseURL+"/api/approvals", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ApprovalResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// GetApproval 查询审批状态
func (c *CoordinatorClient) GetApproval(approvalID string) (*ApprovalResponse, error) {
	httpReq, _ := http.NewRequest("GET", c.baseURL+"/api/approvals/"+approvalID, nil)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ApprovalResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// WaitApproval 等待审批结果（轮询）
func (c *CoordinatorClient) WaitApproval(approvalID string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		result, err := c.GetApproval(approvalID)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		switch result.Status {
		case "approved":
			return true, nil
		case "rejected":
			return false, fmt.Errorf("approval rejected")
		}

		time.Sleep(1 * time.Second)
	}

	return false, fmt.Errorf("approval timeout")
}

// Worker 工作节点
type Worker struct {
	config         *WorkerConfig
	client         *CoordinatorClient
	executor       *executor.Executor
	toolExecutor   *executor.ToolExecutor
	dockerExecutor *executor.DockerExecutor
	aiClient       *ai.AIClient
	id             string
	status         string
	tasks          map[string]*RunningTask
	tasksMu        sync.RWMutex
	stopCh         chan struct{}
}

// RunningTask 运行中的任务
type RunningTask struct {
	ID        string
	Task      *models.Task
	Progress  int
	StartTime time.Time
	Logs      []LogEntry
}

// LogEntry 日志条目
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// NewWorker 创建 Worker
func NewWorker(cfg *WorkerConfig) *Worker {
	execConfig := executor.DefaultConfig()
	if cfg.WorkDir != "" {
		execConfig.BaseWorkDir = cfg.WorkDir
	} else {
		// 使用 ~/.cowork/workers/{name}/workspace 作为工作目录
		execConfig.BaseWorkDir = config.GetWorkerWorkspaceDir(cfg.Name)
	}
	// 设置允许访问的路径
	execConfig.AllowedPaths = cfg.AllowedPaths
	slog.Info("Worker config", "allowed_paths", cfg.AllowedPaths, "work_dir", execConfig.BaseWorkDir)

	worker := &Worker{
		config:   cfg,
		client:   NewCoordinatorClient(cfg.CoordinatorURL),
		executor: executor.New(execConfig),
		status:   "idle",
		tasks:    make(map[string]*RunningTask),
		stopCh:   make(chan struct{}),
	}

	// 初始化 AI 客户端（如果配置了）
	if cfg.AIBaseURL != "" && cfg.AIModel != "" && cfg.AIAPIKey != "" {
		worker.aiClient = ai.NewAIClient(cfg.AIBaseURL, cfg.AIModel, cfg.AIAPIKey)
		worker.executor.SetAIClient(worker.aiClient)
		worker.executor.SetSystemPrompt(cfg.SystemPrompt)
		worker.executor.SetEnabledTools(cfg.EnabledTools)
		worker.executor.SetAllowedPaths(cfg.AllowedPaths)
		slog.Info("AI client initialized", "model", cfg.AIModel, "base_url", cfg.AIBaseURL)
	} else {
		slog.Warn("AI client not configured, agent tasks will use fallback execution")
	}

	// 初始化工具执行器（使用远程注册表客户端）
	toolRegistry := NewRemoteToolRegistry(cfg.CoordinatorURL)
	worker.toolExecutor = executor.NewToolExecutor(execConfig, toolRegistry)

	// 初始化 Docker 执行器（如果启用）
	if cfg.DockerEnabled {
		dockerConfig := executor.DockerConfigFromEnv()
		dockerConfig.Enabled = true
		if cfg.DockerImage != "" {
			dockerConfig.DefaultImage = cfg.DockerImage
		}
		if cfg.WorkDir != "" {
			dockerConfig.WorkDirBase = cfg.WorkDir
		} else {
			// 使用 ~/.cowork/workers/{name}/workspace 作为 Docker 工作目录
			dockerConfig.WorkDirBase = config.GetWorkerWorkspaceDir(cfg.Name)
		}

		dockerExec, err := executor.NewDockerExecutor(dockerConfig)
		if err != nil {
			slog.Warn("Docker executor not available, falling back to local execution", "error", err)
		} else {
			worker.dockerExecutor = dockerExec
			worker.toolExecutor.SetDockerExecutor(dockerExec)
			slog.Info("Docker executor enabled", "image", dockerConfig.DefaultImage)
		}
	}

	return worker
}

// Start 启动 Worker
func (w *Worker) Start() error {
	// 构建能力信息
	capabilities := map[string]interface{}{
		"tools": w.config.EnabledTools,
	}
	if w.dockerExecutor != nil {
		capabilities["docker"] = true
	}

	// 注册到 Coordinator
	resp, err := w.client.Register(w.config.Name, w.config.Tags, w.config.MaxConcurrent, w.config.Description, capabilities)
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	w.id = resp.ID
	w.client.workerID = resp.ID
	slog.Info("Worker registered", "id", w.id, "name", w.config.Name)

	// 确保工作目录存在
	// 默认使用 ~/.cowork/workers/{worker-name}/workspace，持久化且避免与其他进程冲突
	workDir := w.config.WorkDir
	if workDir == "" {
		workDir = config.GetWorkerWorkspaceDir(w.config.Name)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}
	slog.Info("Work directory ready", "path", workDir)

	// 启动心跳循环
	go w.heartbeatLoop(resp.HeartbeatInterval)

	return nil
}

// Stop 停止 Worker
func (w *Worker) Stop() {
	close(w.stopCh)
	w.executor.Stop()
	if w.dockerExecutor != nil {
		w.dockerExecutor.Cleanup()
	}
	slog.Info("Worker stopped", "id", w.id)
}

// heartbeatLoop 心跳循环
func (w *Worker) heartbeatLoop(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.sendHeartbeat(); err != nil {
				slog.Error("Heartbeat failed", "error", err)
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (w *Worker) sendHeartbeat() error {
	// 收集当前任务和进度
	w.tasksMu.RLock()
	currentTasks := make([]string, 0, len(w.tasks))
	progress := make(map[string]int)
	for id, rt := range w.tasks {
		currentTasks = append(currentTasks, id)
		progress[id] = rt.Progress
	}
	w.tasksMu.RUnlock()

	// 确定状态
	status := "idle"
	if len(currentTasks) > 0 {
		status = "busy"
	}

	resp, err := w.client.SendHeartbeat(status, currentTasks, progress)
	if err != nil {
		return err
	}

	// 处理分配的任务
	if len(resp.AssignedTasks) > 0 {
		slog.Info("Received assigned tasks", "tasks", resp.AssignedTasks)
		for _, taskID := range resp.AssignedTasks {
			go w.executeTask(taskID)
		}
	}

	// 处理取消的任务
	if len(resp.CancelledTasks) > 0 {
		slog.Info("Received cancelled tasks", "tasks", resp.CancelledTasks)
		for _, taskID := range resp.CancelledTasks {
			w.cancelTask(taskID)
		}
	}

	return nil
}

// executeTask 执行任务
func (w *Worker) executeTask(taskID string) {
	slog.Info("Executing task", "task_id", taskID)

	// 获取任务详情
	task, err := w.client.FetchTask(taskID)
	if err != nil {
		slog.Error("Failed to fetch task", "task_id", taskID, "error", err)
		return
	}

	// 创建运行时任务
	rt := &RunningTask{
		ID:        taskID,
		Task:      task,
		Progress:  0,
		StartTime: time.Now(),
		Logs:      make([]LogEntry, 0),
	}

	// 注册任务
	w.tasksMu.Lock()
	w.tasks[taskID] = rt
	w.tasksMu.Unlock()

	// 清理
	defer func() {
		w.tasksMu.Lock()
		delete(w.tasks, taskID)
		w.tasksMu.Unlock()
	}()

	// 创建回调
	callback := &taskCallback{
		worker: w,
		taskID: taskID,
	}

	// 根据任务类型选择执行器
	var result *executor.TaskResult

	switch task.Type {
	case "tool":
		// 工具执行任务 - 使用 ToolExecutor
		slog.Info("Executing tool task", "task_id", taskID, "tool_name", task.ToolName)
		result = w.toolExecutor.ExecuteToolFromTask(task, callback)

	case "shell", "script":
		// Shell/脚本任务
		if w.shouldUseDocker(task) {
			if w.dockerExecutor != nil {
				slog.Info("Using Docker executor", "task_id", taskID)
				result = w.dockerExecutor.Execute(task, callback)
			} else {
				slog.Warn("Docker requested but not available, using local executor", "task_id", taskID)
				result = w.executor.Execute(task, callback)
			}
		} else {
			result = w.executor.Execute(task, callback)
		}

	case "docker", "sandbox":
		// Docker 隔离任务
		if w.dockerExecutor != nil {
			slog.Info("Using Docker executor for isolated task", "task_id", taskID)
			result = w.dockerExecutor.Execute(task, callback)
		} else {
			result = &executor.TaskResult{
				TaskID: taskID,
				Status: models.TaskStatusFailed,
				Error:  "Docker executor not available for isolated task",
			}
		}

	default:
		// 默认使用标准执行器
		result = w.executor.Execute(task, callback)
	}

	// 更新任务状态
	if result.Status == models.TaskStatusCompleted {
		w.client.UpdateTask(taskID, TaskUpdateRequest{
			Status:   models.TaskStatusCompleted,
			Progress: 100,
			Output:   result.Output,
		})
		slog.Info("Task completed", "task_id", taskID)
	} else {
		w.client.UpdateTask(taskID, TaskUpdateRequest{
			Status: models.TaskStatusFailed,
			Error:  result.Error,
		})
		slog.Error("Task failed", "task_id", taskID, "error", result.Error)
	}

	// 发送日志
	for _, logEntry := range result.Logs {
		w.client.SendTaskLog(taskID, logEntry.Level, logEntry.Message)
	}
}

// shouldUseDocker 判断是否应该使用 Docker 执行
func (w *Worker) shouldUseDocker(task *models.Task) bool {
	// 如果 Docker 未启用，不使用
	if w.dockerExecutor == nil {
		return false
	}

	// 检查任务类型
	if task.Type == "docker" || task.Type == "sandbox" {
		return true
	}

	// 检查输入中的显式指定
	if useDocker, ok := task.Input["use_docker"].(bool); ok && useDocker {
		return true
	}

	// 检查任务标签
	for _, tag := range task.RequiredTags {
		if tag == "docker" || tag == "isolated" || tag == "sandbox" {
			return true
		}
	}

	return false
}

// cancelTask 取消任务
func (w *Worker) cancelTask(taskID string) {
	w.tasksMu.RLock()
	_, exists := w.tasks[taskID]
	w.tasksMu.RUnlock()

	if exists {
		if err := w.executor.Cancel(taskID); err != nil {
			slog.Error("Failed to cancel task", "task_id", taskID, "error", err)
		} else {
			slog.Info("Task cancelled", "task_id", taskID)
		}
	}
}

// taskCallback 任务回调
type taskCallback struct {
	worker *Worker
	taskID string
}

func (c *taskCallback) OnProgress(taskID string, progress int) {
	c.worker.tasksMu.Lock()
	if rt, exists := c.worker.tasks[taskID]; exists {
		rt.Progress = progress
	}
	c.worker.tasksMu.Unlock()
}

func (c *taskCallback) OnLog(taskID string, level, message string) {
	c.worker.tasksMu.Lock()
	if rt, exists := c.worker.tasks[taskID]; exists {
		rt.Logs = append(rt.Logs, LogEntry{
			Time:    time.Now(),
			Level:   level,
			Message: message,
		})
	}
	c.worker.tasksMu.Unlock()

	// 发送到 Coordinator
	go c.worker.client.SendTaskLog(taskID, level, message)
}

func (c *taskCallback) OnComplete(taskID string, result *executor.TaskResult) {
	// 任务完成时的回调处理
	slog.Info("Task execution finished", "task_id", taskID, "status", string(result.Status))
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

	// 基本命令行参数
	name := flag.String("name", "", "Worker name (required)")
	tagsStr := flag.String("tags", "", "Worker tags (comma-separated)")
	coordinator := flag.String("coordinator", "http://localhost:8080", "Coordinator URL")
	maxConcurrent := flag.Int("max-concurrent", 1, "Maximum concurrent tasks")
	workDir := flag.String("work-dir", "", "Base work directory")
	dockerEnabled := flag.Bool("docker", false, "Enable Docker executor")
	dockerImage := flag.String("docker-image", "alpine:latest", "Default Docker image for tasks")

	// AI 配置参数
	aiBaseURL := flag.String("ai-base-url", "", "AI API base URL (required for agent tasks)")
	aiModel := flag.String("ai-model", "", "AI model name (required for agent tasks)")
	aiAPIKey := flag.String("ai-api-key", "", "AI API key (can also use AI_API_KEY env var)")
	systemPrompt := flag.String("system-prompt", "", "System prompt for AI agent")
	systemPromptFile := flag.String("system-prompt-file", "", "Path to file containing system prompt")
	description := flag.String("description", "", "Worker description for coordinator")
	aiMaxTokens := flag.Int("ai-max-tokens", 4096, "Maximum tokens for AI response")
	aiTemperature := flag.Float64("ai-temperature", 0.7, "Temperature for AI response")

	// 工具配置参数
	toolsStr := flag.String("tools", "shell,file", "Enabled tools (comma-separated: shell,file,web)")
	allowedPathsStr := flag.String("allowed-paths", "", "Allowed paths for file operations (comma-separated)")
	shellTimeout := flag.Int("shell-timeout", 300, "Shell command timeout in seconds")

	flag.Parse()

	// 验证必需参数
	if *name == "" {
		slog.Error("Worker name is required")
		os.Exit(1)
	}

	// 尝试从配置文件加载配置
	configPath := config.GetSettingFilePath()
	workerConfig := loadWorkerConfig(*name, configPath)

	// 合并配置：命令行参数优先
	cfg := &WorkerConfig{
		Name:           *name,
		CoordinatorURL: *coordinator,
		MaxConcurrent:  *maxConcurrent,
		WorkDir:        *workDir,
		DockerEnabled:  *dockerEnabled,
		DockerImage:    *dockerImage,
	}

	// 标签配置
	if *tagsStr != "" {
		cfg.Tags = utils.ParseStringList(*tagsStr)
	} else if len(workerConfig.Tags) > 0 {
		cfg.Tags = workerConfig.Tags
	} else {
		slog.Error("Worker tags are required")
		os.Exit(1)
	}

	// AI 配置合并
	if *aiBaseURL != "" {
		cfg.AIBaseURL = *aiBaseURL
	} else if workerConfig.AIBaseURL != "" {
		cfg.AIBaseURL = workerConfig.AIBaseURL
	}

	if *aiModel != "" {
		cfg.AIModel = *aiModel
	} else if workerConfig.AIModel != "" {
		cfg.AIModel = workerConfig.AIModel
	}

	// API Key: 命令行 > 配置文件 > 环境变量
	if *aiAPIKey != "" {
		cfg.AIAPIKey = *aiAPIKey
	} else if workerConfig.AIAPIKey != "" {
		cfg.AIAPIKey = utils.ExpandEnvVars(workerConfig.AIAPIKey)
	} else {
		cfg.AIAPIKey = os.Getenv("AI_API_KEY")
	}

	// System prompt: 命令行 > 文件 > 配置文件
	if *systemPrompt != "" {
		cfg.SystemPrompt = *systemPrompt
	} else if *systemPromptFile != "" {
		data, err := os.ReadFile(*systemPromptFile)
		if err != nil {
			slog.Error("Failed to read system prompt file", "error", err)
			os.Exit(1)
		}
		cfg.SystemPrompt = string(data)
	} else if workerConfig.SystemPrompt != "" {
		cfg.SystemPrompt = workerConfig.SystemPrompt
	} else {
		// 默认系统提示词
		cfg.SystemPrompt = getDefaultSystemPrompt(cfg.Name)
	}

	// 其他 AI 配置
	cfg.AIMaxTokens = *aiMaxTokens
	if workerConfig.AIMaxTokens > 0 && *aiMaxTokens == 4096 {
		cfg.AIMaxTokens = workerConfig.AIMaxTokens
	}
	cfg.AITemperature = *aiTemperature
	if workerConfig.AITemperature > 0 && *aiTemperature == 0.7 {
		cfg.AITemperature = workerConfig.AITemperature
	}

	// 描述配置
	if *description != "" {
		cfg.Description = *description
	} else if workerConfig.Description != "" {
		cfg.Description = workerConfig.Description
	}

	// 工具配置
	if *toolsStr != "" {
		cfg.EnabledTools = utils.ParseStringList(*toolsStr)
	} else {
		cfg.EnabledTools = []string{"shell", "file"}
	}

	if *allowedPathsStr != "" {
		cfg.AllowedPaths = utils.ParseStringList(*allowedPathsStr)
	} else if len(workerConfig.AllowedPaths) > 0 {
		cfg.AllowedPaths = workerConfig.AllowedPaths
	}
	cfg.ShellTimeout = *shellTimeout

	// 创建 Worker
	worker := NewWorker(cfg)

	// 启动 Worker
	if err := worker.Start(); err != nil {
		slog.Error("Failed to start worker", "error", err)
		os.Exit(1)
	}

	slog.Info("Worker started",
		"name", cfg.Name,
		"tags", cfg.Tags,
		"ai_model", cfg.AIModel,
		"tools", cfg.EnabledTools,
	)

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 停止 Worker
	worker.Stop()
	slog.Info("Worker shutdown complete")
}

// WorkerConfigFromFile 配置文件中的 Worker 配置
type WorkerConfigFromFile struct {
	Tags           []string `json:"tags"`
	AIBaseURL      string   `json:"ai_base_url"`
	AIModel        string   `json:"ai_model"`
	AIAPIKey       string   `json:"ai_api_key"`
	SystemPrompt   string   `json:"system_prompt"`
	AIMaxTokens    int      `json:"ai_max_tokens"`
	AITemperature  float64  `json:"ai_temperature"`
	Description    string   `json:"description"`
	AllowedPaths   []string `json:"allowed_paths"`
}

// SettingFile 配置文件结构
type SettingFile struct {
	Workers map[string]WorkerConfigFromFile `json:"worker"`
}

// loadWorkerConfig 从配置文件加载 Worker 配置
func loadWorkerConfig(name, configPath string) WorkerConfigFromFile {
	var cfg WorkerConfigFromFile

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

	if workerCfg, ok := settings.Workers[name]; ok {
		cfg = workerCfg
		slog.Info("Loaded config from file", "worker", name)
	}

	return cfg
}

// getDefaultSystemPrompt 获取默认系统提示词
func getDefaultSystemPrompt(workerName string) string {
	return fmt.Sprintf(`You are an AI agent running as worker "%s".

Your role is to execute tasks assigned by the coordinator. You have access to tools for:
- Executing shell commands
- Reading and writing files
- Other operations as configured

When executing tasks:
1. Understand the task requirements clearly
2. Plan your approach before executing
3. Use tools appropriately and safely
4. Report progress and results accurately
5. Handle errors gracefully

Always be helpful, accurate, and safe in your operations.`, workerName)
}

// RemoteToolRegistry 远程工具注册表客户端
type RemoteToolRegistry struct {
	baseURL    string
	httpClient *http.Client
	cache      map[string]*models.ToolDefinition
}

// NewRemoteToolRegistry 创建远程工具注册表客户端
func NewRemoteToolRegistry(baseURL string) *RemoteToolRegistry {
	return &RemoteToolRegistry{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      make(map[string]*models.ToolDefinition),
	}
}

// Get 获取工具定义
func (r *RemoteToolRegistry) Get(toolName string) (*models.ToolDefinition, error) {
	// 检查缓存
	if tool, ok := r.cache[toolName]; ok {
		return tool, nil
	}

	// 从 Coordinator 获取
	url := fmt.Sprintf("%s/api/tools/%s", r.baseURL, toolName)
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tool definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	var result struct {
		Success bool                      `json:"success"`
		Data    *models.ToolDefinition    `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success || result.Data == nil {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// 缓存结果
	r.cache[toolName] = result.Data

	return result.Data, nil
}

// GetBuiltinTools 获取内置工具列表（简化版，直接返回已知工具）
func (r *RemoteToolRegistry) GetBuiltinTools() []*models.ToolDefinition {
	// 预定义的内置工具（与 Coordinator 端 builtin.go 保持一致）
	return []*models.ToolDefinition{
		{
			Name:        "execute_shell",
			Description: "在隔离环境中执行 Shell 命令",
			Parameters: models.JSON{
				"type": "object",
				"properties": models.JSON{
					"command": models.JSON{
						"type":        "string",
						"description": "要执行的 Shell 命令",
					},
					"work_dir": models.JSON{
						"type":        "string",
						"description": "工作目录（可选）",
					},
					"timeout": models.JSON{
						"type":        "integer",
						"description": "超时时间（秒），默认 300",
					},
				},
				"required": []string{"command"},
			},
			Category:    models.ToolCategorySystem,
			ExecuteMode: models.ToolExecuteModeRemote,
			Permission:  models.ToolPermissionExecute,
			Handler:     "execute_shell",
			IsEnabled:   true,
			IsBuiltin:   true,
		},
		{
			Name:        "read_file",
			Description: "读取文件内容",
			Parameters: models.JSON{
				"type": "object",
				"properties": models.JSON{
					"path": models.JSON{
						"type":        "string",
						"description": "文件路径",
					},
				},
				"required": []string{"path"},
			},
			Category:    models.ToolCategoryFile,
			ExecuteMode: models.ToolExecuteModeRemote,
			Permission:  models.ToolPermissionRead,
			Handler:     "read_file",
			IsEnabled:   true,
			IsBuiltin:   true,
		},
		{
			Name:        "write_file",
			Description: "写入文件内容",
			Parameters: models.JSON{
				"type": "object",
				"properties": models.JSON{
					"path": models.JSON{
						"type":        "string",
						"description": "文件路径",
					},
					"content": models.JSON{
						"type":        "string",
						"description": "文件内容",
					},
				},
				"required": []string{"path", "content"},
			},
			Category:    models.ToolCategoryFile,
			ExecuteMode: models.ToolExecuteModeRemote,
			Permission:  models.ToolPermissionWrite,
			Handler:     "write_file",
			IsEnabled:   true,
			IsBuiltin:   true,
		},
	}
}
