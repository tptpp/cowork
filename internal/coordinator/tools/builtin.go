package tools

import (
	"github.com/tp/cowork/internal/shared/models"
)

// GetBuiltinTools 获取内置工具定义列表
func GetBuiltinTools() []*models.ToolDefinition {
	return []*models.ToolDefinition{
		{
			Name:        "execute_shell",
			Description: "在隔离环境中执行 Shell 命令。用于文件操作、系统命令等。注意：危险命令会被阻止。",
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
			Description: "读取文件内容。支持文本文件，返回文件内容字符串。",
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
			ExecuteMode: models.ToolExecuteModeLocal,
			Permission:  models.ToolPermissionRead,
			Handler:     "read_file",
			IsEnabled:   true,
			IsBuiltin:   true,
		},
		{
			Name:        "write_file",
			Description: "写入文件内容。如果文件不存在则创建，存在则覆盖。",
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
		{
			Name:        "create_task",
			Description: "创建一个新的任务并分配给 Worker 执行。用于异步执行耗时操作。",
			Parameters: models.JSON{
				"type": "object",
				"properties": models.JSON{
					"type": models.JSON{
						"type":        "string",
						"description": "任务类型：shell, script, docker",
					},
					"description": models.JSON{
						"type":        "string",
						"description": "任务描述",
					},
					"input": models.JSON{
						"type":        "object",
						"description": "任务输入参数",
					},
					"priority": models.JSON{
						"type":        "string",
						"enum":        []string{"low", "medium", "high"},
						"description": "优先级",
					},
					"required_tags": models.JSON{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "所需 Worker 标签",
					},
				},
				"required": []string{"type", "description"},
			},
			Category:    models.ToolCategoryTask,
			ExecuteMode: models.ToolExecuteModeLocal,
			Permission:  models.ToolPermissionExecute,
			Handler:     "create_task",
			IsEnabled:   true,
			IsBuiltin:   true,
		},
		{
			Name:        "query_task",
			Description: "查询任务状态和结果。",
			Parameters: models.JSON{
				"type": "object",
				"properties": models.JSON{
					"task_id": models.JSON{
						"type":        "string",
						"description": "任务 ID",
					},
				},
				"required": []string{"task_id"},
			},
			Category:    models.ToolCategoryTask,
			ExecuteMode: models.ToolExecuteModeLocal,
			Permission:  models.ToolPermissionRead,
			Handler:     "query_task",
			IsEnabled:   true,
			IsBuiltin:   true,
		},
		{
			Name:        "request_approval",
			Description: "请求用户审批。用于需要用户确认的敏感操作。",
			Parameters: models.JSON{
				"type": "object",
				"properties": models.JSON{
					"action": models.JSON{
						"type":        "string",
						"description": "需要审批的操作描述",
					},
					"details": models.JSON{
						"type":        "string",
						"description": "操作详情",
					},
				},
				"required": []string{"action"},
			},
			Category:    models.ToolCategoryTask,
			ExecuteMode: models.ToolExecuteModeLocal,
			Permission:  models.ToolPermissionWrite,
			Handler:     "request_approval",
			IsEnabled:   true,
			IsBuiltin:   true,
		},
	}
}