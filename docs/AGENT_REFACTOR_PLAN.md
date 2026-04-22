# Cowork Agent 体系重构设计文档

> **Status: COMPLETED (2026-04-16)**
>
> This plan has been superseded by the **Agent Architecture Simplification** project.
> See `docs/superpowers/specs/2026-04-16-agent-architecture-simplification-design.md` for the current architecture.
>
> Key changes from this original plan:
> - TaskDecomposer deleted → Agent uses create_task tool for decomposition
> - ToolScheduler deleted → Direct Worker mechanism
> - FunctionCallingEngine deleted → Merged into Agent + LLMClient
> - Unified Agent structure for Coordinator and Workers
> - System services layer for automation without model calls

## 1. 概述

### 1.1 目标

将 Cowork 从简单的分布式任务系统升级为具备 AI Agent 能力的智能任务处理平台：

1. **完整的 Agent 体系**
   - Coordinator 具备工具调用(Function Calling)能力
   - Worker 能够执行工具并返回结果
   - 建立可扩展的工具定义和注册机制

2. **任务拆解能力**
   - 用户通过自然语言与系统交互
   - 协调者自动将对话拆解为结构化任务
   - 任务自动分发给合适的 Worker 执行

### 1.2 当前架构限制

| 问题 | 描述 |
|------|------|
| 无 Function Calling | Agent 仅支持简单对话，无法调用工具 |
| 无任务拆解 | 用户请求不能自动分解为子任务 |
| 任务类型有限 | 仅支持 shell/script 执行，无工具执行机制 |
| 无工具注册 | 缺少工具定义和注册机制 |

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (React)                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │ Agent Chat  │  │ Task List   │  │ Tool Management UI      │ │
│  │   Widget    │  │   Widget    │  │    (Future)             │ │
│  └──────┬──────┘  └──────┬──────┘  └─────────────────────────┘ │
└─────────┼────────────────┼──────────────────────────────────────┘
          │                │
          ▼                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Coordinator (Go)                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Agent Core                                ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ ││
│  │  │   Chat      │  │  Function   │  │  Task               │ ││
│  │  │  Engine     │──│  Calling    │──│  Decomposer         │ ││
│  │  │  (LLM API)  │  │  Engine     │  │                     │ ││
│  │  └─────────────┘  └──────┬──────┘  └──────────┬──────────┘ ││
│  │                          │                     │            ││
│  │  ┌───────────────────────┴─────────────────────┴─────────┐ ││
│  │  │                    Tool Registry                       │ ││
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────┐  │ ││
│  │  │  │ Execute │ │  File   │ │  Web    │ │  Project    │  │ ││
│  │  │  │ Shell   │ │  Ops    │ │ Search  │ │  Analysis   │  │ ││
│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────────┘  │ ││
│  │  └───────────────────────────────────────────────────────┘ ││
│  └─────────────────────────────────────────────────────────────┘│
│                           │                                      │
│  ┌────────────────────────┴────────────────────────────────────┐│
│  │                    Task Scheduler                            ││
│  │        (Priority-based, Tag-matching Distribution)          ││
│  └─────────────────────────────────────────────────────────────┘│
└───────────────────────────┬─────────────────────────────────────┘
                            │
            ┌───────────────┼───────────────┐
            ▼               ▼               ▼
     ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
     │   Worker 1   │ │   Worker 2   │ │   Worker N   │
     │  (Shell,     │ │  (Docker,    │ │  (GPU,       │
     │   Script)    │ │   Python)    │ │   ML)        │
     └──────────────┘ └──────────────┘ └──────────────┘
```

### 2.2 核心组件设计

#### 2.2.1 Agent Core (协调者核心)

**职责**：
- 管理 LLM 对话上下文
- 处理 Function Calling 流程
- 任务拆解与编排
- 工具调用决策

**核心流程**：

```
用户输入 → LLM 推理 → 判断是否需要工具调用
                         │
              ┌──────────┴──────────┐
              ▼                     ▼
         需要工具调用           直接回复用户
              │
              ▼
      生成 Tool Call 请求
              │
              ▼
      执行工具 (创建 Task)
              │
              ▼
      收集工具执行结果
              │
              ▼
      将结果反馈给 LLM
              │
              ▼
      生成最终回复
