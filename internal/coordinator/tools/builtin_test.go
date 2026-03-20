package tools

import (
	"testing"

	"github.com/tp/cowork/internal/shared/models"
)

func TestGetBuiltinTools(t *testing.T) {
	tools := GetBuiltinTools()

	if len(tools) == 0 {
		t.Error("Expected at least one builtin tool")
	}

	// 检查必要的工具是否存在
	expectedTools := []string{"execute_shell", "read_file", "write_file", "create_task", "query_task", "request_approval"}
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected builtin tool '%s' not found", expected)
		}
	}
}

func TestBuiltinToolDefinitions(t *testing.T) {
	tools := GetBuiltinTools()

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			// 检查基本字段
			if tool.Name == "" {
				t.Error("Tool name should not be empty")
			}
			if tool.Description == "" {
				t.Error("Tool description should not be empty")
			}
			if tool.Parameters == nil {
				t.Error("Tool parameters should not be nil")
			}

			// 检查内置工具标记
			if !tool.IsBuiltin {
				t.Error("Builtin tool should have IsBuiltin=true")
			}

			// 检查默认启用
			if !tool.IsEnabled {
				t.Error("Builtin tool should be enabled by default")
			}
		})
	}
}

func TestExecuteShellTool(t *testing.T) {
	tools := GetBuiltinTools()
	var executeShell *models.ToolDefinition

	for _, tool := range tools {
		if tool.Name == "execute_shell" {
			executeShell = tool
			break
		}
	}

	if executeShell == nil {
		t.Fatal("execute_shell tool not found")
	}

	// 检查参数定义
	params := executeShell.Parameters["properties"]
	if params == nil {
		t.Fatal("execute_shell parameters properties not found")
	}

	// 检查必需参数
	required, ok := executeShell.Parameters["required"].([]string)
	if !ok {
		t.Fatal("execute_shell required parameters not found")
	}

	if len(required) != 1 || required[0] != "command" {
		t.Error("execute_shell should have 'command' as required parameter")
	}

	// 检查执行模式
	if executeShell.ExecuteMode != models.ToolExecuteModeRemote {
		t.Error("execute_shell should be remote execution mode")
	}

	// 检查权限
	if executeShell.Permission != models.ToolPermissionExecute {
		t.Error("execute_shell should have execute permission")
	}
}

func TestReadFileTool(t *testing.T) {
	tools := GetBuiltinTools()
	var readFile *models.ToolDefinition

	for _, tool := range tools {
		if tool.Name == "read_file" {
			readFile = tool
			break
		}
	}

	if readFile == nil {
		t.Fatal("read_file tool not found")
	}

	// 检查执行模式
	if readFile.ExecuteMode != models.ToolExecuteModeRemote {
		t.Error("read_file should be remote execution mode")
	}

	// 检查权限
	if readFile.Permission != models.ToolPermissionRead {
		t.Error("read_file should have read permission")
	}

	// 检查类别
	if readFile.Category != models.ToolCategoryFile {
		t.Error("read_file should be in file category")
	}
}

func TestWriteFileTool(t *testing.T) {
	tools := GetBuiltinTools()
	var writeFile *models.ToolDefinition

	for _, tool := range tools {
		if tool.Name == "write_file" {
			writeFile = tool
			break
		}
	}

	if writeFile == nil {
		t.Fatal("write_file tool not found")
	}

	// 检查执行模式
	if writeFile.ExecuteMode != models.ToolExecuteModeRemote {
		t.Error("write_file should be remote execution mode")
	}

	// 检查权限
	if writeFile.Permission != models.ToolPermissionWrite {
		t.Error("write_file should have write permission")
	}

	// 检查必需参数
	required, ok := writeFile.Parameters["required"].([]string)
	if !ok {
		t.Fatal("write_file required parameters not found")
	}

	if len(required) != 2 {
		t.Error("write_file should have 2 required parameters")
	}
}

func TestCreateTaskTool(t *testing.T) {
	tools := GetBuiltinTools()
	var createTask *models.ToolDefinition

	for _, tool := range tools {
		if tool.Name == "create_task" {
			createTask = tool
			break
		}
	}

	if createTask == nil {
		t.Fatal("create_task tool not found")
	}

	// 检查执行模式 - 应该是本地执行
	if createTask.ExecuteMode != models.ToolExecuteModeLocal {
		t.Error("create_task should be local execution mode")
	}

	// 检查类别
	if createTask.Category != models.ToolCategoryTask {
		t.Error("create_task should be in task category")
	}
}

func TestToOpenAITool(t *testing.T) {
	tool := &models.ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: models.JSON{
			"type": "object",
			"properties": models.JSON{
				"param1": models.JSON{
					"type": "string",
				},
			},
		},
	}

	openaiTool := tool.ToOpenAITool()

	// 检查类型
	toolType, ok := openaiTool["type"].(string)
	if !ok || toolType != "function" {
		t.Error("OpenAI tool type should be 'function'")
	}

	// 检查 function 字段
	function, ok := openaiTool["function"].(map[string]interface{})
	if !ok {
		t.Fatal("OpenAI tool function field not found")
	}

	if function["name"] != "test_tool" {
		t.Error("OpenAI tool name mismatch")
	}

	if function["description"] != "A test tool" {
		t.Error("OpenAI tool description mismatch")
	}

	if function["parameters"] == nil {
		t.Error("OpenAI tool parameters should not be nil")
	}
}