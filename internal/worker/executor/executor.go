package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tp/cowork/internal/shared/models"
	"github.com/tp/cowork/internal/shared/utils"
	"github.com/tp/cowork/internal/worker/ai"
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
	AllowedPaths   []string      // 允许访问的路径列表（除了工作目录外）
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		// BaseWorkDir 由调用方设置，基于 worker 名称动态生成
		// 例如: ~/.cowork/workers/{worker-name}/workspace
		BaseWorkDir:    "",
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

	// AI 支持
	aiClient      *ai.AIClient
	systemPrompt  string
	enabledTools  []string
	allowedPaths  []string
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

	rt.Cmd = cmd

	// 使用 CombinedOutput 捕获所有输出
	output, err := cmd.CombinedOutput()

	// 记录输出到日志
	if len(output) > 0 {
		e.addLog(rt, "info", string(output))
		callback.OnLog(task.ID, "info", string(output))
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Status = models.TaskStatusFailed
		result.Error = "execution timeout"
	} else if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = err.Error()
		result.Output = models.JSON{
			"exit_code": 1,
			"output":    string(output),
			"error":     err.Error(),
		}
	} else {
		result.Status = models.TaskStatusCompleted
		result.Output = models.JSON{
			"exit_code": 0,
			"output":    string(output),
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

	e.addLog(rt, "info", "Starting agent task...")

	// 检查 AI 客户端是否可用
	if e.aiClient == nil {
		e.addLog(rt, "error", "AI client not configured")
		result.Status = models.TaskStatusFailed
		result.Error = "AI client not configured. Please provide --ai-base-url, --ai-model, and --ai-api-key"
		return result
	}

	// 解析任务输入
	agentInput, err := parseAgentInput(task.Input)
	if err != nil {
		e.addLog(rt, "error", fmt.Sprintf("Failed to parse input: %v", err))
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to parse input: %v", err)
		return result
	}

	e.addLog(rt, "info", fmt.Sprintf("Task prompt: %s", truncateString(agentInput.Prompt, 200)))

	// 构建消息
	messages := e.buildMessages(rt, agentInput)

	// 获取可用工具
	tools := e.getTools()

	// 更新进度
	callback.OnProgress(task.ID, 10)

	// 创建工具执行器
	toolExecutor := &agentToolExecutor{
		executor:     e,
		rt:           rt,
		task:         task,
		callback:     callback,
		allowedPaths: e.allowedPaths,
	}

	// 调用 AI（支持工具调用循环）
	e.addLog(rt, "info", fmt.Sprintf("Calling AI model: %s", e.aiClient.GetModel()))

	finalContent, toolCalls, err := e.aiClient.ChatWithTools(
		e.systemPrompt,
		messages,
		tools,
		4096,
		0.7,
		toolExecutor,
		10, // 最大工具调用轮数
	)

	if err != nil {
		e.addLog(rt, "error", fmt.Sprintf("AI call failed: %v", err))
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("AI call failed: %v", err)
		return result
	}

	// 更新进度
	callback.OnProgress(task.ID, 90)

	// 构建输出
	result.Status = models.TaskStatusCompleted
	result.Output = models.JSON{
		"content":    finalContent,
		"tool_calls": len(toolCalls),
		"message":    "Agent task completed successfully",
	}

	e.addLog(rt, "info", fmt.Sprintf("Agent task completed with %d tool calls", len(toolCalls)))
	callback.OnProgress(task.ID, 100)

	return result
}

// AgentInput Agent 任务输入
type AgentInput struct {
	Prompt  string   `json:"prompt"`
	Files   []string `json:"files,omitempty"`
	Context string   `json:"context,omitempty"`
}

// parseAgentInput 解析任务输入
func parseAgentInput(input models.JSON) (*AgentInput, error) {
	result := &AgentInput{}

	// 尝试解析为结构化输入
	if prompt, ok := input["prompt"].(string); ok {
		result.Prompt = prompt
	}

	if files, ok := input["files"].([]interface{}); ok {
		for _, f := range files {
			if filePath, ok := f.(string); ok {
				result.Files = append(result.Files, filePath)
			}
		}
	}

	if context, ok := input["context"].(string); ok {
		result.Context = context
	}

	// 如果没有结构化输入，尝试从其他字段获取
	if result.Prompt == "" {
		if description, ok := input["description"].(string); ok {
			result.Prompt = description
		} else if command, ok := input["command"].(string); ok {
			result.Prompt = command
		}
	}

	if result.Prompt == "" {
		return nil, fmt.Errorf("no prompt found in task input")
	}

	return result, nil
}

// buildMessages 构建消息列表
func (e *Executor) buildMessages(rt *RunningTask, input *AgentInput) []ai.Message {
	messages := []ai.Message{}

	// 添加用户消息
	content := input.Prompt

	// 添加文件上下文
	if len(input.Files) > 0 {
		fileContext := e.buildFileContext(rt, input.Files)
		if fileContext != "" {
			content = fmt.Sprintf("%s\n\n---\nFile Context:\n%s", content, fileContext)
		}
	}

	// 添加额外上下文
	if input.Context != "" {
		content = fmt.Sprintf("%s\n\n---\nAdditional Context:\n%s", content, input.Context)
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: content,
	})

	return messages
}

