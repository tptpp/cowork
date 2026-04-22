# Agent Template Integration Implementation Plan

> **Status: SUPERSEDED (2026-04-16)**
>
> This plan is outdated due to the Agent Architecture Simplification project.
> TaskDecomposer, ToolScheduler, and FunctionCallingEngine have been deleted.
>
> See `docs/superpowers/specs/2026-04-16-agent-architecture-simplification-design.md` for the new architecture.
> Template integration is now handled by the unified Agent structure in `internal/coordinator/agent/agent.go`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate AgentTemplate into the actual execution flow for role-based permissions, model config, and approval handling.

**Architecture:** Template-driven execution - Session selects Coordinator/Worker templates, TaskDecomposer infers template per task, Worker enforces tool permissions and approval levels.

**Tech Stack:** Go (GORM), React (TypeScript), WebSocket

---

## Phase 1: Data Model Changes

### Task 1: Update AgentTemplate Model

**Files:**
- Modify: `internal/shared/models/agent.go:8-36`

- [ ] **Step 1: Add model config fields to AgentTemplate struct**

```go
// internal/shared/models/agent.go

type AgentTemplate struct {
    ID          string `gorm:"primaryKey;type:varchar(64)" json:"id"`
    Name        string `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
    Description string `gorm:"type:text" json:"description"`

    // 提示词
    BasePrompt   string `gorm:"type:text" json:"base_prompt"`
    SystemPrompt string `gorm:"type:text" json:"system_prompt"`

    // 模型配置 (新增)
    DefaultModel  string  `gorm:"type:varchar(50)" json:"default_model"`   // 如 "gpt-4"
    MaxTokens     int     `gorm:"default:4096" json:"max_tokens"`
    Temperature   float64 `gorm:"default:0.7" json:"temperature"`

    // 工具配置
    AllowedTools    StringArray `gorm:"type:text" json:"allowed_tools"`
    RestrictedTools StringArray `gorm:"type:text" json:"restricted_tools"`

    // 需要的节点能力
    RequiredCapabilities NodeCapabilities `gorm:"type:text" json:"required_capabilities"`

    // 默认审批级别
    DefaultApprovalLevel string `gorm:"type:varchar(20);default:'medium'" json:"default_approval_level"`

    // 是否系统预设
    IsSystem bool `gorm:"default:false" json:"is_system"`

    // 创建者
    CreatedBy *string `gorm:"type:varchar(64)" json:"created_by"`

    // 时间
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
```

- [ ] **Step 2: Build and verify**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
cd /home/tp/.openclaw/workspace/projects/cowork
git add internal/shared/models/agent.go
git commit -m "feat: add model config fields to AgentTemplate

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Simplify AgentSession Model

**Files:**
- Modify: `internal/shared/models/models.go:301-330`

- [ ] **Step 1: Replace AgentSession struct fields**

```go
// internal/shared/models/models.go (replace AgentSession struct)

