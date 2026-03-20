package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// TaskDecomposer 任务拆解器
// 负责将复杂任务拆解为多个可执行的子任务
type TaskDecomposer struct {
	engine       *FunctionCallingEngine
	taskStore    store.TaskStore
	groupStore   store.TaskGroupStore
	depStore     store.TaskDependencyStore
	httpClient   HTTPClient

	// 配置
	config DecomposerConfig
}

// DecomposerConfig 拆解器配置
type DecomposerConfig struct {
	MaxSubTasks      int           // 最大子任务数量
	MinSubTasks      int           // 最小子任务数量
	DecomposeTimeout time.Duration // 拆解超时时间
	DefaultModel     ModelConfig   // 默认模型配置
}

// HTTPClient HTTP 客户端接口
type HTTPClient interface {
	Do(req interface{}) (interface{}, error)
}

// NewTaskDecomposer 创建任务拆解器
func NewTaskDecomposer(
	engine *FunctionCallingEngine,
	taskStore store.TaskStore,
	groupStore store.TaskGroupStore,
	depStore store.TaskDependencyStore,
	config DecomposerConfig,
) *TaskDecomposer {
	if config.MaxSubTasks <= 0 {
		config.MaxSubTasks = 10
	}
	if config.MinSubTasks <= 0 {
		config.MinSubTasks = 1
	}
	if config.DecomposeTimeout <= 0 {
		config.DecomposeTimeout = 60 * time.Second
	}

	return &TaskDecomposer{
		engine:     engine,
		taskStore:  taskStore,
		groupStore: groupStore,
		depStore:   depStore,
		config:     config,
	}
}

// DecomposeTask 拆解任务
// ctx: 上下文
// goal: 用户目标描述
// conversationID: 会话 ID
// modelCfg: 模型配置
// Returns: 任务组, 错误
func (d *TaskDecomposer) DecomposeTask(
	ctx context.Context,
	goal string,
	conversationID string,
	modelCfg ModelConfig,
) (*models.TaskGroup, error) {
	// 1. 调用 LLM 进行任务拆解
	decomposition, err := d.callLLMForDecomposition(ctx, goal, modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decompose task: %w", err)
	}

	if !decomposition.Success {
		return nil, fmt.Errorf("decomposition failed: %s", decomposition.Error)
	}

	// 2. 验证拆解结果
	if err := d.validateDecomposition(decomposition); err != nil {
		return nil, fmt.Errorf("invalid decomposition: %w", err)
	}

	// 3. 创建任务组
	group := &models.TaskGroup{
		ID:                   uuid.New().String(),
		ConversationID:       conversationID,
		OriginalGoal:         goal,
		DecompositionReasoning: decomposition.Reasoning,
		Status:               models.TaskGroupStatusPending,
		TotalTasks:           len(decomposition.Tasks),
	}

	if err := d.groupStore.Create(group); err != nil {
		return nil, fmt.Errorf("failed to create task group: %w", err)
	}

	// 4. 创建子任务和依赖关系
	taskIDMap := make(map[string]string) // LLM ID -> 实际 ID

	for _, dt := range decomposition.Tasks {
		task := &models.Task{
			ID:          uuid.New().String(),
			Type:        dt.Type,
			Title:       dt.Title,
			Description: dt.Description,
			Status:      models.TaskStatusPending,
			Priority:    dt.Priority,
			GroupID:     &group.ID,
			Input:       dt.Input,
			RequiredTags: models.StringArray(dt.RequiredTags),
		}

		if task.Priority == "" {
			task.Priority = models.PriorityMedium
		}

		if err := d.taskStore.Create(task); err != nil {
			return nil, fmt.Errorf("failed to create task: %w", err)
		}

		taskIDMap[dt.ID] = task.ID
	}

	// 5. 创建依赖关系
	for _, dt := range decomposition.Tasks {
		for _, depID := range dt.DependsOn {
			actualTaskID := taskIDMap[dt.ID]
			actualDepID := taskIDMap[depID]

			dep := &models.TaskDependency{
				TaskID:         actualTaskID,
				DependsOnTaskID: actualDepID,
				Type:           models.DependencyTypeFinish,
				IsSatisfied:     false,
			}

			if err := d.depStore.Create(dep); err != nil {
				return nil, fmt.Errorf("failed to create dependency: %w", err)
			}
		}
	}

	return group, nil
}