// buildFileContext 构建文件上下文
func (e *Executor) buildFileContext(rt *RunningTask, files []string) string {
	var builder strings.Builder

	for _, filePath := range files {
		// 检查路径是否在允许范围内
		if !e.isPathAllowed(filePath) {
			e.addLog(rt, "warn", fmt.Sprintf("File path not allowed: %s", filePath))
			continue
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			e.addLog(rt, "warn", fmt.Sprintf("Failed to read file %s: %v", filePath, err))
			continue
		}

		builder.WriteString(fmt.Sprintf("\n### File: %s\n```\n%s\n```\n", filePath, string(data)))
	}

	return builder.String()
}

// isPathAllowed 检查路径是否被允许访问
func (e *Executor) isPathAllowed(path string) bool {
	// 如果没有配置允许路径，默认允许所有
	if len(e.allowedPaths) == 0 {
		return true
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, allowed := range e.allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absAllowed) {
			return true
		}
	}

	return false
}

// getTools 获取可用工具列表
func (e *Executor) getTools() []ai.Tool {
	tools := []ai.Tool{}

	for _, toolName := range e.enabledTools {
		switch toolName {
		case "shell":
			tools = append(tools, ai.Tool{
				Type: "function",
				Function: ai.ToolDefFunc{
					Name:        "execute_shell",
					Description: "Execute a shell command in the work directory",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"command": map[string]interface{}{
								"type":        "string",
								"description": "The shell command to execute",
							},
							"timeout": map[string]interface{}{
								"type":        "integer",
								"description": "Timeout in seconds (default: 300)",
							},
						},
						"required": []string{"command"},
					},
				},
			})
		case "file":
			tools = append(tools, ai.Tool{
				Type: "function",
				Function: ai.ToolDefFunc{
					Name:        "read_file",
					Description: "Read the contents of a file",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "The path to the file to read",
							},
						},
						"required": []string{"path"},
					},
				},
			})
			tools = append(tools, ai.Tool{
				Type: "function",
				Function: ai.ToolDefFunc{
					Name:        "write_file",
					Description: "Write content to a file",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "The path to the file to write",
							},
							"content": map[string]interface{}{
								"type":        "string",
								"description": "The content to write to the file",
							},
						},
						"required": []string{"path", "content"},
					},
				},
			})
		case "web":
			tools = append(tools, ai.Tool{
				Type: "function",
				Function: ai.ToolDefFunc{
					Name:        "http_request",
					Description: "Make an HTTP request",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"method": map[string]interface{}{
								"type":        "string",
								"description": "HTTP method (GET, POST, etc.)",
								"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
							},
							"url": map[string]interface{}{
								"type":        "string",
								"description": "The URL to request",
							},
							"headers": map[string]interface{}{
								"type":        "object",
								"description": "HTTP headers to send",
							},
							"body": map[string]interface{}{
								"type":        "string",
								"description": "Request body (for POST/PUT)",
							},
						},
						"required": []string{"method", "url"},
					},
				},
			})
		}
	}

	return tools
}

