package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tp/cowork/internal/shared/models"
)

// MockToolRegistry 模拟工具注册表
type MockToolRegistry struct {
	tools map[string]*models.ToolDefinition
}

func NewMockToolRegistry() *MockToolRegistry {
	return &MockToolRegistry{
		tools: map[string]*models.ToolDefinition{
			"execute_shell": {
				Name:        "execute_shell",
				Description: "Execute shell command",
				Parameters: models.JSON{
					"type": "object",
					"properties": models.JSON{
						"command": models.JSON{
							"type":        "string",
							"description": "Command to execute",
						},
					},
					"required": []interface{}{"command"},
				},
				Category:    models.ToolCategorySystem,
				ExecuteMode: models.ToolExecuteModeRemote,
				Permission:  models.ToolPermissionExecute,
				Handler:     "execute_shell",
				IsEnabled:   true,
				IsBuiltin:   true,
			},
			"read_file": {
				Name:        "read_file",
				Description: "Read file content",
				Parameters: models.JSON{
					"type": "object",
					"properties": models.JSON{
						"path": models.JSON{
							"type":        "string",
							"description": "File path",
						},
					},
					"required": []interface{}{"path"},
				},
				Category:    models.ToolCategoryFile,
				ExecuteMode: models.ToolExecuteModeRemote,
				Permission:  models.ToolPermissionRead,
				Handler:     "read_file",
				IsEnabled:   true,
				IsBuiltin:   true,
			},
			"write_file": {
				Name:        "write_file",
				Description: "Write file content",
				Parameters: models.JSON{
					"type": "object",
					"properties": models.JSON{
						"path": models.JSON{
							"type":        "string",
							"description": "File path",
						},
						"content": models.JSON{
							"type":        "string",
							"description": "File content",
						},
					},
					"required": []interface{}{"path", "content"},
				},
				Category:    models.ToolCategoryFile,
				ExecuteMode: models.ToolExecuteModeRemote,
				Permission:  models.ToolPermissionWrite,
				Handler:     "write_file",
				IsEnabled:   true,
				IsBuiltin:   true,
			},
		},
	}
}

func (r *MockToolRegistry) Get(toolName string) (*models.ToolDefinition, error) {
	if tool, ok := r.tools[toolName]; ok {
		return tool, nil
	}
	return nil, &ToolNotFoundError{Name: toolName}
}

// ToolNotFoundError 工具未找到错误
type ToolNotFoundError struct {
	Name string
}

func (e *ToolNotFoundError) Error() string {
	return "tool not found: " + e.Name
}

// MockCallback 模拟回调
type MockCallback struct {
	progress []int
	logs     []struct {
		level   string
		message string
	}
	completed bool
}

func NewMockCallback() *MockCallback {
	return &MockCallback{
		progress: make([]int, 0),
		logs:     make([]struct{ level, message string }, 0),
	}
}

func (c *MockCallback) OnProgress(taskID string, progress int) {
	c.progress = append(c.progress, progress)
}

func (c *MockCallback) OnLog(taskID string, level, message string) {
	c.logs = append(c.logs, struct{ level, message string }{level, message})
}

func (c *MockCallback) OnComplete(taskID string, result *TaskResult) {
	c.completed = true
}

// TestToolExecutor_ExecuteTool_ExecuteShell 测试 execute_shell 工具
func TestToolExecutor_ExecuteTool_ExecuteShell(t *testing.T) {
	// 创建临时工作目录
	tmpDir, err := os.MkdirTemp("", "cowork-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建执行器
	config := Config{
		BaseWorkDir:   tmpDir,
		MaxExecTime:   30 * time.Second,
		CleanupOnExit: false,
	}
	registry := NewMockToolRegistry()
	executor := NewToolExecutor(config, registry)

	// 创建回调
	callback := NewMockCallback()

	// 执行简单命令
	result := executor.ExecuteTool(
		context.Background(),
		"execute_shell",
		map[string]interface{}{
			"command": "echo 'Hello, World!'",
		},
		tmpDir,
		callback,
	)

	// 验证结果
	if result.Status != models.TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s, error: %s", result.Status, result.Error)
	}

	if result.Output == nil {
		t.Error("Expected output to be non-nil")
	}
}