```

#### 2.2.2 Tool Registry (工具注册中心)

**职责**：
- 管理可用工具定义
- 提供工具 Schema (OpenAI Compatible)
- 工具权限控制
- 工具路由 (本地执行 vs Worker 执行)

**工具分类**：

| 类别 | 工具示例 | 执行位置 |
|------|----------|----------|
| 系统工具 | execute_shell, read_file, write_file | Worker |
| 信息工具 | web_search, fetch_url | Worker/Coordinator |
| 任务工具 | create_task, query_task, cancel_task | Coordinator |
| 项目工具 | analyze_project, generate_code | Worker |
| 审批工具 | request_approval (Human-in-loop) | Coordinator |

#### 2.2.3 Task Decomposer (任务拆解器)

**职责**：
- 将复杂请求拆解为原子任务
- 分析任务依赖关系
- 生成执行计划

**拆解策略**：

```json
{
  "user_request": "帮我完成 XX 项目的用户认证功能",
  "decomposed_tasks": [
    {
      "id": "task-1",
      "type": "analyze",
      "description": "分析项目结构和现有认证机制",
      "dependencies": []
    },
    {
      "id": "task-2",
      "type": "implement",
      "description": "实现 JWT 认证中间件",
      "dependencies": ["task-1"]
    },
    {
      "id": "task-3",
      "type": "test",
      "description": "编写单元测试",
      "dependencies": ["task-2"]
    }
  ]
}
```

---

## 3. Function Calling 流程设计

### 3.1 工具定义规范 (OpenAI Compatible)

```go
// Tool 工具定义
type Tool struct {
    Type     string      `json:"type"`              // "function"
    Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// ToolCall 工具调用请求
type ToolCall struct {
    ID       string     `json:"id"`
    Type     string     `json:"type"`
    Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"` // JSON string
}

// ToolResult 工具执行结果
type ToolResult struct {
    ToolCallID string `json:"tool_call_id"`
    Output     string `json:"output"`
    IsError    bool   `json:"is_error"`
}
```

### 3.2 内置工具定义

```go
// 系统工具定义
var BuiltinTools = []Tool{
    {
        Type: "function",
        Function: FunctionDef{
            Name:        "execute_shell",
            Description: "在隔离环境中执行 Shell 命令。用于文件操作、系统命令等。注意：危险命令会被阻止。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "command": map[string]interface{}{
                        "type":        "string",
                        "description": "要执行的 Shell 命令",
                    },
                    "work_dir": map[string]interface{}{
                        "type":        "string",
                        "description": "工作目录（可选）",
                    },
                    "timeout": map[string]interface{}{
                        "type":        "integer",
                        "description": "超时时间（秒），默认 300",
                    },
                },
                "required": []string{"command"},
            },
        },
    },
    {
        Type: "function",
        Function: FunctionDef{
            Name:        "read_file",
            Description: "读取文件内容。支持文本文件，返回文件内容字符串。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "文件路径",
                    },
                },
                "required": []string{"path"},
            },
        },
    },
    {
        Type: "function",
        Function: FunctionDef{
            Name:        "write_file",
            Description: "写入文件内容。如果文件不存在则创建，存在则覆盖。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "文件路径",
                    },
                    "content": map[string]interface{}{
                        "type":        "string",
                        "description": "文件内容",
                    },
                },
                "required": []string{"path", "content"},
            },
        },
    },
    {
        Type: "function",
        Function: FunctionDef{
            Name:        "create_task",
            Description: "创建一个新的任务并分配给 Worker 执行。用于异步执行耗时操作。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "type": map[string]interface{}{
                        "type":        "string",
                        "description": "任务类型：shell, script, docker",
                    },
                    "description": map[string]interface{}{
                        "type":        "string",
                        "description": "任务描述",
                    },
                    "input": map[string]interface{}{
                        "type":        "object",
                        "description": "任务输入参数",
                    },
                    "priority": map[string]interface{}{
                        "type":        "string",
                        "enum":        []string{"low", "medium", "high"},
                        "description": "优先级",
                    },
                    "required_tags": map[string]interface{}{
                        "type":        "array",
                        "items":       map[string]string{"type": "string"},
                        "description": "所需 Worker 标签",
                    },
                },
                "required": []string{"type", "description"},
            },
        },
    },
    {
        Type: "function",
        Function: FunctionDef{
            Name:        "query_task",
            Description: "查询任务状态和结果。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "task_id": map[string]interface{}{
                        "type":        "string",
                        "description": "任务 ID",
                    },
                },
                "required": []string{"task_id"},
            },
        },
    },
    {
        Type: "function",
        Function: FunctionDef{
            Name:        "request_approval",
            Description: "请求用户审批。用于需要用户确认的敏感操作。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "action": map[string]interface{}{
                        "type":        "string",
                        "description": "需要审批的操作描述",
                    },
                    "details": map[string]interface{}{
                        "type":        "string",
                        "description": "操作详情",
                    },
                },
                "required": []string{"action"},
            },
        },
    },
}
```

### 3.3 Function Calling 执行流程

```
┌──────────────────────────────────────────────────────────────────┐
│                      Coordinator Agent                            │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  1. 用户消息                                                      │
│       │                                                           │
│       ▼                                                           │
│  ┌─────────────────────────────────────────┐                     │
│  │ 构建 LLM 请求                            │                     │
│  │ - System Prompt (Agent 角色)             │                     │
│  │ - Conversation History                   │                     │
│  │ - Available Tools                       │                     │
│  └────────────────┬────────────────────────┘                     │
│                   │                                               │
│                   ▼                                               │
│  ┌─────────────────────────────────────────┐                     │
│  │ 调用 LLM API                             │                     │
│  │ (OpenAI/Anthropic/GLM)                   │                     │
│  └────────────────┬────────────────────────┘                     │
│                   │                                               │
│         ┌─────────┴─────────┐                                     │
│         ▼                   ▼                                     │
│  [有 Tool Calls]      [无 Tool Calls]                             │
│         │                   │                                     │
│         ▼                   ▼                                     │
│  ┌─────────────────┐   返回文本响应                                │
│  │ 处理每个 Tool   │                                               │
│  │ Call           │                                               │
│  └────────┬────────┘                                               │
│           │                                                        │
│           ▼                                                        │
│  ┌─────────────────────────────────────────┐                      │
│  │ 判断工具类型:                            │                      │
│  │ - 本地工具 (create_task, query_task)    │                      │
│  │   → 直接执行                            │                      │
│  │ - 远程工具 (execute_shell, read_file)   │                      │
│  │   → 创建 Task 分发给 Worker             │                      │
│  └────────────────┬────────────────────────┘                      │
│                   │                                               │
│                   ▼                                               │
│  ┌─────────────────────────────────────────┐                      │
│  │ 收集工具执行结果                         │                      │
│  │ 构建 Tool Result 消息                   │                      │
│  └────────────────┬────────────────────────┘                      │
│                   │                                               │
│                   ▼                                               │
│  ┌─────────────────────────────────────────┐                      │
│  │ 再次调用 LLM                             │                      │
│  │ (带上 Tool Results)                      │                      │
│  └────────────────┬────────────────────────┘                      │
│                   │                                               │
│                   ▼                                               │
│            生成最终响应                                           │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## 4. 数据模型变更