// callLLMForDecomposition 调用 LLM 进行任务拆解
func (d *TaskDecomposer) callLLMForDecomposition(
	ctx context.Context,
	goal string,
	modelCfg ModelConfig,
) (*models.DecompositionResult, error) {
	// 构建系统提示
	systemPrompt := d.buildDecompositionPrompt()

	// 构建用户消息
	userMessage := fmt.Sprintf(`请分析以下目标并拆解为可执行的子任务：

目标: %s

请按照 JSON 格式返回拆解结果。`, goal)

	// 构建请求
	messages := []ChatMessage{
		{Role: "user", Content: userMessage},
	}

	req, err := d.engine.BuildOpenAIRequest(modelCfg.Model, messages, systemPrompt, nil)
	if err != nil {
		return nil, err
	}

	// 设置响应格式为 JSON
	req.MaxTokens = 4096
	req.Temperature = 0.3 // 较低的温度以获得更确定的输出

	// 调用 API
	resp, err := d.engine.CallOpenAI(modelCfg, req)
	if err != nil {
		return nil, err
	}

	// 解析响应
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	// 解析 JSON
	return d.parseDecompositionResponse(content)
}

// buildDecompositionPrompt 构建任务拆解的系统提示
func (d *TaskDecomposer) buildDecompositionPrompt() string {
	return `你是一个专业的任务规划助手，负责将复杂目标拆解为可执行的子任务。

## 任务拆解原则

1. **原子性**: 每个子任务应该是独立可完成的
2. **依赖性**: 明确标识任务之间的依赖关系
3. **可执行性**: 每个任务都应该有明确的完成标准
4. **优先级**: 根据重要性和依赖关系设置优先级
5. **适度粒度**: 不要过度拆分，每个任务应该有意义

## 任务类型说明

- **code**: 代码编写、修改、重构
- **research**: 调研、分析、文档阅读
- **file**: 文件操作（创建、修改、删除）
- **shell**: 命令执行
- **web**: 网络请求、API 调用
- **review**: 代码审查、测试
- **deploy**: 部署、配置

## 输出格式

请严格按照以下 JSON 格式输出：

` + "```json" + `
{
  "success": true,
  "reasoning": "拆解思路和整体策略说明",
  "tasks": [
    {
      "id": "task-1",
      "title": "任务标题（简洁明了）",
      "description": "任务详细描述，包括具体要做什么",
      "type": "code",
      "priority": "high",
      "input": {
        "key": "value"
      },
      "depends_on": [],
      "required_tags": ["dev", "coding"],
      "estimated_steps": 3
    },
    {
      "id": "task-2",
      "title": "任务标题",
      "description": "任务描述",
      "type": "file",
      "priority": "medium",
      "depends_on": ["task-1"],
      "estimated_steps": 2
    }
  ],
  "execution_order": ["task-1", "task-2"]
}
` + "```" + `

## 优先级说明

- **high**: 核心任务，必须优先完成
- **medium**: 重要但非紧急
- **low**: 可以后续完成

## 注意事项

1. 任务 ID 只在本次拆解内有效，用于标识依赖关系
2. depends_on 数组中的 ID 必须是其他任务的 ID
3. 如果目标本身很简单，可以直接返回单个任务
4. 最多拆分为 ` + fmt.Sprintf("%d", d.config.MaxSubTasks) + ` 个子任务
5. 确保依赖关系不形成循环

## 示例

目标: 实现一个用户登录 API

` + "```json" + `
{
  "success": true,
  "reasoning": "实现用户登录 API 需要数据模型、业务逻辑和 HTTP 接口三部分，按依赖关系依次实现。",
  "tasks": [
    {
      "id": "task-1",
      "title": "设计用户数据模型",
      "description": "创建 User 结构体，包含 ID、Username、PasswordHash、Email 等字段",
      "type": "code",
      "priority": "high",
      "depends_on": [],
      "estimated_steps": 2
    },
    {
      "id": "task-2",
      "title": "实现密码加密验证",
      "description": "使用 bcrypt 实现密码加密和验证函数",
      "type": "code",
      "priority": "high",
      "depends_on": [],
      "estimated_steps": 2
    },
    {
      "id": "task-3",
      "title": "实现登录业务逻辑",
      "description": "实现登录验证逻辑，包括用户查询、密码验证、Token 生成",
      "type": "code",
      "priority": "high",
      "depends_on": ["task-1", "task-2"],
      "estimated_steps": 3
    },
    {
      "id": "task-4",
      "title": "创建登录 HTTP 接口",
      "description": "实现 POST /api/login 接口，处理请求并返回 Token",
      "type": "code",
      "priority": "high",
      "depends_on": ["task-3"],
      "estimated_steps": 2
    },
    {
      "id": "task-5",
      "title": "编写单元测试",
      "description": "为登录功能编写单元测试",
      "type": "code",
      "priority": "medium",
      "depends_on": ["task-4"],
      "estimated_steps": 3
    }
  ],
  "execution_order": ["task-1", "task-2", "task-3", "task-4", "task-5"]
}
` + "```" + `

请根据以上说明进行任务拆解。`
}

