package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tp/cowork/internal/worker/executor"
	"github.com/tp/cowork/internal/shared/models"
)

// WorkerConfig Worker 配置
type WorkerConfig struct {
	Name          string
	Tags          []string
	Model         string
	GatewayURL    string
	MaxConcurrent int
	WorkDir       string
}

// GatewayClient Gateway 客户端
type GatewayClient struct {
	baseURL    string
	httpClient *http.Client
	workerID   string
}

// NewGatewayClient 创建 Gateway 客户端
func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
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
func (c *GatewayClient) Register(name string, tags []string, model string, maxConcurrent int) (*RegisterResponse, error) {
	payload := map[string]interface{}{
		"name":           name,
		"tags":           tags,
		"model":          model,
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
		Success bool               `json:"success"`
		Data    RegisterResponse   `json:"data"`
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
func (c *GatewayClient) SendHeartbeat(status string, currentTasks []string, progress map[string]int) (*HeartbeatResponse, error) {
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
func (c *GatewayClient) FetchTask(taskID string) (*models.Task, error) {
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
func (c *GatewayClient) UpdateTask(taskID string, update TaskUpdateRequest) error {
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
func (c *GatewayClient) SendTaskLog(taskID string, level, message string) error {
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
	config     *WorkerConfig
	client     *GatewayClient
	executor   *executor.Executor
	id         string
	status     string
	tasks      map[string]*RunningTask
	tasksMu    sync.RWMutex
	stopCh     chan struct{}
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
	}

	return &Worker{
		config:   cfg,
		client:   NewGatewayClient(cfg.GatewayURL),
		executor: executor.New(execConfig),
		status:   "idle",
		tasks:    make(map[string]*RunningTask),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动 Worker
func (w *Worker) Start() error {
	// 注册到 Gateway
	resp, err := w.client.Register(w.config.Name, w.config.Tags, w.config.Model, w.config.MaxConcurrent)
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	w.id = resp.ID
	w.client.workerID = resp.ID
	log.Printf("Worker registered: id=%s, name=%s", w.id, w.config.Name)

	// 确保工作目录存在
	workDir := w.config.WorkDir
	if workDir == "" {
		workDir = filepath.Join(os.TempDir(), "cowork", w.id)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}
	log.Printf("Work directory: %s", workDir)

	// 启动心跳循环
	go w.heartbeatLoop(resp.HeartbeatInterval)

	return nil
}

// Stop 停止 Worker
func (w *Worker) Stop() {
	close(w.stopCh)
	w.executor.Stop()
	log.Printf("Worker stopped: id=%s", w.id)
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
				log.Printf("Heartbeat failed: %v", err)
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
		log.Printf("Received assigned tasks: %v", resp.AssignedTasks)
		for _, taskID := range resp.AssignedTasks {
			go w.executeTask(taskID)
		}
	}

	// 处理取消的任务
	if len(resp.CancelledTasks) > 0 {
		log.Printf("Received cancelled tasks: %v", resp.CancelledTasks)
		for _, taskID := range resp.CancelledTasks {
			w.cancelTask(taskID)
		}
	}

	return nil
}

// executeTask 执行任务
func (w *Worker) executeTask(taskID string) {
	log.Printf("Executing task: %s", taskID)

	// 获取任务详情
	task, err := w.client.FetchTask(taskID)
	if err != nil {
		log.Printf("Failed to fetch task %s: %v", taskID, err)
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

	// 执行任务
	result := w.executor.Execute(task, callback)

	// 更新任务状态
	if result.Status == models.TaskStatusCompleted {
		w.client.UpdateTask(taskID, TaskUpdateRequest{
			Status:   models.TaskStatusCompleted,
			Progress: 100,
			Output:   result.Output,
		})
		log.Printf("Task completed: %s", taskID)
	} else {
		w.client.UpdateTask(taskID, TaskUpdateRequest{
			Status: models.TaskStatusFailed,
			Error:  result.Error,
		})
		log.Printf("Task failed: %s - %s", taskID, result.Error)
	}

	// 发送日志
	for _, logEntry := range result.Logs {
		w.client.SendTaskLog(taskID, logEntry.Level, logEntry.Message)
	}
}

// cancelTask 取消任务
func (w *Worker) cancelTask(taskID string) {
	w.tasksMu.RLock()
	_, exists := w.tasks[taskID]
	w.tasksMu.RUnlock()

	if exists {
		if err := w.executor.Cancel(taskID); err != nil {
			log.Printf("Failed to cancel task %s: %v", taskID, err)
		} else {
			log.Printf("Task cancelled: %s", taskID)
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

	// 发送到 Gateway
	go c.worker.client.SendTaskLog(taskID, level, message)
}

func (c *taskCallback) OnComplete(taskID string, result *executor.TaskResult) {
	// 任务完成时的回调处理
	log.Printf("Task %s execution finished with status: %s", taskID, result.Status)
}

func main() {
	// 命令行参数
	name := flag.String("name", "", "Worker name (required)")
	tagsStr := flag.String("tags", "", "Worker tags (comma-separated)")
	model := flag.String("model", "", "Default model")
	gateway := flag.String("gateway", "http://localhost:8080", "Gateway URL")
	maxConcurrent := flag.Int("max-concurrent", 1, "Maximum concurrent tasks")
	workDir := flag.String("work-dir", "", "Base work directory")
	flag.Parse()

	// 验证参数
	if *name == "" {
		log.Fatal("Worker name is required")
	}
	if *tagsStr == "" {
		log.Fatal("Worker tags are required")
	}

	// 解析标签
	tags := strings.Split(*tagsStr, ",")
	for i, tag := range tags {
		tags[i] = strings.TrimSpace(tag)
	}

	// 创建 Worker
	cfg := &WorkerConfig{
		Name:          *name,
		Tags:          tags,
		Model:         *model,
		GatewayURL:    *gateway,
		MaxConcurrent: *maxConcurrent,
		WorkDir:       *workDir,
	}

	worker := NewWorker(cfg)

	// 启动 Worker
	if err := worker.Start(); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	log.Printf("Worker started: name=%s, tags=%v, model=%s", cfg.Name, cfg.Tags, cfg.Model)

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 停止 Worker
	worker.Stop()
	log.Println("Worker shutdown complete")
}

// readFile 辅助函数
func readFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}