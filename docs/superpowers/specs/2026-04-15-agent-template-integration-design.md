# Agent Template Integration Design

> **Status: SUPERSEDED (2026-04-16)**
>
> This design is outdated due to the Agent Architecture Simplification project.
> TaskDecomposer, ToolScheduler, and FunctionCallingEngine have been deleted.
>
> See `docs/superpowers/specs/2026-04-16-agent-architecture-simplification-design.md` for the new architecture.
> Template integration is now handled by the unified Agent structure.

**Date**: 2026-04-15
**Original Status**: Approved

## Overview

Integrate Agent Template into the actual execution flow. Template becomes the core concept for:
- Role definition (Coordinator/Developer/Tester/etc)
- Tool permissions (AllowedTools/RestrictedTools)
- Approval level (Low/Medium/High)
- Model configuration (DefaultModel/MaxTokens/Temperature)

## Goals

1. **Permission Control** - Define what tools an Agent can use
2. **Resource Matching** - Match tasks to workers by RequiredCapabilities
3. **Simplified UX** - User selects role, system handles the rest
4. **Approval Flow** - High-risk operations require human approval

## Architecture

### Coordinator vs Worker Roles

| Component | Role | Local Tools | Remote Tools |
|-----------|------|-------------|--------------|
| Coordinator | Brain + Simple queries | read_file, query_task, create_task, request_approval | None |
| Worker | Execution | None | write_file, execute_shell, deploy |

### Template Application Flow

```
User Message → Coordinator (Session Template)
        ↓
    Analyze & Decompose
        ↓
    Tasks (LLM inffers TemplateID)
        ↓
    Scheduler matches RequiredCapabilities
        ↓
    Worker executes with Template permissions
```

## Data Model Changes

### AgentTemplate (Modified)