### 4.1 新增模型

```go
// AgentConversation Agent 对话 (替代原 AgentSession)
type AgentConversation struct {
    ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
    Title       string    `gorm:"type:varchar(255)" json:"title"`
    Model       string    `gorm:"type:varchar(50)" json:"model"`
    Status      string    `gorm:"type:varchar(20);default:'active'" json:"status"` // active, waiting_tool, completed

    // 上下文管理
    SystemPrompt string    `gorm:"type:text" json:"system_prompt"`
    ContextSummary string  `gorm:"type:text" json:"context_summary"` // 长对话摘要

    // 配置
    Config      JSON      `gorm:"type:text" json:"config"`

    // 关联任务
    ActiveTaskID *string  `gorm:"type:varchar(64)" json:"active_task_id"`
    ActiveTask   *Task    `gorm:"foreignKey:ActiveTaskID" json:"active_task,omitempty"`

    CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`

    Messages    []AgentMessage `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
}

// AgentMessage 扩展 (增加 Tool Call 支持)
type AgentMessage struct {
    ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
    ConversationID string `gorm:"type:varchar(64);index;not null" json:"conversation_id"`

    Role      string `gorm:"type:varchar(20);not null" json:"role"` // user, assistant, system, tool
    Content   string `gorm:"type:text" json:"content"`

    // Tool Call 支持
    ToolCalls  JSON `gorm:"type:text" json:"tool_calls,omitempty"`  // []ToolCall
    ToolCallID string `gorm:"type:varchar(64)" json:"tool_call_id,omitempty"` // 当 role=tool 时

    Tokens     int       `json:"tokens"`
    CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
    ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
    Name        string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
    Description string    `gorm:"type:text" json:"description"`
    Parameters  JSON      `gorm:"type:text" json:"parameters"` // JSON Schema

    // 分类与权限
    Category    string    `gorm:"type:varchar(50);index" json:"category"` // system, file, web, task, custom
    ExecuteMode string    `gorm:"type:varchar(20)" json:"execute_mode"`   // local, remote
    Permission  string    `gorm:"type:varchar(20)" json:"permission"`     // read, write, execute, admin

    // 实现信息
    Handler     string    `gorm:"type:varchar(255)" json:"handler"` // 处理函数名或 API endpoint

    IsEnabled   bool      `gorm:"default:true" json:"is_enabled"`
    IsBuiltin   bool      `gorm:"default:false" json:"is_builtin"`

    CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ToolExecution 工具执行记录
type ToolExecution struct {
    ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    ConversationID string    `gorm:"type:varchar(64);index" json:"conversation_id"`
    MessageID      uint      `gorm:"index" json:"message_id"`
    TaskID         *string   `gorm:"type:varchar(64);index" json:"task_id"`

    ToolName       string    `gorm:"type:varchar(100);index" json:"tool_name"`
    ToolCallID     string    `gorm:"type:varchar(64)" json:"tool_call_id"`
    Arguments      JSON      `gorm:"type:text" json:"arguments"`

    Status         string    `gorm:"type:varchar(20);index" json:"status"` // pending, running, completed, failed
    Result         string    `gorm:"type:text" json:"result"`
    IsError        bool      `gorm:"default:false" json:"is_error"`

    StartedAt      *time.Time `json:"started_at"`
    CompletedAt    *time.Time `json:"completed_at"`
    CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

// Task 扩展 (增加工具执行相关字段)
type Task struct {
    // ... 原有字段 ...

    // 新增: 工具执行相关
    ToolCallID     *string `gorm:"type:varchar(64)" json:"tool_call_id"`
    ConversationID *string `gorm:"type:varchar(64);index" json:"conversation_id"`
    ToolName       string  `gorm:"type:varchar(100)" json:"tool_name"`
}
```