// AgentSession Agent 对话会话
type AgentSession struct {
    ID string `gorm:"primaryKey;type:varchar(64)" json:"id"`

    // 模板配置 (替代 Model, SystemPrompt, Config)
    CoordinatorTemplateID string      `gorm:"type:varchar(64);default:'coordinator-template'" json:"coordinator_template_id"`
    WorkerTemplateIDs     StringArray `gorm:"type:text" json:"worker_template_ids"` // 空数组=自动选择

    // 上下文
    Context JSON `gorm:"type:text" json:"context"`

    // 关联任务
    TaskID *string `gorm:"type:varchar(64);index" json:"task_id"`
    Task   *Task   `gorm:"foreignKey:TaskID" json:"task,omitempty"`

    // 时间
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

    // 消息
    Messages []AgentMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: Compilation errors in handler/agent.go (need to fix references)

- [ ] **Step 3: Commit**

```bash
git add internal/shared/models/models.go
git commit -m "feat: simplify AgentSession with template-based config

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Update CreateSession API Handler

**Files:**
- Modify: `internal/coordinator/handler/agent.go:181-218`

- [ ] **Step 1: Update CreateAgentSessionRequest struct**

```go
// internal/coordinator/handler/agent.go

// CreateAgentSessionRequest 创建 Agent 会话请求
type CreateAgentSessionRequest struct {
    CoordinatorTemplateID string      `json:"coordinator_template_id"` // 可选，默认 coordinator-template
    WorkerTemplateIDs     StringArray `json:"worker_template_ids"`     // 可选，空数组=自动
    Context               models.JSON `json:"context"`
    TaskID                *string     `json:"task_id"`
}

// CreateAgentSession 创建 Agent 会话
func (h *AgentHandler) CreateAgentSession(c *gin.Context) {
    var req CreateAgentSessionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        failWithError(c, errors.InvalidRequest(err.Error()))
        return
    }

    // 设置默认 Coordinator 模板
    if req.CoordinatorTemplateID == "" {
        req.CoordinatorTemplateID = "coordinator-template"
    }

    session := &models.AgentSession{
        ID:                   uuid.New().String(),
        CoordinatorTemplateID: req.CoordinatorTemplateID,
        WorkerTemplateIDs:     req.WorkerTemplateIDs,
        Context:              req.Context,
        TaskID:               req.TaskID,
    }

    if err := h.store.Create(session); err != nil {
        failWithError(c, errors.WrapInternal("Failed to create session", err))
        return
    }

    success(c, session)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/handler/agent.go
git commit -m "feat: update CreateSession API for template config

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Change read_file Tool to Local Execution

**Files:**
- Modify: `internal/coordinator/tools/builtin.go` (find read_file definition)

- [ ] **Step 1: Find and update read_file ExecuteMode**

First, locate the read_file tool definition:

Run: `grep -n "read_file" internal/coordinator/tools/builtin.go`

Change ExecuteMode from "remote" to "local":

```go
// internal/coordinator/tools/builtin.go (read_file definition)
{
    Name:        "read_file",
    Description: "Read file contents from the filesystem",
    ExecuteMode: models.ToolExecuteModeLocal, // 改为 local
    ...
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/tools/builtin.go
git commit -m "feat: change read_file to local execution mode

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 2: TaskDecomposer Changes

### Task 5: Update DecomposedTask Structure

**Files:**
- Modify: `internal/coordinator/agent/task_decomposer.go`

- [ ] **Step 1: Add TemplateID to DecomposedTask struct**

Find the DecomposedTask struct definition and add TemplateID:

```go
// internal/coordinator/agent/task_decomposer.go

type DecomposedTask struct {
    ID          string
    Type        string
    Title       string
    Description string
    Priority    models.Priority
    RequiredTags []string
    DependsOn   []string
    Input       models.JSON
    
    // 新增
    TemplateID string // LLM 推断的角色模板
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/agent/task_decomposer.go
git commit -m "feat: add TemplateID to DecomposedTask struct

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 6: Update TaskDecomposer Prompt

**Files:**
- Modify: `internal/coordinator/agent/task_decomposer.go:209-250` (buildDecompositionPrompt)

- [ ] **Step 1: Add template selection to prompt**

Replace the buildDecompositionPrompt function to include template selection:

```go
// internal/coordinator/agent/task_decomposer.go

func (d *TaskDecomposer) buildDecompositionPrompt(workerTemplateIDs []string) string {
    templateSection := `
## Agent Templates

Available templates for sub-tasks:
- coordinator-template: Task coordination, scheduling, monitoring
- dev-template: Code development, file editing, API implementation
- test-template: Testing execution, result analysis, bug reporting
- review-template: Code review, security audit, best practices
- deploy-template: Deployment, environment configuration (requires approval)
- research-template: Technical research, documentation analysis
`

    if len(workerTemplateIDs) > 0 {
        templateSection += fmt.Sprintf(`
**IMPORTANT**: Only use these templates: %v
Do not use templates outside this list.
`, workerTemplateIDs)
    }

    return `你是一个专业的任务规划助手，负责将复杂目标拆解为可执行的子任务。

` + templateSection + `

## 任务拆解原则

1. **原子性**: 每个子任务应该是独立可完成的
2. **依赖性**: 明确标识任务之间的依赖关系
3. **可执行性**: 每个任务都应该有明确的完成标准
4. **优先级**: 根据重要性和依赖关系设置优先级
5. **适度粒度**: 不要过度拆分，每个任务应该有意义

## 输出格式

请严格按照以下 JSON 格式输出：

` + "```json" + `
{
  "success": true,
  "reasoning": "拆解思路和整体策略说明",
  "tasks": [
    {
      "id": "task-1",
      "title": "任务标题",
      "description": "任务详细描述",
      "type": "code",
      "priority": "high",
      "template_id": "dev-template",
      "input": {},
      "depends_on": [],
      "required_tags": ["dev", "coding"]
    }
  ]
}
` + "```" + `
`
}
```

- [ ] **Step 2: Update callLLMForDecomposition to pass workerTemplateIDs**

The method signature and call need to be updated to pass the template constraints.

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/coordinator/agent/task_decomposer.go
git commit -m "feat: add template selection to decomposition prompt

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Update Task Creation with TemplateID

**Files:**
- Modify: `internal/coordinator/agent/task_decomposer.go:112-134` (task creation loop)

- [ ] **Step 1: Add TemplateID to task creation**

```go
// internal/coordinator/agent/task_decomposer.go (in DecomposeTask method)

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
        TemplateID:  dt.TemplateID, // 新增
    }

    if task.Priority == "" {
        task.Priority = models.PriorityMedium
    }

    if err := d.taskStore.Create(task); err != nil {
        return nil, fmt.Errorf("failed to create task: %w", err)
    }

    taskIDMap[dt.ID] = task.ID
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/agent/task_decomposer.go
git commit -m "feat: write TemplateID to Task during decomposition

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 3: Coordinator Execution

### Task 8: Load Template for Coordinator System Prompt

**Files:**
- Modify: `internal/coordinator/agent/coordinator.go`
- Modify: `cmd/coordinator/main.go`

- [ ] **Step 1: Add TemplateStore to ConversationCoordinator**

```go
// internal/coordinator/agent/coordinator.go

type ConversationCoordinator struct {
    engine         *FunctionCallingEngine
    scheduler      *ToolScheduler
    sessionStore   store.AgentSessionStore
    toolExecStore  store.ToolExecutionStore
    registry       *tools.Registry
    
    // Phase 5: 任务拆解相关
    decomposer     *TaskDecomposer
    groupStore     store.TaskGroupStore
    depStore       store.TaskDependencyStore
    
    // 新增
    templateStore  store.AgentTemplateStore
}
```

- [ ] **Step 2: Update constructor to accept templateStore**

```go
// NewConversationCoordinatorWithDecomposer
func NewConversationCoordinatorWithDecomposer(
    ...
    templateStore store.AgentTemplateStore, // 新增参数
    config CoordinatorConfig,
) *ConversationCoordinator {
    ...
    return &ConversationCoordinator{
        ...
        templateStore: templateStore,
    }
}
```

- [ ] **Step 3: Load Template in ProcessMessage**

```go
// ProcessMessage method - load template for system prompt

func (c *ConversationCoordinator) ProcessMessage(...) (*ProcessResult, error) {
    // 加载 Coordinator Template
    template, err := c.templateStore.Get(session.CoordinatorTemplateID)
    if err != nil {
        template = c.getDefaultCoordinatorTemplate()
    }
    
    systemPrompt := template.BasePrompt
    
    // 后续使用 systemPrompt 调用 LLM
    ...
}
```

- [ ] **Step 4: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/coordinator/agent/coordinator.go cmd/coordinator/main.go
git commit -m "feat: load Template for Coordinator system prompt

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 4: Worker Execution

### Task 9: Add Template Permission Check to ToolExecutor

**Files:**
- Modify: `internal/worker/executor/tool_executor.go`

- [ ] **Step 1: Add checkToolPermission method**

```go
// internal/worker/executor/tool_executor.go

func (e *ToolExecutor) checkToolPermission(toolName string, template *models.AgentTemplate) error {
    // 1. 检查黑名单
    for _, t := range template.RestrictedTools {
        if t == toolName {
            return fmt.Errorf("tool '%s' is restricted for template '%s'", toolName, template.ID)
        }
    }
    
    // 2. 检查白名单 (如果有定义)
    if len(template.AllowedTools) > 0 {
        allowed := false
        for _, t := range template.AllowedTools {
            if t == toolName {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("tool '%s' not in allowed list for template '%s'", toolName, template.ID)
        }
    }
    
    return nil
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/worker/executor/tool_executor.go
git commit -m "feat: add Template permission check to ToolExecutor

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 10: Add Approval Polling to Worker

**Files:**
- Modify: `cmd/worker/main.go` (CoordinatorClient)

- [ ] **Step 1: Add approval request method to CoordinatorClient**

```go
// cmd/worker/main.go

// ApprovalRequest 审批请求
type ApprovalRequest struct {
    TaskID      string          `json:"task_id"`
    ToolName    string          `json:"tool_name"`
    Arguments   map[string]interface{} `json:"arguments"`
    RiskLevel   string          `json:"risk_level"`
}

// ApprovalResponse 审批响应
type ApprovalResponse struct {
    ID     string `json:"id"`
    Status string `json:"status"` // pending, approved, rejected
}

// RequestApproval 请求审批
func (c *CoordinatorClient) RequestApproval(taskID string, toolName string, args map[string]interface{}) (*ApprovalResponse, error) {
    req := ApprovalRequest{
        TaskID:    taskID,
        ToolName:  toolName,
        Arguments: args,
        RiskLevel: "high",
    }
    
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", c.baseURL+"/api/approvals", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result ApprovalResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}

// GetApproval 查询审批状态
func (c *CoordinatorClient) GetApproval(approvalID string) (*ApprovalResponse, error) {
    httpReq, _ := http.NewRequest("GET", c.baseURL+"/api/approvals/"+approvalID, nil)
    
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result ApprovalResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}

// WaitApproval 等待审批结果（轮询）
func (c *CoordinatorClient) WaitApproval(approvalID string, timeout time.Duration) (bool, error) {
    deadline := time.Now().Add(timeout)
    
    for time.Now().Before(deadline) {
        result, err := c.GetApproval(approvalID)
        if err != nil {
            time.Sleep(1 * time.Second)
            continue
        }
        
        switch result.Status {
        case "approved":
            return true, nil
        case "rejected":
            return false, fmt.Errorf("approval rejected")
        }
        
        time.Sleep(1 * time.Second)
    }
    
    return false, fmt.Errorf("approval timeout")
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/worker/main.go
git commit -m "feat: add approval polling methods to Worker CoordinatorClient

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 5: Approval API Endpoints

### Task 11: Create Approval Handler

**Files:**
- Create: `internal/coordinator/handler/approval.go`

- [ ] **Step 1: Create approval handler file**

```go
// internal/coordinator/handler/approval.go

package handler

import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/tp/cowork/internal/coordinator/approval"
    "github.com/tp/cowork/internal/shared/models"
)

// ApprovalHandler 审批处理器
type ApprovalHandler struct {
    service *approval.Service
}

// NewApprovalHandler 创建审批处理器
func NewApprovalHandler(service *approval.Service) *ApprovalHandler {
    return &ApprovalHandler{service: service}
}

// CreateApprovalRequest 创建审批请求
type CreateApprovalRequest struct {
    AgentID string          `json:"agent_id"`
    Action  string          `json:"action"`
    Detail  models.JSON     `json:"detail"`
}

// Create 创建审批请求 (Worker 调用)
func (h *ApprovalHandler) Create(c *gin.Context) {
    var req CreateApprovalRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    approvalReq, err := h.service.CreateRequest(c.Request.Context(), req.AgentID, req.Action, req.Detail)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, approvalReq)
}

// Get 查询审批状态 (Worker 调用)
func (h *ApprovalHandler) Get(c *gin.Context) {
    id := c.Param("id")
    
    req, err := h.service.reqStore.Get(id)
    if err != nil {
        c.JSON(404, gin.H{"error": "approval not found"})
        return
    }
    
    c.JSON(200, req)
}

// Approve 批准审批 (前端调用)
func (h *ApprovalHandler) Approve(c *gin.Context) {
    id := c.Param("id")
    
    var req struct {
        UserID string `json:"user_id"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    if err := h.service.Approve(c.Request.Context(), id, req.UserID); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"status": "approved"})
}

// Reject 拒绝审批 (前端调用)
func (h *ApprovalHandler) Reject(c *gin.Context) {
    id := c.Param("id")
    
    var req struct {
        UserID string `json:"user_id"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    if err := h.service.Reject(c.Request.Context(), id, req.UserID); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"status": "rejected"})
}

// ListPending 列出待审批 (前端调用)
func (h *ApprovalHandler) ListPending(c *gin.Context) {
    reqs, err := h.service.ListPending(c.Request.Context(), 50)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, reqs)
}

