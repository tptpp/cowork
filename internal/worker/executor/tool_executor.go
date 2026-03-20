package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tp/cowork/internal/shared/models"
)

// ToolExecutor 工具执行器 - 处理工具调用
type ToolExecutor struct {
	config       Config
	shellExec    *Executor
	dockerExec   *DockerExecutor
	registry     ToolRegistry
	validator    ToolValidator
}

// ToolRegistry 工具注册表接口
type ToolRegistry interface {
	Get(toolName string) (*models.ToolDefinition, error)
}

// ToolValidator 工具验证器接口
type ToolValidator interface {
	Validate(toolName string, args map[string]interface{}) error
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor(cfg Config, registry ToolRegistry) *ToolExecutor {
	return &ToolExecutor{
		config:    cfg,
		shellExec: New(cfg),
		registry:  registry,
		validator: NewSchemaValidator(registry),
	}
}

// SetDockerExecutor 设置 Docker 执行器
func (e *ToolExecutor) SetDockerExecutor(dockerExec *DockerExecutor) {
	e.dockerExec = dockerExec
}

// ExecuteTool 执行工具调用
func (e *ToolExecutor) ExecuteTool(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
	workDir string,
	callback Callback,
) *TaskResult {
	result := &TaskResult{
		TaskID:    fmt.Sprintf("tool-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
	}

	// 1. 获取工具定义
	toolDef, err := e.registry.Get(toolName)
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("tool not found: %s", toolName)
		result.EndTime = time.Now()
		return result
	}

	// 2. 验证参数
	if e.validator != nil {
		if err := e.validator.Validate(toolName, args); err != nil {
			result.Status = models.TaskStatusFailed
			result.Error = fmt.Sprintf("validation failed: %v", err)
			result.EndTime = time.Now()
			return result
		}
	}

	// 3. 根据工具名称分发执行
	switch toolName {
	case "execute_shell":
		return e.executeShellTool(ctx, toolDef, args, workDir, callback)
	case "read_file":
		return e.executeReadFile(ctx, toolDef, args, workDir, callback)
	case "write_file":
		return e.executeWriteFile(ctx, toolDef, args, workDir, callback)
	default:
		// 通用工具执行
		return e.executeGenericTool(ctx, toolDef, args, workDir, callback)
	}
}

// executeShellTool 执行 execute_shell 工具
func (e *ToolExecutor) executeShellTool(
	ctx context.Context,
	toolDef *models.ToolDefinition,
	args map[string]interface{},
	workDir string,
	callback Callback,
) *TaskResult {
	result := &TaskResult{
		TaskID:    fmt.Sprintf("shell-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
	}

	// 提取参数
	command, _ := args["command"].(string)
	if command == "" {
		result.Status = models.TaskStatusFailed
		result.Error = "missing required parameter: command"
		result.EndTime = time.Now()
		return result
	}

	// 获取可选参数
	execWorkDir, _ := args["work_dir"].(string)
	timeoutSec, _ := args["timeout"].(float64)
	timeout := 300 * time.Second // 默认 300 秒
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	// 安全检查
	if err := validateCommand(command); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("security check failed: %v", err)
		result.EndTime = time.Now()
		return result
	}

	// 确定工作目录
	execDir := workDir
	if execWorkDir != "" {
		if filepath.IsAbs(execWorkDir) {
			execDir = execWorkDir
		} else {
			execDir = filepath.Join(workDir, execWorkDir)
		}
	}

	// 确保工作目录存在
	if err := os.MkdirAll(execDir, 0755); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to create work directory: %v", err)
		result.EndTime = time.Now()
		return result
	}

	// 创建任务模型用于执行
	task := &models.Task{
		ID:   result.TaskID,
		Type: "shell",
		Input: models.JSON{
			"command": command,
		},
	}

	// 使用 Docker 执行（如果可用且需要隔离）
	if e.dockerExec != nil && e.shouldUseDocker(toolDef, args) {
		callback.OnLog(result.TaskID, "info", "Executing in Docker container")
		dockerResult := e.dockerExec.Execute(task, callback)
		result.Status = dockerResult.Status
		result.Output = dockerResult.Output
		result.Error = dockerResult.Error
		result.Logs = dockerResult.Logs
	} else {
		// 本地 Shell 执行
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		shellResult := e.shellExec.Execute(task, callback)
		result.Status = shellResult.Status
		result.Output = shellResult.Output
		result.Error = shellResult.Error
		result.Logs = shellResult.Logs

		if execCtx.Err() == context.DeadlineExceeded {
			result.Status = models.TaskStatusFailed
			result.Error = "execution timeout"
		}
	}

	result.EndTime = time.Now()
	return result
}