### 4.2 数据库迁移

```sql
-- 重命名表 (兼容旧数据)
ALTER TABLE agent_sessions RENAME TO agent_conversations;

-- 修改 messages 表
ALTER TABLE agent_messages ADD COLUMN conversation_id VARCHAR(64);
ALTER TABLE agent_messages ADD COLUMN tool_calls TEXT;
ALTER TABLE agent_messages ADD COLUMN tool_call_id VARCHAR(64);
UPDATE agent_messages SET conversation_id = session_id WHERE conversation_id IS NULL;

-- 新增工具定义表
CREATE TABLE tool_definitions (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    parameters TEXT,
    category VARCHAR(50),
    execute_mode VARCHAR(20) DEFAULT 'remote',
    permission VARCHAR(20) DEFAULT 'read',
    handler VARCHAR(255),
    is_enabled BOOLEAN DEFAULT TRUE,
    is_builtin BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 新增工具执行记录表
CREATE TABLE tool_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id VARCHAR(64),
    message_id INTEGER,
    task_id VARCHAR(64),
    tool_name VARCHAR(100),
    tool_call_id VARCHAR(64),
    arguments TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    result TEXT,
    is_error BOOLEAN DEFAULT FALSE,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 任务表扩展
ALTER TABLE tasks ADD COLUMN tool_call_id VARCHAR(64);
ALTER TABLE tasks ADD COLUMN conversation_id VARCHAR(64);
ALTER TABLE tasks ADD COLUMN tool_name VARCHAR(100);

-- 索引
CREATE INDEX idx_agent_messages_conversation ON agent_messages(conversation_id);
CREATE INDEX idx_tool_executions_conversation ON tool_executions(conversation_id);
CREATE INDEX idx_tasks_conversation ON tasks(conversation_id);
```