// RegisterRoutes 注册路由
func (h *ApprovalHandler) RegisterRoutes(r *gin.RouterGroup) {
    r.POST("/approvals", h.Create)
    r.GET("/approvals/:id", h.Get)
    r.POST("/approvals/:id/approve", h.Approve)
    r.POST("/approvals/:id/reject", h.Reject)
    r.GET("/approvals/pending", h.ListPending)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/handler/approval.go
git commit -m "feat: create ApprovalHandler for Worker polling API

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 12: Wire Approval Handler to Coordinator

**Files:**
- Modify: `cmd/coordinator/main.go`

- [ ] **Step 1: Initialize and register ApprovalHandler**

```go
// cmd/coordinator/main.go (在 handler 初始化区域)

// 初始化 Approval Handler
approvalHandler := handler.NewApprovalHandler(approvalService)

// 注册 Approval API
approvalHandler.RegisterRoutes(api)
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/coordinator/main.go
git commit -m "feat: wire ApprovalHandler to Coordinator API

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 6: Frontend UI

### Task 13: Update AgentChatWidget with Template Selection

**Files:**
- Modify: `web/src/components/widgets/AgentChatWidget.tsx`

- [ ] **Step 1: Add template selection state and UI**

This is a frontend task requiring React/TypeScript changes. The key changes:

1. Add state for selected templates:
```typescript
const [coordinatorTemplate, setCoordinatorTemplate] = useState('coordinator-template');
const [workerTemplates, setWorkerTemplates] = useState<string[]>([]);
const [showAdvanced, setShowAdvanced] = useState(false);
```

2. Add expandable advanced settings panel with template checkboxes.

3. Update createSession API call to pass template fields.

- [ ] **Step 2: Build and verify**

Run: `cd web && npm run build`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/src/components/widgets/AgentChatWidget.tsx
git commit -m "feat: add template selection UI to AgentChatWidget

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 14: Add WebSocket Approval Notification

**Files:**
- Modify: `internal/coordinator/ws/hub.go`
- Modify: `web/src/stores/approvalStore.ts` (or create)

- [ ] **Step 1: Add approval notification channel to WebSocket Hub**

```go
// internal/coordinator/ws/hub.go

// BroadcastApproval 广播审批请求
func (h *Hub) BroadcastApproval(approval *models.ApprovalRequest) {
    msg := Message{
        Type: "approval_request",
        Channel: "approvals",
        Data: approval,
    }
    h.BroadcastToChannel("approvals", msg)
}
```

- [ ] **Step 2: Call broadcast in ApprovalService.CreateRequest**

- [ ] **Step 3: Build and verify**

Run: `go build ./... && cd web && npm run build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/coordinator/ws/hub.go internal/coordinator/approval/service.go web/src/stores/approvalStore.ts
git commit -m "feat: add WebSocket approval notification

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Final Verification

### Task 15: Integration Test

- [ ] **Step 1: Run full build**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go build ./... && cd web && npm run build`
Expected: All pass

- [ ] **Step 2: Run existing tests**

Run: `cd /home/tp/.openclaw/workspace/projects/cowork && go test ./...`
Expected: All pass

- [ ] **Step 3: Manual smoke test**

1. Start Coordinator
2. Start Worker
3. Open frontend, create session with template selection
4. Send message, verify template-driven execution

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete Agent Template integration

- AgentTemplate now drives model config, tool permissions, approval level
- AgentSession simplified to template-based config
- TaskDecomposer infers template per sub-task
- Coordinator loads template for system prompt
- Worker enforces tool permissions and approval flow
- Frontend UI for template selection

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## File Changes Summary

| File | Task | Change |
|------|------|--------|
| `internal/shared/models/agent.go` | 1 | Add DefaultModel, MaxTokens, Temperature |
| `internal/shared/models/models.go` | 2 | Simplify AgentSession |
| `internal/coordinator/handler/agent.go` | 3 | Update CreateSession API |
| `internal/coordinator/tools/builtin.go` | 4 | read_file → local |
| `internal/coordinator/agent/task_decomposer.go` | 5-7 | TemplateID + prompt |
| `internal/coordinator/agent/coordinator.go` | 8 | Load Template |
| `cmd/coordinator/main.go` | 8,12 | Wire templateStore, approvalHandler |
| `internal/worker/executor/tool_executor.go` | 9 | Permission check |
| `cmd/worker/main.go` | 10 | Approval polling |
| `internal/coordinator/handler/approval.go` | 11 | New file |
| `internal/coordinator/ws/hub.go` | 14 | Approval broadcast |
| `web/src/components/widgets/AgentChatWidget.tsx` | 13 | Template selection UI |