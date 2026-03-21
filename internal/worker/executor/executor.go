package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tp/cowork/internal/shared/models"
)

// 危险命令黑名单
var dangerousCommands = []string{
	"rm -rf /",
	"rm -rf /*",
	"mkfs",
	"dd if=/dev/zero",
	"dd if=/dev/urandom",
	":(){ :|:& };:",  // fork bomb
	"chmod -R 777 /",
	"chown -R",
	"> /dev/sda",
	"> /dev/hda",
	"wget http",
	"curl http",
	"nc -l",
	"netcat -l",
	"shutdown",
	"reboot",
	"init 0",
	"init 6",
	"systemctl stop",
	"service stop",
}

// 危险命令正则表达式
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)rm\s+-rf\s+/(?:\s|$)`),
	regexp.MustCompile(`(?i)rm\s+-rf\s+/\*`),
	regexp.MustCompile(`(?i)mkfs\.`),
	regexp.MustCompile(`(?i)dd\s+if=/dev/zero`),
	regexp.MustCompile(`(?i)dd\s+if=/dev/urandom`),
	regexp.MustCompile(`(?i)chmod\s+-R\s+777\s+/`),
	regexp.MustCompile(`(?i)chown\s+-R\s+\S+\s+/`),
	regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`),
	regexp.MustCompile(`(?i)>\s*/dev/hd[a-z]`),
	regexp.MustCompile(`(?i)wget\s+http`),
	regexp.MustCompile(`(?i)curl\s+http`),
	regexp.MustCompile(`(?i)nc\s+-l`),
	regexp.MustCompile(`(?i)netcat\s+-l`),
	regexp.MustCompile(`(?i)shutdown\b`),
	regexp.MustCompile(`(?i)reboot\b`),
	regexp.MustCompile(`(?i)init\s+[06]`),
	regexp.MustCompile(`(?i)systemctl\s+stop`),
	regexp.MustCompile(`(?i)service\s+\S+\s+stop`),
}

// SecurityError 安全错误
type SecurityError struct {
	Command string
	Reason  string
}

func (e *SecurityError) Error() string {
	return fmt.Sprintf("command blocked for security: %s (%s)", e.Command, e.Reason)
}

// Config 执行器配置
type Config struct {
	BaseWorkDir    string        // 基础工作目录
	MaxExecTime    time.Duration // 最大执行时间
	CleanupOnExit  bool          // 退出时清理工作目录
	EnableSandbox  bool          // 是否启用沙箱
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		// 使用 /tmp/cowork-worker 作为基础路径，避免与协调者二进制文件（可能在 /tmp/cowork）冲突
		BaseWorkDir:    "/tmp/cowork-worker",
		MaxExecTime:    30 * time.Minute,
		CleanupOnExit:  true,
		EnableSandbox:  false,
	}
}

// isCommandDangerous 检查命令是否危险
func isCommandDangerous(cmdStr string) (bool, string) {
	// 标准化命令（去除多余空格）
	normalizedCmd := strings.TrimSpace(cmdStr)
	lowerCmd := strings.ToLower(normalizedCmd)

	// 检查黑名单
	for _, dangerous := range dangerousCommands {
		if strings.Contains(lowerCmd, strings.ToLower(dangerous)) {
			return true, fmt.Sprintf("matches dangerous pattern: %s", dangerous)
		}
	}

	// 检查正则表达式
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(normalizedCmd) {
			return true, fmt.Sprintf("matches dangerous pattern: %s", pattern.String())
		}
	}

	// 检查管道到危险命令
	parts := strings.Split(normalizedCmd, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		for _, dangerous := range dangerousCommands {
			if strings.Contains(strings.ToLower(part), strings.ToLower(dangerous)) {
				return true, fmt.Sprintf("pipes to dangerous command: %s", part)
			}
		}
	}

	// 检查命令替换
	if strings.Contains(normalizedCmd, "$(") || strings.Contains(normalizedCmd, "`") {
		// 递归检查命令替换中的内容
		innerCmds := extractCommandSubstitutions(normalizedCmd)
		for _, innerCmd := range innerCmds {
			if dangerous, reason := isCommandDangerous(innerCmd); dangerous {
				return true, fmt.Sprintf("command substitution contains dangerous command: %s", reason)
			}
		}
	}

	return false, ""
}

