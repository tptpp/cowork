package agent

import (
	"encoding/json"
	"testing"

	"github.com/tp/cowork/internal/shared/models"
)

// TestParseDecompositionResponse 测试解析拆解响应
func TestParseDecompositionResponse(t *testing.T) {
	decomposer := &TaskDecomposer{
		config: DecomposerConfig{
			MaxSubTasks: 10,
		},
	}

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		numTasks int
	}{
		{
			name: "simple json",
			input: `{
				"success": true,
				"reasoning": "test",
				"tasks": [
					{
						"id": "task-1",
						"title": "Test Task",
						"description": "A test task",
						"type": "code",
						"priority": "high"
					}
				]
			}`,
			wantErr:  false,
			numTasks: 1,
		},
		{
			name: "json in markdown",
			input: "Here's the decomposition:\n```json\n{\n\t\"success\": true,\n\t\"reasoning\": \"test\",\n\t\"tasks\": [\n\t\t{\n\t\t\t\"id\": \"task-1\",\n\t\t\t\"title\": \"Task 1\",\n\t\t\t\"description\": \"First task\",\n\t\t\t\"type\": \"code\"\n\t\t},\n\t\t{\n\t\t\t\"id\": \"task-2\",\n\t\t\t\"title\": \"Task 2\",\n\t\t\t\"description\": \"Second task\",\n\t\t\t\"type\": \"file\",\n\t\t\t\"depends_on\": [\"task-1\"]\n\t\t}\n\t]\n}\n```\nDone!",
			wantErr:  false,
			numTasks: 2,
		},
		{
			name: "invalid json",
			input: `{ not valid json }`,
			wantErr: true,
		},
		{
			name: "empty tasks",
			input: `{
				"success": true,
				"reasoning": "test",
				"tasks": []
			}`,
			wantErr:  false,
			numTasks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decomposer.parseDecompositionResponse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDecompositionResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != nil {
				if len(result.Tasks) != tt.numTasks {
					t.Errorf("expected %d tasks, got %d", tt.numTasks, len(result.Tasks))
				}
			}
		})
	}
}