// agentToolExecutor Agent 工具执行器
type agentToolExecutor struct {
	executor     *Executor
	rt           *RunningTask
	task         *models.Task
	callback     Callback
	allowedPaths []string
}

// ExecuteToolCall 执行工具调用
func (e *agentToolExecutor) ExecuteToolCall(name string, arguments string) (string, error) {
	e.executor.addLog(e.rt, "info", fmt.Sprintf("Tool call: %s", name))

	// 解析参数
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	switch name {
	case "execute_shell":
		return e.executeShell(args)
	case "read_file":
		return e.readFile(args)
	case "write_file":
		return e.writeFile(args)
	case "http_request":
		return e.httpRequest(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// executeShell 执行 shell 命令
func (e *agentToolExecutor) executeShell(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'command' argument")
	}

	// 安全检查
	if err := validateCommand(command); err != nil {
		return "", err
	}

	// 获取超时时间
	timeout := 300
	if t, ok := args["timeout"].(float64); ok {
		timeout = int(t)
	}

	// 创建命令上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = e.rt.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	e.executor.addLog(e.rt, "info", fmt.Sprintf("Shell output: %s", truncateString(string(output), 500)))
	return string(output), nil
}

// readFile 读取文件
func (e *agentToolExecutor) readFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}

	// 检查路径权限
	if !e.executor.isPathAllowed(path) {
		return "", fmt.Errorf("path not allowed: %s", path)
	}

	// 如果是相对路径，相对于工作目录
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.rt.WorkDir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	e.executor.addLog(e.rt, "info", fmt.Sprintf("Read file: %s (%d bytes)", path, len(data)))
	return string(data), nil
}

// writeFile 写入文件
func (e *agentToolExecutor) writeFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'content' argument")
	}

	// 检查路径权限
	if !e.executor.isPathAllowed(path) {
		return "", fmt.Errorf("path not allowed: %s", path)
	}

	// 如果是相对路径，相对于工作目录
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.rt.WorkDir, path)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	e.executor.addLog(e.rt, "info", fmt.Sprintf("Wrote file: %s (%d bytes)", path, len(content)))
	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

// httpRequest 执行 HTTP 请求
func (e *agentToolExecutor) httpRequest(args map[string]interface{}) (string, error) {
	method, ok := args["method"].(string)
	if !ok {
		method = "GET"
	}

	url, ok := args["url"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'url' argument")
	}

	e.executor.addLog(e.rt, "info", fmt.Sprintf("HTTP request: %s %s", method, url))

	// HTTP 工具尚未实现，返回明确的错误
	// 如需使用，请启用 "web" 工具并提供相应的 HTTP 客户端实现
	return "", fmt.Errorf("HTTP tool not implemented: %s %s (enable 'web' tool for HTTP support)", method, url)
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	return utils.TruncateString(s, maxLen)
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

// SetAIClient 设置 AI 客户端
func (e *Executor) SetAIClient(client *ai.AIClient) {
	e.aiClient = client
}

// SetSystemPrompt 设置系统提示词
func (e *Executor) SetSystemPrompt(prompt string) {
	e.systemPrompt = prompt
}

// SetEnabledTools 设置启用的工具
func (e *Executor) SetEnabledTools(tools []string) {
	e.enabledTools = tools
}

// SetAllowedPaths 设置允许的文件路径
func (e *Executor) SetAllowedPaths(paths []string) {
	e.allowedPaths = paths
}