```go
type AgentTemplate struct {
    ID          string `gorm:"primaryKey" json:"id"`
    Name        string `gorm:"uniqueIndex" json:"name"`
    Description string `json:"description"
    
    // Prompts
    BasePrompt string `json:"base_prompt"`
    
    // Model config (NEW)
    DefaultModel  string  `json:"default_model"`   // "gpt-4", "claude-3-opus"
    MaxTokens     int     `json:"max_tokens"`      // 4096
    Temperature   float64 `json:"temperature"`     // 0.7
    
    // Tool permissions
    AllowedTools    StringArray `json:"allowed_tools"`
    RestrictedTools StringArray `json:"restricted_tools"`
    
    // Worker requirements
    RequiredCapabilities NodeCapabilities `json:"required_capabilities"`
    
    // Approval level
    DefaultApprovalLevel string `json:"default_approval_level"` // low/medium/high
    
    IsSystem bool `json:"is_system"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### AgentSession (Modified)

```go
type AgentSession struct {
    ID string `gorm:"primaryKey" json:"id"`
    
    // Template config (NEW - replaces Model, SystemPrompt, Config)
    CoordinatorTemplateID string      `gorm:"default:'coordinator-template'" json:"coordinator_template_id"`
    WorkerTemplateIDs     StringArray `json:"worker_template_ids"` // empty = auto
    
    // Context
    Context JSON `json:"context"`
    
    // Task association
    TaskID *string `json:"task_id"`
    
    // Timestamps
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

**Removed fields**: `Model`, `SystemPrompt`, `Config` - all derived from Template.

### Task (Modified)

```go
type Task struct {
    // ... existing fields ...
    
    // NEW
    TemplateID *string `json:"template_id"`
}
```

### ToolDefinition ExecuteMode Change

```go
// read_file: remote → local (Coordinator can query directly)
ToolDefinition{
    Name: "read_file",
    ExecuteMode: "local", // CHANGED from "remote"
}
```

## TaskDecomposer Changes

### DecomposedTask (Modified)

```go
type DecomposedTask struct {
    ID          string
    Type        string
    Title       string
    Description string
    Priority    Priority
    RequiredTags []string
    DependsOn   []string
    Input       JSON
    
    // NEW
    TemplateID string // LLM infers
}
```

### Prompt Enhancement

Add template selection instructions to decomposition prompt:

```
Available Agent Templates:
- coordinator-template: Task coordination, scheduling
- dev-template: Code development, file editing
- test-template: Testing, bug reporting
- review-template: Code review, security audit
- deploy-template: Deployment (requires approval)
- research-template: Research, documentation

For each sub-task, specify template_id.
If WorkerTemplateIDs is specified, only use those templates.

Return JSON with template_id field for each task.
```

## Approval Flow

### API Endpoints

| Method | Path | Caller | Purpose |
|--------|------|--------|---------|
| POST | /api/approvals | Worker | Create approval request |
| GET | /api/approvals/:id | Worker | Poll approval status |
| POST | /api/approvals/:id/approve | Frontend | Approve request |
| POST | /api/approvals/:id/reject | Frontend | Reject request |
| GET | /api/approvals/pending | Frontend | List pending requests |

### WebSocket Push

- Channel: `approval_requests`
- Event: `new_approval` → Frontend ApprovalWidget displays

### Worker Polling Flow

```go
func executeWithApproval(tool, args) {
    // 1. Create approval request
    approval := POST /api/approvals
    
    // 2. Poll status (1 second interval)
    for i := 0; i < timeout_seconds; i++ {
        result := GET /api/approvals/:id
        if result.Status == "approved" {
            return execute(tool, args)
        }
        if result.Status == "rejected" {
            return error("rejected")
        }
        sleep(1s)
    }
    
    return error("timeout")
}
```

### Approval Levels Behavior

| Level | Behavior |
|-------|----------|
| low | Auto-approved, execute immediately |
| medium | Auto-approved after timeout (5s default) |
| high | Must wait for human approval, no timeout |

## Frontend UI

### Create Session Widget

```
┌─────────────────────────────────────┐
│ Agent Chat                          │
├─────────────────────────────────────┤
│ [Message input box]                 │
│                                     │
│ [⚙ Advanced Settings] ← Expandable  │
│                                     │
│ ─────── Expanded ───────            │
│ Coordinator: [Coordinator ▼]        │
│                                     │
│ Worker (optional, multiple):        │
│   ☐ Developer    ☐ Tester           │
│   ☐ Deployer     ☐ Researcher       │
│   ☐ Reviewer                       │
│                                     │
│ [Collapse]                          │
│                                     │
│ [Send]                              │
└─────────────────────────────────────┘
```

**Default values**:
- CoordinatorTemplateID = "coordinator-template"
- WorkerTemplateIDs = [] (empty, LLM auto-selects)

### Approval Widget

Real-time display of pending approvals via WebSocket.

## Implementation Checklist

### Phase 1: Data Model
- [ ] AgentTemplate add DefaultModel, MaxTokens, Temperature
- [ ] AgentSession replace Model/SystemPrompt/Config with Template fields
- [ ] Task add TemplateID
- [ ] ToolDefinition change read_file to local

### Phase 2: TaskDecomposer
- [ ] Update prompt with template selection
- [ ] DecomposedTask add TemplateID
- [ ] Task creation write TemplateID

### Phase 3: Coordinator Execution
- [ ] Load Template.BasePrompt as system_prompt
- [ ] Execute local tools (read_file) directly

### Phase 4: Worker Execution
- [ ] Load Template from Task.TemplateID
- [ ] Check AllowedTools/RestrictedTools before execution
- [ ] Implement approval polling flow

### Phase 5: Frontend
- [ ] AgentChatWidget add advanced settings panel
- [ ] ApprovalWidget real-time updates
- [ ] Create session API pass template fields

### Phase 6: Approval API
- [ ] POST /api/approvals endpoint
- [ ] GET /api/approvals/:id endpoint
- [ ] WebSocket push new_approval event

## File Changes Summary

| File | Change |
|------|--------|
| `internal/shared/models/agent.go` | AgentTemplate add model config |
| `internal/shared/models/models.go` | AgentSession simplify, Task add TemplateID |
| `internal/coordinator/agent/task_decomposer.go` | Prompt + DecomposedTask |
| `internal/coordinator/agent/coordinator.go` | Load Template for system_prompt |
| `internal/coordinator/tools/registry.go` | read_file → local |
| `internal/worker/executor/tool_executor.go` | Permission check + approval polling |
| `cmd/worker/main.go` | CoordinatorClient approval methods |
| `internal/coordinator/handler/agent.go` | CreateSession API |
| `internal/coordinator/handler/approval.go` | New approval endpoints |
| `web/src/components/widgets/AgentChatWidget.tsx` | Advanced settings UI |