// extractCommandSubstitutions 提取命令替换中的命令
func extractCommandSubstitutions(cmdStr string) []string {
	var result []string

	// 提取 $() 中的内容
	dollarStart := strings.Index(cmdStr, "$(")
	for dollarStart != -1 {
		depth := 1
		start := dollarStart + 2
		i := start
		for i < len(cmdStr) && depth > 0 {
			if cmdStr[i] == '(' {
				depth++
			} else if cmdStr[i] == ')' {
				depth--
			}
			i++
		}
		if depth == 0 {
			result = append(result, cmdStr[start:i-1])
		}
		remaining := cmdStr[i:]
		dollarStart = strings.Index(remaining, "$(")
		if dollarStart != -1 {
			dollarStart += i
		}
	}

	// 提取 `` 中的内容
	backtickStart := strings.Index(cmdStr, "`")
	for backtickStart != -1 && backtickStart < len(cmdStr)-1 {
		backtickEnd := strings.Index(cmdStr[backtickStart+1:], "`")
		if backtickEnd == -1 {
			break
		}
		backtickEnd += backtickStart + 1
		result = append(result, cmdStr[backtickStart+1:backtickEnd])
		remaining := cmdStr[backtickEnd+1:]
		backtickStart = strings.Index(remaining, "`")
		if backtickStart != -1 {
			backtickStart += backtickEnd + 1
		}
	}

	return result
}

// validateCommand 验证命令是否安全
func validateCommand(cmdStr string) error {
	// 空命令允许通过（会在后面处理）
	if strings.TrimSpace(cmdStr) == "" {
		return nil
	}

	dangerous, reason := isCommandDangerous(cmdStr)
	if dangerous {
		return &SecurityError{
			Command: cmdStr,
			Reason:  reason,
		}
	}

	return nil
}