// TestToolExecutor_ExecuteTool_ReadFile 测试 read_file 工具
func TestToolExecutor_ExecuteTool_ReadFile(t *testing.T) {
	// 创建临时工作目录
	tmpDir, err := os.MkdirTemp("", "cowork-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testContent := "Hello, this is a test file!"
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 创建执行器
	config := Config{
		BaseWorkDir:   tmpDir,
		MaxExecTime:   30 * time.Second,
		CleanupOnExit: false,
	}
	registry := NewMockToolRegistry()
	executor := NewToolExecutor(config, registry)

	// 创建回调
	callback := NewMockCallback()

	// 读取文件
	result := executor.ExecuteTool(
		context.Background(),
		"read_file",
		map[string]interface{}{
			"path": "test.txt",
		},
		tmpDir,
		callback,
	)

	// 验证结果
	if result.Status != models.TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s, error: %s", result.Status, result.Error)
	}

	if result.Output == nil {
		t.Fatal("Expected output to be non-nil")
	}

	content, ok := result.Output["content"].(string)
	if !ok {
		t.Fatal("Expected output to have 'content' field of type string")
	}

	if content != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, content)
	}
}

// TestToolExecutor_ExecuteTool_WriteFile 测试 write_file 工具
func TestToolExecutor_ExecuteTool_WriteFile(t *testing.T) {
	// 创建临时工作目录
	tmpDir, err := os.MkdirTemp("", "cowork-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建执行器
	config := Config{
		BaseWorkDir:   tmpDir,
		MaxExecTime:   30 * time.Second,
		CleanupOnExit: false,
	}
	registry := NewMockToolRegistry()
	executor := NewToolExecutor(config, registry)

	// 创建回调
	callback := NewMockCallback()

	// 写入文件
	testContent := "Hello, this is written content!"
	result := executor.ExecuteTool(
		context.Background(),
		"write_file",
		map[string]interface{}{
			"path":    "output.txt",
			"content": testContent,
		},
		tmpDir,
		callback,
	)

	// 验证结果
	if result.Status != models.TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s, error: %s", result.Status, result.Error)
	}

	// 验证文件是否创建
	outputFile := filepath.Join(tmpDir, "output.txt")
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected file content '%s', got '%s'", testContent, string(content))
	}
}

// TestToolExecutor_ExecuteTool_UnknownTool 测试未知工具
func TestToolExecutor_ExecuteTool_UnknownTool(t *testing.T) {
	// 创建临时工作目录
	tmpDir, err := os.MkdirTemp("", "cowork-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建执行器
	config := Config{
		BaseWorkDir:   tmpDir,
		MaxExecTime:   30 * time.Second,
		CleanupOnExit: false,
	}
	registry := NewMockToolRegistry()
	executor := NewToolExecutor(config, registry)

	// 创建回调
	callback := NewMockCallback()

	// 执行未知工具
	result := executor.ExecuteTool(
		context.Background(),
		"unknown_tool",
		map[string]interface{}{},
		tmpDir,
		callback,
	)

	// 验证结果
	if result.Status != models.TaskStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}

	if result.Error == "" {
		t.Error("Expected error message for unknown tool")
	}
}