// TestValidateDecomposition 测试验证拆解结果
func TestValidateDecomposition(t *testing.T) {
	decomposer := &TaskDecomposer{
		config: DecomposerConfig{
			MaxSubTasks: 10,
			MinSubTasks: 1,
		},
	}

	tests := []struct {
		name    string
		input   *models.DecompositionResult
		wantErr bool
	}{
		{
			name: "valid single task",
			input: &models.DecompositionResult{
				Success:   true,
				Reasoning: "test",
				Tasks: []models.DecomposedTask{
					{ID: "t1", Title: "Task 1", Type: "code"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tasks with dependency",
			input: &models.DecompositionResult{
				Success:   true,
				Reasoning: "test",
				Tasks: []models.DecomposedTask{
					{ID: "t1", Title: "Task 1", Type: "code"},
					{ID: "t2", Title: "Task 2", Type: "code", DependsOn: []string{"t1"}},
				},
			},
			wantErr: false,
		},
		{
			name: "no tasks",
			input: &models.DecompositionResult{
				Success:   true,
				Reasoning: "test",
				Tasks:     []models.DecomposedTask{},
			},
			wantErr: true,
		},
		{
			name: "duplicate task id",
			input: &models.DecompositionResult{
				Success:   true,
				Reasoning: "test",
				Tasks: []models.DecomposedTask{
					{ID: "t1", Title: "Task 1", Type: "code"},
					{ID: "t1", Title: "Task 2", Type: "code"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid dependency",
			input: &models.DecompositionResult{
				Success:   true,
				Reasoning: "test",
				Tasks: []models.DecomposedTask{
					{ID: "t1", Title: "Task 1", Type: "code", DependsOn: []string{"nonexistent"}},
				},
			},
			wantErr: true,
		},
		{
			name: "too many tasks",
			input: &models.DecompositionResult{
				Success:   true,
				Reasoning: "test",
				Tasks:     make([]models.DecomposedTask, 15), // exceeds MaxSubTasks
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 填充 tasks 如果是 "too many tasks" 测试
			if tt.name == "too many tasks" {
				for i := range tt.input.Tasks {
					tt.input.Tasks[i] = models.DecomposedTask{
						ID:    string(rune('a' + i)),
						Title: "Task",
						Type:  "code",
					}
				}
			}

			err := decomposer.validateDecomposition(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDecomposition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDetectCycle 测试循环依赖检测
func TestDetectCycle(t *testing.T) {
	decomposer := &TaskDecomposer{}

	tests := []struct {
		name    string
		tasks   []models.DecomposedTask
		wantErr bool
	}{
		{
			name: "no cycle",
			tasks: []models.DecomposedTask{
				{ID: "a", DependsOn: []string{}},
				{ID: "b", DependsOn: []string{"a"}},
				{ID: "c", DependsOn: []string{"b"}},
			},
			wantErr: false,
		},
		{
			name: "simple cycle",
			tasks: []models.DecomposedTask{
				{ID: "a", DependsOn: []string{"b"}},
				{ID: "b", DependsOn: []string{"a"}},
			},
			wantErr: true,
		},
		{
			name: "indirect cycle",
			tasks: []models.DecomposedTask{
				{ID: "a", DependsOn: []string{"c"}},
				{ID: "b", DependsOn: []string{"a"}},
				{ID: "c", DependsOn: []string{"b"}},
			},
			wantErr: true,
		},
		{
			name: "self dependency",
			tasks: []models.DecomposedTask{
				{ID: "a", DependsOn: []string{"a"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decomposer.detectCycle(tt.tasks)
			if (err != nil) != tt.wantErr {
				t.Errorf("detectCycle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetExecutionOrder 测试执行顺序计算
func TestGetExecutionOrder(t *testing.T) {
	decomposer := &TaskDecomposer{}

	tasks := []models.DecomposedTask{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}

	order := decomposer.GetExecutionOrder(tasks)

	// 验证顺序：a 必须在 b、c 之前，b、c 必须在 d 之前
	posA := indexOf(order, "a")
	posB := indexOf(order, "b")
	posC := indexOf(order, "c")
	posD := indexOf(order, "d")

	if posA > posB || posA > posC {
		t.Error("task 'a' should come before 'b' and 'c'")
	}
	if posB > posD || posC > posD {
		t.Error("tasks 'b' and 'c' should come before 'd'")
	}
}

// TestShouldDecompose 测试是否需要拆解判断
func TestShouldDecompose(t *testing.T) {
	decomposer := &TaskDecomposer{}

	tests := []struct {
		goal      string
		shouldDecompose bool
	}{
		{"写一个 hello world 程序", false},
		{"实现一个用户登录 API 并且添加单元测试", true},
		{"设计数据库模型，然后创建迁移脚本，最后编写测试", true},
		{"这是一个非常长的目标描述，需要实现多个功能模块，包括用户认证、权限管理、数据同步等等", true},
	}

	for _, tt := range tests {
		result := decomposer.ShouldDecompose(tt.goal)
		if result != tt.shouldDecompose {
			t.Errorf("ShouldDecompose(%q) = %v, want %v", tt.goal, result, tt.shouldDecompose)
		}
	}
}

// TestBuildDecompositionPrompt 测试构建拆解提示
func TestBuildDecompositionPrompt(t *testing.T) {
	decomposer := &TaskDecomposer{
		config: DecomposerConfig{
			MaxSubTasks: 10,
		},
	}

	prompt := decomposer.buildDecompositionPrompt()

	// 检查关键内容
	keywords := []string{
		"任务拆解原则",
		"原子性",
		"依赖性",
		"JSON",
		"success",
		"tasks",
		"depends_on",
	}

	for _, kw := range keywords {
		if !contains(prompt, kw) {
			t.Errorf("prompt should contain keyword: %s", kw)
		}
	}
}

// TestJSONMarshaling 测试 JSON 序列化/反序列化
func TestJSONMarshaling(t *testing.T) {
	// 测试 DecompositionResult
	result := models.DecompositionResult{
		Success:   true,
		Reasoning: "test reasoning",
		Tasks: []models.DecomposedTask{
			{
				ID:          "t1",
				Title:       "Task 1",
				Description: "Description",
				Type:        "code",
				Priority:    models.PriorityHigh,
				DependsOn:   []string{},
			},
		},
		ExecutionOrder: []string{"t1"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded models.DecompositionResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Success != result.Success {
		t.Error("success mismatch")
	}
	if len(decoded.Tasks) != len(result.Tasks) {
		t.Error("tasks length mismatch")
	}
}

// Helper functions

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
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