// parseDecompositionResponse 解析拆解响应
func (d *TaskDecomposer) parseDecompositionResponse(content string) (*models.DecompositionResult, error) {
	// 尝试提取 JSON
	jsonStr := content

	// 如果响应包含 markdown 代码块，提取其中的 JSON
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json")
		if start != -1 {
			start += 7
			end := strings.Index(content[start:], "```")
			if end != -1 {
				jsonStr = strings.TrimSpace(content[start : start+end])
			}
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```")
		if start != -1 {
			start += 3
			end := strings.Index(content[start:], "```")
			if end != -1 {
				jsonStr = strings.TrimSpace(content[start : start+end])
			}
		}
	}

	var result models.DecompositionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// 尝试修复常见的 JSON 格式问题
		jsonStr = d.fixJSON(jsonStr)
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return nil, fmt.Errorf("failed to parse decomposition JSON: %w", err)
		}
	}

	return &result, nil
}

// fixJSON 尝试修复 JSON 格式问题
func (d *TaskDecomposer) fixJSON(jsonStr string) string {
	// 移除可能的注释
	lines := strings.Split(jsonStr, "\n")
	var cleaned []string
	for _, line := range lines {
		// 移除单行注释
		if idx := strings.Index(line, "//"); idx != -1 {
			line = line[:idx]
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

// validateDecomposition 验证拆解结果
func (d *TaskDecomposer) validateDecomposition(result *models.DecompositionResult) error {
	if len(result.Tasks) == 0 {
		return fmt.Errorf("no tasks in decomposition")
	}

	if len(result.Tasks) > d.config.MaxSubTasks {
		return fmt.Errorf("too many tasks: %d > %d", len(result.Tasks), d.config.MaxSubTasks)
	}

	// 检查任务 ID 唯一性
	taskIDs := make(map[string]bool)
	for _, task := range result.Tasks {
		if task.ID == "" {
			return fmt.Errorf("task missing ID")
		}
		if taskIDs[task.ID] {
			return fmt.Errorf("duplicate task ID: %s", task.ID)
		}
		taskIDs[task.ID] = true
	}

	// 检查依赖关系
	for _, task := range result.Tasks {
		for _, depID := range task.DependsOn {
			if !taskIDs[depID] {
				return fmt.Errorf("task %s depends on non-existent task %s", task.ID, depID)
			}
		}
	}

	// 检查循环依赖
	if err := d.detectCycle(result.Tasks); err != nil {
		return err
	}

	return nil
}

// detectCycle 检测循环依赖
func (d *TaskDecomposer) detectCycle(tasks []models.DecomposedTask) error {
	// 构建邻接表
	graph := make(map[string][]string)
	for _, task := range tasks {
		graph[task.ID] = task.DependsOn
	}

	// DFS 检测环
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(id string) bool
	dfs = func(id string) bool {
		visited[id] = true
		recStack[id] = true

		for _, dep := range graph[id] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				return true // 发现环
			}
		}

		recStack[id] = false
		return false
	}

	for _, task := range tasks {
		if !visited[task.ID] {
			if dfs(task.ID) {
				return fmt.Errorf("circular dependency detected")
			}
		}
	}

	return nil
}

// GetExecutionOrder 获取执行顺序（拓扑排序）
func (d *TaskDecomposer) GetExecutionOrder(tasks []models.DecomposedTask) []string {
	// 计算入度
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for _, task := range tasks {
		inDegree[task.ID] = 0
	}

	for _, task := range tasks {
		for _, dep := range task.DependsOn {
			graph[dep] = append(graph[dep], task.ID)
			inDegree[task.ID]++
		}
	}

	// BFS 拓扑排序
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, next := range graph[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	return order
}

// ShouldDecompose 判断是否需要拆解
func (d *TaskDecomposer) ShouldDecompose(goal string) bool {
	// 简单的启发式规则判断是否需要拆解
	keywords := []string{
		"实现", "开发", "构建", "设计", "创建",
		"重构", "迁移", "集成", "部署",
		"并且", "然后", "之后", "同时",
		"包括", "包含", "以及", "还有",
	}

	count := 0
	for _, kw := range keywords {
		if strings.Contains(goal, kw) {
			count++
		}
	}

	// 超过 2 个关键词或目标长度超过 100 字符，建议拆解
	return count >= 2 || len(goal) > 100
}