// Executor 任务执行器
type Executor struct {
	config     Config
	running    map[string]*RunningTask
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// RunningTask 正在运行的任务
type RunningTask struct {
	TaskID     string
	WorkDir    string
	Cmd        *exec.Cmd
	CancelFunc context.CancelFunc
	StartTime  time.Time
	Logs       []LogEntry
	LogMu      sync.RWMutex
}

// LogEntry 日志条目
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// TaskResult 任务执行结果
type TaskResult struct {
	TaskID    string
	Status    models.TaskStatus
	Output    models.JSON
	Error     string
	Logs      []LogEntry
	StartTime time.Time
	EndTime   time.Time
}

// Callback 执行回调接口
type Callback interface {
	OnProgress(taskID string, progress int)
	OnLog(taskID string, level, message string)
	OnComplete(taskID string, result *TaskResult)
}

// New 创建新的执行器
func New(cfg Config) *Executor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Executor{
		config:  cfg,
		running: make(map[string]*RunningTask),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Stop 停止执行器
func (e *Executor) Stop() {
	e.cancel()

	e.mu.Lock()
	for _, rt := range e.running {
		rt.CancelFunc()
		if rt.Cmd != nil && rt.Cmd.Process != nil {
			rt.Cmd.Process.Kill()
		}
	}
	e.mu.Unlock()
}

// PrepareWorkDir 准备工作目录
func (e *Executor) PrepareWorkDir(taskID string) (string, error) {
	workDir := filepath.Join(e.config.BaseWorkDir, taskID)

	// 创建目录
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	// 创建子目录
	subDirs := []string{"input", "output", "logs"}
	for _, sub := range subDirs {
		if err := os.MkdirAll(filepath.Join(workDir, sub), 0755); err != nil {
			return "", fmt.Errorf("failed to create subdirectory %s: %w", sub, err)
		}
	}

	return workDir, nil
}

// CleanupWorkDir 清理工作目录
func (e *Executor) CleanupWorkDir(taskID string) error {
	if !e.config.CleanupOnExit {
		return nil
	}

	workDir := filepath.Join(e.config.BaseWorkDir, taskID)
	return os.RemoveAll(workDir)
}

// Execute 执行任务
func (e *Executor) Execute(task *models.Task, callback Callback) *TaskResult {
	result := &TaskResult{
		TaskID:    task.ID,
		StartTime: time.Now(),
	}

	// 准备工作目录
	workDir, err := e.PrepareWorkDir(task.ID)
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = err.Error()
		result.EndTime = time.Now()
		return result
	}

	// 创建运行时任务
	ctx, cancel := context.WithTimeout(e.ctx, e.config.MaxExecTime)
	rt := &RunningTask{
		TaskID:     task.ID,
		WorkDir:    workDir,
		CancelFunc: cancel,
		StartTime:  time.Now(),
		Logs:       make([]LogEntry, 0),
	}

	// 注册运行任务
	e.mu.Lock()
	e.running[task.ID] = rt
	e.mu.Unlock()

	// 清理
	defer func() {
		e.mu.Lock()
		delete(e.running, task.ID)
		e.mu.Unlock()
		e.CleanupWorkDir(task.ID)
	}()

	// 记录日志
	e.addLog(rt, "info", fmt.Sprintf("Task started: %s", task.ID))
	e.addLog(rt, "info", fmt.Sprintf("Work directory: %s", workDir))
	callback.OnLog(task.ID, "info", fmt.Sprintf("Task started in %s", workDir))

	// 根据任务类型执行
	switch task.Type {
	case "shell":
		result = e.executeShell(ctx, rt, task, callback)
	case "script":
		result = e.executeScript(ctx, rt, task, callback)
	case "agent":
		result = e.executeAgent(ctx, rt, task, callback)
	default:
		result = e.executeDefault(ctx, rt, task, callback)
	}

	// 收集日志
	rt.LogMu.RLock()
	result.Logs = make([]LogEntry, len(rt.Logs))
	copy(result.Logs, rt.Logs)
	rt.LogMu.RUnlock()

	result.EndTime = time.Now()

	// 回调完成
	callback.OnComplete(task.ID, result)

	return result
}

// executeShell 执行 Shell 命令
func (e *Executor) executeShell(ctx context.Context, rt *RunningTask, task *models.Task, callback Callback) *TaskResult {
	result := &TaskResult{
		TaskID: task.ID,
	}

	// 从任务配置获取命令
	cmdStr, _ := task.Input["command"].(string)
	if cmdStr == "" {
		result.Status = models.TaskStatusFailed
		result.Error = "no command specified"
		return result
	}

	// 安全检查：验证命令是否包含危险操作
	if err := validateCommand(cmdStr); err != nil {
		e.addLog(rt, "error", fmt.Sprintf("Security check failed: %v", err))
		result.Status = models.TaskStatusFailed
		result.Error = err.Error()
		return result
	}

	// 创建命令
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = rt.WorkDir

	// 设置环境变量
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("COWORK_TASK_ID=%s", task.ID),
		fmt.Sprintf("COWORK_WORK_DIR=%s", rt.WorkDir),
	)

	// 获取输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to create stdout pipe: %v", err)
		return result
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to create stderr pipe: %v", err)
		return result
	}

	rt.Cmd = cmd

	// 启动命令
	if err := cmd.Start(); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to start command: %v", err)
		return result
	}

	// 读取输出
	outputCh := make(chan struct{})
	go e.readOutput(rt, stdout, "info", callback)
	go e.readOutput(rt, stderr, "error", callback)

	// 等待完成
	err = cmd.Wait()
	close(outputCh)

	if ctx.Err() == context.DeadlineExceeded {
		result.Status = models.TaskStatusFailed
		result.Error = "execution timeout"
	} else if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = err.Error()
	} else {
		result.Status = models.TaskStatusCompleted
		result.Output = models.JSON{
			"exit_code": 0,
		}
	}

	return result
}