---

## 5. API 变更

### 5.1 新增接口

#### 5.1.1 Agent Conversation API

```
# 创建对话 (支持工具调用)
POST /api/agent/conversations
{
    "model": "openai",
    "system_prompt": "You are a helpful assistant...",
    "tools": ["execute_shell", "read_file", "write_file"] // 可选，指定可用工具
}

# 发送消息 (支持 Function Calling 流程)
POST /api/agent/conversations/:id/messages
{
    "content": "帮我查看当前目录结构",
    "auto_execute_tools": true  // 是否自动执行工具
}

# 流式响应 (SSE)
GET /api/agent/conversations/:id/stream

# 执行工具调用 (手动模式)
POST /api/agent/conversations/:id/tools/execute
{
    "tool_call_id": "call_xxx",
    "approved": true  // Human-in-loop 审批
}

# 获取对话状态
GET /api/agent/conversations/:id/status
{
    "status": "waiting_tool",
    "pending_tool_calls": [...]
}
```

#### 5.1.2 Tools API

```
# 获取可用工具列表
GET /api/tools
Response: {
    "tools": [
        {
            "name": "execute_shell",
            "description": "...",
            "parameters": {...},
            "category": "system"
        }
    ]
}

# 获取工具详情
GET /api/tools/:name

# 注册自定义工具 (管理员)
POST /api/tools
{
    "name": "custom_analyzer",
    "description": "...",
    "parameters": {...},
    "handler": "http://worker:8081/tools/custom_analyzer",
    "execute_mode": "remote"
}

# 更新工具定义
PUT /api/tools/:name

# 删除自定义工具
DELETE /api/tools/:name
```

#### 5.1.3 Task API 扩展

```
# 获取对话关联的任务
GET /api/agent/conversations/:id/tasks

# 任务审批 (Human-in-loop)
POST /api/tasks/:id/approve
{
    "approved": true,
    "comment": "同意执行"
}
```

### 5.2 修改接口

#### 5.2.1 Agent Session → Conversation

```
# 旧接口 (保留兼容)
POST /api/agent/sessions
GET /api/agent/sessions/:id

# 新接口
POST /api/agent/conversations
GET /api/agent/conversations/:id
```

#### 5.2.2 Message 结构扩展

```
# 发送消息响应 (支持 Tool Call)
{
    "id": 123,
    "role": "assistant",
    "content": "",
    "tool_calls": [
        {
            "id": "call_xxx",
            "type": "function",
            "function": {
                "name": "execute_shell",
                "arguments": "{\"command\": \"ls -la\"}"
            }
        }
    ]
}

# Tool Result 消息
{
    "id": 124,
    "role": "tool",
    "content": "total 24\ndrwxr-xr-x 5 user user 4096 ...",
    "tool_call_id": "call_xxx"
}
```

---

## 6. 前端变更

### 6.1 Agent Store 扩展

```typescript
interface AgentState {
    // 现有字段 ...

    // 新增
    pendingToolCalls: ToolCall[]
    toolExecutions: ToolExecution[]

    // 新增 Actions
    sendMessageWithTools: (content: string, autoExecute?: boolean) => Promise<void>
    approveToolCall: (toolCallId: string, approved: boolean) => Promise<void>
    getToolStatus: (toolCallId: string) => Promise<ToolExecution>
}
```

### 6.2 UI 组件

1. **Tool Call 显示组件**: 展示工具调用状态、参数、结果
2. **审批弹窗**: Human-in-loop 审批交互
3. **任务进度展示**: 与对话关联的任务实时进度

---

## 7. 分阶段实施计划

### Phase 1: 工具系统基础 (预计 3 天) ✅ 已完成

**目标**: 建立工具定义和注册机制