// executeReadFile 执行 read_file 工具
func (e *ToolExecutor) executeReadFile(
	ctx context.Context,
	toolDef *models.ToolDefinition,
	args map[string]interface{},
	workDir string,
	callback Callback,
) *TaskResult {
	result := &TaskResult{
		TaskID:    fmt.Sprintf("read-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
	}

	// 提取参数
	path, _ := args["path"].(string)
	if path == "" {
		result.Status = models.TaskStatusFailed
		result.Error = "missing required parameter: path"
		result.EndTime = time.Now()
		return result
	}

	// 解析路径（支持相对路径）
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(workDir, path)
	}

	// 安全检查：确保路径在工作目录内（防止目录遍历攻击）
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("invalid path: %v", err)
		result.EndTime = time.Now()
		return result
	}

	// 检查路径是否在工作目录内
	if !strings.HasPrefix(absPath, absWorkDir) {
		result.Status = models.TaskStatusFailed
		result.Error = "access denied: path outside work directory"
		result.EndTime = time.Now()
		return result
	}

	callback.OnLog(result.TaskID, "info", fmt.Sprintf("Reading file: %s", absPath))

	// 读取文件
	content, err := os.ReadFile(absPath)
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to read file: %v", err)
		result.EndTime = time.Now()
		return result
	}

	// 获取文件信息
	info, err := os.Stat(absPath)
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to get file info: %v", err)
		result.EndTime = time.Now()
		return result
	}

	result.Status = models.TaskStatusCompleted
	result.Output = models.JSON{
		"content":  string(content),
		"path":     path,
		"size":     len(content),
		"modified": info.ModTime().Format(time.RFC3339),
	}
	result.EndTime = time.Now()
	return result
}

// executeWriteFile 执行 write_file 工具
func (e *ToolExecutor) executeWriteFile(
	ctx context.Context,
	toolDef *models.ToolDefinition,
	args map[string]interface{},
	workDir string,
	callback Callback,
) *TaskResult {
	result := &TaskResult{
		TaskID:    fmt.Sprintf("write-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
	}

	// 提取参数
	path, _ := args["path"].(string)
	if path == "" {
		result.Status = models.TaskStatusFailed
		result.Error = "missing required parameter: path"
		result.EndTime = time.Now()
		return result
	}

	content, _ := args["content"].(string)

	// 解析路径
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(workDir, path)
	}

	// 安全检查：确保路径在工作目录内
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("invalid path: %v", err)
		result.EndTime = time.Now()
		return result
	}

	if !strings.HasPrefix(absPath, absWorkDir) {
		result.Status = models.TaskStatusFailed
		result.Error = "access denied: path outside work directory"
		result.EndTime = time.Now()
		return result
	}

	callback.OnLog(result.TaskID, "info", fmt.Sprintf("Writing file: %s", absPath))

	// 确保目录存在
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to create directory: %v", err)
		result.EndTime = time.Now()
		return result
	}

	// 写入文件
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("failed to write file: %v", err)
		result.EndTime = time.Now()
		return result
	}

	result.Status = models.TaskStatusCompleted
	result.Output = models.JSON{
		"path":     path,
		"size":     len(content),
		"written":  true,
	}
	result.EndTime = time.Now()
	return result
}