// executeScript 执行脚本
func (e *Executor) executeScript(ctx context.Context, rt *RunningTask, task *models.Task, callback Callback) *TaskResult {
	result := &TaskResult{
		TaskID: task.ID,
	}

	// 从任务配置获取脚本内容
	script, _ := task.Input["script"].(string)
	if script == "" {
		result.Status = models.TaskStatusFailed
		result.Error = "no script specified"
		return result
	}

	// 写入脚本文件
	scriptPath := filepath.Join(rt.WorkDir, "script.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to write script: %v", err)
		return result
	}

	e.addLog(rt, "info", fmt.Sprintf("Script written to %s", scriptPath))

	// 执行脚本
	task.Input["command"] = scriptPath
	return e.executeShell(ctx, rt, task, callback)
}

// executeAgent 执行 Agent 任务
func (e *Executor) executeAgent(ctx context.Context, rt *RunningTask, task *models.Task, callback Callback) *TaskResult {
	result := &TaskResult{
		TaskID: task.ID,
	}

	// Agent 任务需要连接到 AI 服务
	// 这里是一个简化的实现
	e.addLog(rt, "info", "Starting agent task...")

	// 模拟进度更新
	for i := 0; i <= 100; i += 10 {
		select {
		case <-ctx.Done():
			result.Status = models.TaskStatusFailed
			result.Error = "task cancelled"
			return result
		case <-time.After(500 * time.Millisecond):
			callback.OnProgress(task.ID, i)
		}
	}

	result.Status = models.TaskStatusCompleted
	result.Output = models.JSON{
		"message": "Agent task completed",
	}

	return result
}

// executeDefault 默认执行器
func (e *Executor) executeDefault(ctx context.Context, rt *RunningTask, task *models.Task, callback Callback) *TaskResult {
	result := &TaskResult{
		TaskID: task.ID,
	}

	e.addLog(rt, "info", fmt.Sprintf("Executing task type: %s", task.Type))

	// 模拟执行
	for i := 0; i <= 100; i += 20 {
		select {
		case <-ctx.Done():
			result.Status = models.TaskStatusFailed
			result.Error = "task cancelled"
			return result
		case <-time.After(200 * time.Millisecond):
			callback.OnProgress(task.ID, i)
		}
	}

	result.Status = models.TaskStatusCompleted
	result.Output = models.JSON{
		"message": fmt.Sprintf("Task %s completed", task.Type),
	}

	return result
}

// readOutput 读取命令输出
func (e *Executor) readOutput(rt *RunningTask, reader interface{}, level string, callback Callback) {
	var scanner interface{ Scan() bool; Text() string }

	switch r := reader.(type) {
	case interface{ Scan() bool; Text() string }:
		scanner = r
	default:
		return
	}

	for scanner.Scan() {
		line := scanner.Text()
		e.addLog(rt, level, line)
		callback.OnLog(rt.TaskID, level, line)
	}
}

// addLog 添加日志
func (e *Executor) addLog(rt *RunningTask, level, message string) {
	rt.LogMu.Lock()
	defer rt.LogMu.Unlock()

	rt.Logs = append(rt.Logs, LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	})
}

// Cancel 取消任务
func (e *Executor) Cancel(taskID string) error {
	e.mu.RLock()
	rt, exists := e.running[taskID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task not found or not running")
	}

	rt.CancelFunc()

	if rt.Cmd != nil && rt.Cmd.Process != nil {
		rt.Cmd.Process.Kill()
	}

	return nil
}

// GetRunningTasks 获取正在运行的任务
func (e *Executor) GetRunningTasks() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	taskIDs := make([]string, 0, len(e.running))
	for id := range e.running {
		taskIDs = append(taskIDs, id)
	}
	return taskIDs
}

// GetTaskLogs 获取任务日志
func (e *Executor) GetTaskLogs(taskID string) []LogEntry {
	e.mu.RLock()
	rt, exists := e.running[taskID]
	e.mu.RUnlock()

	if !exists {
		return nil
	}

	rt.LogMu.RLock()
	defer rt.LogMu.RUnlock()

	logs := make([]LogEntry, len(rt.Logs))
	copy(logs, rt.Logs)
	return logs
}