// TestSchemaValidator_Validate 测试参数验证
func TestSchemaValidator_Validate(t *testing.T) {
	registry := NewMockToolRegistry()
	validator := NewSchemaValidator(registry)

	tests := []struct {
		name      string
		toolName  string
		args      map[string]interface{}
		wantError bool
	}{
		{
			name:     "Valid execute_shell args",
			toolName: "execute_shell",
			args: map[string]interface{}{
				"command": "ls -la",
			},
			wantError: false,
		},
		{
			name:     "Missing required command",
			toolName: "execute_shell",
			args: map[string]interface{}{
				"work_dir": "/tmp",
			},
			wantError: true,
		},
		{
			name:     "Valid read_file args",
			toolName: "read_file",
			args: map[string]interface{}{
				"path": "/tmp/test.txt",
			},
			wantError: false,
		},
		{
			name:     "Missing required path",
			toolName: "read_file",
			args:     map[string]interface{}{},
			wantError: true,
		},
		{
			name:     "Valid write_file args",
			toolName: "write_file",
			args: map[string]interface{}{
				"path":    "/tmp/test.txt",
				"content": "Hello",
			},
			wantError: false,
		},
		{
			name:     "Missing required content",
			toolName: "write_file",
			args: map[string]interface{}{
				"path": "/tmp/test.txt",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.toolName, tt.args)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestToolExecutor_SecurityCheck 测试安全检查
func TestToolExecutor_SecurityCheck(t *testing.T) {
	// 创建临时工作目录
	tmpDir, err := os.MkdirTemp("", "cowork-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建执行器
	config := Config{
		BaseWorkDir:   tmpDir,
		MaxExecTime:   30 * time.Second,
		CleanupOnExit: false,
	}
	registry := NewMockToolRegistry()
	executor := NewToolExecutor(config, registry)

	// 创建回调
	callback := NewMockCallback()

	// 测试危险命令
	dangerousCommands := []string{
		"rm -rf /",
		"mkfs.ext4 /dev/sda",
		"dd if=/dev/zero of=/dev/sda",
	}

	for _, cmd := range dangerousCommands {
		result := executor.ExecuteTool(
			context.Background(),
			"execute_shell",
			map[string]interface{}{
				"command": cmd,
			},
			tmpDir,
			callback,
		)

		if result.Status != models.TaskStatusFailed {
			t.Errorf("Expected dangerous command '%s' to be blocked", cmd)
		}

		if result.Error == "" || !containsSecurityError(result.Error) {
			t.Errorf("Expected security error for command '%s', got: %s", cmd, result.Error)
		}
	}
}

// TestToolExecutor_PathTraversal 测试路径遍历攻击防护
func TestToolExecutor_PathTraversal(t *testing.T) {
	// 创建临时工作目录
	tmpDir, err := os.MkdirTemp("", "cowork-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建执行器
	config := Config{
		BaseWorkDir:   tmpDir,
		MaxExecTime:   30 * time.Second,
		CleanupOnExit: false,
	}
	registry := NewMockToolRegistry()
	executor := NewToolExecutor(config, registry)

	// 创建回调
	callback := NewMockCallback()

	// 尝试路径遍历攻击
	result := executor.ExecuteTool(
		context.Background(),
		"read_file",
		map[string]interface{}{
			"path": "../../../etc/passwd",
		},
		tmpDir,
		callback,
	)

	// 验证结果
	if result.Status != models.TaskStatusFailed {
		t.Error("Expected path traversal attack to be blocked")
	}

	if result.Error == "" || !containsAccessDenied(result.Error) {
		t.Errorf("Expected access denied error, got: %s", result.Error)
	}
}

// TestToolExecutor_ExecuteToolFromTask 测试从任务模型执行工具
func TestToolExecutor_ExecuteToolFromTask(t *testing.T) {
	// 创建临时工作目录
	tmpDir, err := os.MkdirTemp("", "cowork-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建执行器
	config := Config{
		BaseWorkDir:   tmpDir,
		MaxExecTime:   30 * time.Second,
		CleanupOnExit: false,
	}
	registry := NewMockToolRegistry()
	executor := NewToolExecutor(config, registry)

	// 创建回调
	callback := NewMockCallback()

	// 创建任务模型
	task := &models.Task{
		ID:       "test-task-001",
		Type:     "tool",
		ToolName: "execute_shell",
		Input: models.JSON{
			"command": "echo 'test'",
		},
		WorkDir: tmpDir,
	}

	// 执行任务
	result := executor.ExecuteToolFromTask(task, callback)

	// 验证结果
	if result.Status != models.TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s, error: %s", result.Status, result.Error)
	}
}

// 辅助函数
func containsSecurityError(err string) bool {
	return contains(err, "security") || contains(err, "blocked") || contains(err, "dangerous")
}

func containsAccessDenied(err string) bool {
	return contains(err, "access denied") || contains(err, "outside work directory")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}