// executeGenericTool 执行通用工具
func (e *ToolExecutor) executeGenericTool(
	ctx context.Context,
	toolDef *models.ToolDefinition,
	args map[string]interface{},
	workDir string,
	callback Callback,
) *TaskResult {
	result := &TaskResult{
		TaskID:    fmt.Sprintf("tool-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
	}

	callback.OnLog(result.TaskID, "info", fmt.Sprintf("Executing tool: %s", toolDef.Name))

	// 根据工具处理器类型执行
	switch toolDef.Handler {
	case "execute_shell":
		return e.executeShellTool(ctx, toolDef, args, workDir, callback)
	case "read_file":
		return e.executeReadFile(ctx, toolDef, args, workDir, callback)
	case "write_file":
		return e.executeWriteFile(ctx, toolDef, args, workDir, callback)
	default:
		// 未知处理器，尝试作为 shell 命令执行
		if command, ok := args["command"].(string); ok {
			return e.executeShellTool(ctx, toolDef, map[string]interface{}{
				"command": command,
			}, workDir, callback)
		}

		result.Status = models.TaskStatusFailed
		result.Error = fmt.Sprintf("unknown tool handler: %s", toolDef.Handler)
		result.EndTime = time.Now()
		return result
	}
}

// shouldUseDocker 判断是否应该使用 Docker 执行
func (e *ToolExecutor) shouldUseDocker(toolDef *models.ToolDefinition, args map[string]interface{}) bool {
	// 检查参数中的显式指定
	if useDocker, ok := args["use_docker"].(bool); ok {
		return useDocker
	}

	// 检查工具定义中的权限
	if toolDef.Permission == models.ToolPermissionExecute {
		// 执行权限的工具默认使用 Docker
		return e.dockerExec != nil
	}

	return false
}

// ExecuteToolFromTask 从任务模型执行工具
func (e *ToolExecutor) ExecuteToolFromTask(task *models.Task, callback Callback) *TaskResult {
	// 从任务中提取工具信息
	toolName := task.ToolName
	if toolName == "" {
		toolName = task.Type // 兼容旧格式
	}

	// 获取工具参数
	args := task.Input
	if args == nil {
		args = make(map[string]interface{})
	}

	// 确定工作目录
	workDir := task.WorkDir
	if workDir == "" {
		workDir = filepath.Join(e.config.BaseWorkDir, task.ID)
		os.MkdirAll(workDir, 0755)
	}

	return e.ExecuteTool(context.Background(), toolName, args, workDir, callback)
}

// SchemaValidator 基于 JSON Schema 的工具验证器
type SchemaValidator struct {
	registry ToolRegistry
}

// NewSchemaValidator 创建 Schema 验证器
func NewSchemaValidator(registry ToolRegistry) *SchemaValidator {
	return &SchemaValidator{registry: registry}
}

// Validate 验证工具参数
func (v *SchemaValidator) Validate(toolName string, args map[string]interface{}) error {
	toolDef, err := v.registry.Get(toolName)
	if err != nil {
		return fmt.Errorf("tool not found: %s", toolName)
	}

	// 获取参数定义
	params, ok := toolDef.Parameters["properties"].(map[string]interface{})
	if !ok {
		return nil // 没有参数定义，跳过验证
	}

	// 获取必需参数
	required := make(map[string]bool)

	// 尝试多种类型断言
	switch req := toolDef.Parameters["required"].(type) {
	case []interface{}:
		for _, r := range req {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	case []string:
		for _, s := range req {
			required[s] = true
		}
	case string:
		required[req] = true
	}

	// 检查必需参数
	for name := range required {
		if _, exists := args[name]; !exists {
			return fmt.Errorf("missing required parameter: %s", name)
		}
	}

	// 验证参数类型
	for name, value := range args {
		paramDef, exists := params[name].(map[string]interface{})
		if !exists {
			continue // 未知参数，允许通过
		}

		if err := v.validateParamType(name, value, paramDef); err != nil {
			return err
		}
	}

	return nil
}

// validateParamType 验证参数类型
func (v *SchemaValidator) validateParamType(name string, value interface{}, paramDef map[string]interface{}) error {
	paramType, _ := paramDef["type"].(string)

	switch paramType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter %s must be a string", name)
		}
	case "integer", "number":
		switch value.(type) {
		case int, int64, float64:
			// OK
		default:
			return fmt.Errorf("parameter %s must be a number", name)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter %s must be a boolean", name)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("parameter %s must be an array", name)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("parameter %s must be an object", name)
		}
	}

	return nil
}

// ToolResultJSON 工具执行结果的 JSON 表示
type ToolResultJSON struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Status     string `json:"status"`
	Output     string `json:"output"`
	IsError    bool   `json:"is_error"`
	Duration   int64  `json:"duration_ms"`
}

// ToJSON 将工具结果转换为 JSON 格式
func (r *TaskResult) ToToolResultJSON(toolCallID, toolName string) *ToolResultJSON {
	output, _ := json.Marshal(r.Output)
	return &ToolResultJSON{
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Status:     string(r.Status),
		Output:     string(output),
		IsError:    r.Status == models.TaskStatusFailed,
		Duration:   r.EndTime.Sub(r.StartTime).Milliseconds(),
	}
}