**任务**:
1. 实现 `ToolDefinition` 模型和数据库迁移
2. 实现 `ToolRegistry` 工具注册中心
3. 定义内置工具 (execute_shell, read_file, write_file)
4. 实现 `GET /api/tools` 接口
5. 编写单元测试

**验收标准**:
- [x] 工具定义可正确存储和查询
- [x] 内置工具定义符合 OpenAI 规范
- [x] API 接口正常工作

### Phase 2: Function Calling 引擎 (预计 5 天) ✅ 已完成

**目标**: 实现完整的 Function Calling 流程

**任务**:
1. 扩展 `AgentConversation` 和 `AgentMessage` 模型
2. 实现 `ToolExecution` 模型
3. 实现 Function Calling 请求构建 (含 Tools 参数)
4. 解析 LLM 响应中的 Tool Calls
5. 实现工具执行调度 (本地 vs 远程)
6. 实现工具结果反馈和后续对话
7. 支持多轮 Tool Calling

**验收标准**:
- [x] LLM 能正确返回 Tool Calls
- [x] 工具能被正确执行
- [x] 结果能正确反馈给 LLM
- [x] 多轮 Tool Calling 正常工作

### Phase 3: Worker 工具执行 (预计 3 天) ✅ 已完成

**目标**: Worker 能执行工具调用任务

**任务**:
1. 扩展 Task 模型支持工具调用
2. 实现 Worker 端工具执行器
3. 工具执行结果格式化
4. 错误处理和重试机制
5. 安全检查 (危险命令阻止)

**验收标准**:
- [x] Worker 能接收并执行工具调用任务
- [x] 执行结果能正确返回
- [x] 安全检查生效

### Phase 4: 前端集成 (预计 3 天) ✅ 已完成

**目标**: 前端支持 Function Calling 交互

**任务**:
1. 扩展 `agentStore` 支持工具调用状态
2. 实现 Tool Call 显示组件
3. 实现流式响应中 Tool Call 的处理
4. 实现 Human-in-loop 审批 UI
5. 任务进度实时展示

**验收标准**:
- [x] 用户能看到工具调用状态
- [x] 审批流程正常工作
- [x] 任务进度实时更新

### Phase 5: 任务拆解能力 (预计 4 天) ✅ 已完成

**目标**: 实现自动任务拆解

**任务**:
1. 设计任务拆解 Prompt
2. 实现拆解结果解析
3. 实现任务依赖管理
4. 实现任务创建和分发
5. 实现整体进度追踪

**验收标准**:
- [x] 复杂请求能被正确拆解
- [x] 任务依赖关系正确
- [x] 执行顺序符合预期

### Phase 6: 测试与优化 (预计 2 天) ✅ 已完成

**目标**: 系统稳定性和性能优化

**任务**:
1. 端到端测试
2. 错误处理完善
3. 性能优化 (并发、缓存)
4. 文档更新

**验收标准**:
- [x] 核心流程测试覆盖
- [x] 错误处理完善
- [x] 性能满足预期

---

## 8. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| LLM 不稳定返回 Tool Call | 功能异常 | 添加重试机制，Fallback 到文本提示 |
| 工具执行超时 | 用户体验差 | 设置合理超时，异步通知结果 |
| 危险命令执行 | 安全风险 | 多层安全检查，审批机制 |
| 上下文过长 | API 成本高 | 实现对话摘要，限制历史长度 |
| Worker 不可用 | 任务失败 | 超时重试，任务重新分配 |

---

## 9. 后续扩展方向

1. **更多内置工具**: 数据库操作、API 调用、代码执行等
2. **自定义工具**: 支持用户注册自定义工具
3. **工具链编排**: 多工具组合执行
4. **记忆系统**: 长期记忆、知识库检索
5. **多 Agent 协作**: 专业 Agent 分工协作

---

## 10. 参考资料

- [OpenAI Function Calling](https://platform.openai.com/docs/guides/function-calling)
- [Anthropic Tool Use](https://docs.anthropic.com/claude/docs/tool-use)
- [LangChain Tools](https://python.langchain.com/docs/modules/tools/)
- [AutoGPT Architecture](https://docs.auto-gpt.com/)