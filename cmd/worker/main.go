package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tp/cowork/internal/shared/config"
	"github.com/tp/cowork/internal/shared/logger"
	"github.com/tp/cowork/internal/shared/models"
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

// Register 注册 Worker
func (c *CoordinatorClient) Register(name string, tags []string, maxConcurrent int) (*RegisterResponse, error) {
	payload := map[string]interface{}{
		"name":           name,
		"tags":           tags,
		"max_concurrent": maxConcurrent,
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

// Worker 工作节点
type Worker struct {
	config         *WorkerConfig
	client         *CoordinatorClient
	executor       *executor.Executor
	toolExecutor   *executor.ToolExecutor
	dockerExecutor *executor.DockerExecutor
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

	worker := &Worker{
		config:   cfg,
		client:   NewCoordinatorClient(cfg.CoordinatorURL),
		executor: executor.New(execConfig),
		status:   "idle",
		tasks:    make(map[string]*RunningTask),
		stopCh:   make(chan struct{}),
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
			log.Warn().Err(err).Msg("Docker executor not available, falling back to local execution")
		} else {
			worker.dockerExecutor = dockerExec
			worker.toolExecutor.SetDockerExecutor(dockerExec)
			log.Info().Str("image", dockerConfig.DefaultImage).Msg("Docker executor enabled")
		}
	}

	return worker
}

// Start 启动 Worker
func (w *Worker) Start() error {
	// 注册到 Coordinator
	resp, err := w.client.Register(w.config.Name, w.config.Tags, w.config.MaxConcurrent)
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	w.id = resp.ID
	w.client.workerID = resp.ID
	log.Info().Str("id", w.id).Str("name", w.config.Name).Msg("Worker registered")

	// 确保工作目录存在
	// 默认使用 ~/.cowork/workers/{worker-name}/workspace，持久化且避免与其他进程冲突
	workDir := w.config.WorkDir
	if workDir == "" {
		workDir = config.GetWorkerWorkspaceDir(w.config.Name)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}
	log.Info().Str("path", workDir).Msg("Work directory ready")

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
	log.Info().Str("id", w.id).Msg("Worker stopped")
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
				log.Error().Err(err).Msg("Heartbeat failed")
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
		log.Info().Strs("tasks", resp.AssignedTasks).Msg("Received assigned tasks")
		for _, taskID := range resp.AssignedTasks {
			go w.executeTask(taskID)
		}
	}

	// 处理取消的任务
	if len(resp.CancelledTasks) > 0 {
		log.Info().Strs("tasks", resp.CancelledTasks).Msg("Received cancelled tasks")
		for _, taskID := range resp.CancelledTasks {
			w.cancelTask(taskID)
		}
	}

	return nil
}

// executeTask 执行任务
func (w *Worker) executeTask(taskID string) {
	log.Info().Str("task_id", taskID).Msg("Executing task")

	// 获取任务详情
	task, err := w.client.FetchTask(taskID)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to fetch task")
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
		log.Info().Str("task_id", taskID).Str("tool_name", task.ToolName).Msg("Executing tool task")
		result = w.toolExecutor.ExecuteToolFromTask(task, callback)

	case "shell", "script":
		// Shell/脚本任务
		if w.shouldUseDocker(task) {
			if w.dockerExecutor != nil {
				log.Info().Str("task_id", taskID).Msg("Using Docker executor")
				result = w.dockerExecutor.Execute(task, callback)
			} else {
				log.Warn().Str("task_id", taskID).Msg("Docker requested but not available, using local executor")
				result = w.executor.Execute(task, callback)
			}
		} else {
			result = w.executor.Execute(task, callback)
		}

	case "docker", "sandbox":
		// Docker 隔离任务
		if w.dockerExecutor != nil {
			log.Info().Str("task_id", taskID).Msg("Using Docker executor for isolated task")
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
		log.Info().Str("task_id", taskID).Msg("Task completed")
	} else {
		w.client.UpdateTask(taskID, TaskUpdateRequest{
			Status: models.TaskStatusFailed,
			Error:  result.Error,
		})
		log.Error().Str("task_id", taskID).Str("error", result.Error).Msg("Task failed")
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
			log.Error().Err(err).Str("task_id", taskID).Msg("Failed to cancel task")
		} else {
			log.Info().Str("task_id", taskID).Msg("Task cancelled")
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
	log.Info().Str("task_id", taskID).Str("status", string(result.Status)).Msg("Task execution finished")
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
	logger.Configure(logger.Config{
		Level:  logLevel,
		Format: logFormat,
	})

	// 命令行参数
	name := flag.String("name", "", "Worker name (required)")
	tagsStr := flag.String("tags", "", "Worker tags (comma-separated)")
	coordinator := flag.String("coordinator", "http://localhost:8080", "Coordinator URL")
	maxConcurrent := flag.Int("max-concurrent", 1, "Maximum concurrent tasks")
	workDir := flag.String("work-dir", "", "Base work directory")
	dockerEnabled := flag.Bool("docker", false, "Enable Docker executor")
	dockerImage := flag.String("docker-image", "alpine:latest", "Default Docker image for tasks")
	flag.Parse()

	// 验证参数
	if *name == "" {
		log.Fatal().Msg("Worker name is required")
	}
	if *tagsStr == "" {
		log.Fatal().Msg("Worker tags are required")
	}

	// 解析标签
	tags := strings.Split(*tagsStr, ",")
	for i, tag := range tags {
		tags[i] = strings.TrimSpace(tag)
	}

	// 创建 Worker
	cfg := &WorkerConfig{
		Name:           *name,
		Tags:           tags,
		CoordinatorURL: *coordinator,
		MaxConcurrent:  *maxConcurrent,
		WorkDir:        *workDir,
		DockerEnabled:  *dockerEnabled,
		DockerImage:    *dockerImage,
	}

	worker := NewWorker(cfg)

	// 启动 Worker
	if err := worker.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start worker")
	}

	log.Info().Str("name", cfg.Name).Strs("tags", cfg.Tags).Msg("Worker started")

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 停止 Worker
	worker.Stop()
	log.Info().Msg("Worker shutdown complete")
}

// readFile 辅助函数
func readFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
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