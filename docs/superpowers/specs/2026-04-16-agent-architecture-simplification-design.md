# Agent 架构简化设计

**日期**: 2026-04-16
**状态**: Draft

## 一、问题分析

### 当前架构问题

现有代码约 4000 行，分散在 4 个文件：

| 文件 | 行数 | 职责 |
|------|------|------|
| `function_calling.go` | 831 | LLM API 调用 + 工具执行 |
| `scheduler.go` | 472 | 远程工具队列管理 |
| `task_decomposer.go` | 569 | 任务拆解逻辑 |
| `coordinator.go` | 538 | 整合上述组件 |

**核心问题**：

1. **职责分散** - 工具执行逻辑分散在 Engine、Coordinator、Scheduler 三处
2. **抽象过度** - TaskDecomposer 只做一件事（调用 LLM），却单独成组件
3. **概念混乱** - Coordinator 到底是编排者还是执行者？

### 设计目标

简化为统一 Agent 结构，差异由模板定义，系统服务处理自动化流程。

---

## 二、核心概念

### Agent 统一结构

**Agent 的本质 = 调用模型 + 执行工具**

所有 Agent（包括 Coordinator）都遵循相同模式：
1. 收到消息（含上下文）
2. 调用模型决策
3. 调用工具执行

差异由模板定义：

| 模板字段 | 作用 |
|----------|------|
| `base_prompt` | 角色认知（"你是协调者..." vs "你是开发者..."） |
| `allowed_tools` | 工具权限（全局 vs 局部） |
| `default_model` | 模型配置 |

---

### Coordinator vs 执行者

差异不在代码结构，在于：

| | Coordinator | 执行者 |
|---|-------------|--------|
| **监听信道** | 用户 + 任务 + 消息（多个） | 自己的任务信道（单个） |
| **消息来源** | 用户 + 所有 Agent | Coordinator + 其他 Agent |
| **工具权限** | 全局操作（create_task, cancel_task） | 局部操作（read_file, write_file） |
| **运行模式** | 持续在线 | 任务完成后离线 |
| **上下文获取** | 消息注入 + 工具查询 | workspace 文件 + 任务描述 |

---

## 三、三层架构

### 架构图

```
┌─────────────────────────────────────────────┐
│              Agent 层                        │
│                                             │
│  Coordinator Agent         执行者 Agent      │
│  ├── 监听多信道             ├── 监听单信道    │
│  ├── 全局工具权限           ├── 局部工具权限  │
│  ├── 持续在线               ├── 完成后离线    │
│  └── 模型决策               └── 模型决策      │
│                                             │
│  核心逻辑统一：调用模型 → 决策 → 调用工具      │
└─────────────────────────────────────────────┘
                    ↓ 调用工具
┌─────────────────────────────────────────────┐
│            系统服务层                         │
│                                             │
│  TaskScheduler     - 依赖满足时自动调度       │
│  NodeAssigner      - 匹配能力 + 分配节点      │
│  MessageRouter     - 消息路由 + 恢复代理      │
│  ContextInjector   - 消息预处理 + 注入上下文  │
│  ProgressMonitor   - 监听状态 + 推送前端      │
│                                             │
│  特点：自动响应事件，固定规则，不调用模型      │
└─────────────────────────────────────────────┘
                    ↓ 操作数据
┌─────────────────────────────────────────────┐
│              数据层                          │
│                                             │
│  agents 表         - 任务 + Agent 统一       │
│  nodes 表          - 节点注册表              │
│  messages 表       - Agent 间消息            │
│  agent_templates 表 - 模板定义               │
│  workspace 目录    - 工作空间                │
└─────────────────────────────────────────────┘
```

---

### Agent 层职责

**统一 Agent 结构**：

```go
type Agent struct {
    ID           string
    Template     *AgentTemplate
    Messages     []Message      // 对话历史
    Workspace    string         // 工作目录
    
    // 核心：调用模型 + 执行工具
    ProcessMessage(msg Message) Response
}

func (a *Agent) ProcessMessage(msg Message) Response {
    // 1. 消息 + 上下文 加入对话历史
    a.Messages = append(a.Messages, msg)
    
    // 2. 调用模型
    response := a.CallLLM()
    
    // 3. 执行工具调用（如果有）
    for _, toolCall := range response.ToolCalls {
        result := a.ExecuteTool(toolCall)
        // 工具结果加入对话历史，继续调用模型
    }
    
    // 4. 返回响应
    return response
}
```

**Coordinator 特殊之处**：

1. **多信道监听** - 订阅 `user`、`tasks`、`messages` 三个信道
2. **全局工具权限** - 模板定义允许调用 `create_task`、`cancel_task` 等
3. **持续在线** - 不像执行者那样完成后离线

---

### 系统服务层职责

**不需要模型决策的自动化流程**：

| 服务 | 触发条件 | 做什么 |
|------|----------|--------|
| TaskScheduler | 任务完成 | 查依赖表 → 通知下游可开始 |
| NodeAssigner | 依赖满足 | 查节点能力 → 分配空闲节点 |
| MessageRouter | Agent 发消息 | 路由到目标（可能创建恢复代理） |
| ContextInjector | 消息到达 Coordinator | 查相关数据 → 注入上下文 |
| ProgressMonitor | 任务状态变更 | 推送到前端 WebSocket |

**这些服务不调用模型，只执行固定规则。**

---

## 四、消息流设计

### 消息类型

| 类型 | 用途 | 示例 |
|------|------|------|
| `user` | 用户对话 | "帮我实现登录功能" |
| `notify` | 状态通知 | "Agent-001 任务完成" |
| `question` | Agent 间问答 | "字段含义是什么？" |
| `request` | 服务请求 | "帮我创建修复任务" |

---

### 消息格式

```json
{
  "id": "msg-xxx",
  "type": "notify",
  "from": "Agent-001",
  "to": "Coordinator",
  "content": "任务完成",
  "context": {
    "task_id": "001",
    "output_files": ["api_spec.json"],
    "downstream_tasks": ["002", "003"]
  },
  "timestamp": "2026-04-16T10:00:00Z"
}
```

**`context` 字段由 ContextInjector 服务自动注入。**

---

### 消息流示例

```
Agent-001 完成
    ↓
ProgressMonitor 检测 → 更新状态 → 推送前端
    ↓
TaskScheduler 检测 → 查依赖 → 标记下游可开始
    ↓
NodeAssigner 检测 → 分配节点 → 启动 Agent-002
    ↓
ContextInjector 构建消息 → 注入上下文
    ↓
消息到达 Coordinator：{from: Agent-001, content: "完成", context: {...}}
    ↓
Coordinator 收到 → 放入对话历史 → 调用模型
    ↓
模型决策：调用 report_to_user 工具 → 汇报进度
```

---

## 五、工具设计

### Coordinator 工具

| 工具 | 用途 | 参数 | 返回 |
|------|------|------|------|
| `create_task` | 创建任务 | title, description, depends_on, template_id | task_id |
| `cancel_task` | 取消任务 | task_id | success/error |
| `query_tasks` | 查询任务状态 | filter（可选） | 任务列表 |
| `query_nodes` | 查询节点状态 | - | 节点列表 |
| `send_message` | 发消息给 Agent | to, content | success |
| `report_to_user` | 汇报给用户 | content | success |

---

### 执行者工具

| 工具 | 用途 | 执行位置 |
|------|------|----------|
| `read_file` | 读文件 | 本地（Coordinator 也可用） |
| `write_file` | 写文件 | 远程（Worker） |
| `execute_shell` | 执行命令 | 远程（Worker） |
| `test_code` | 测试代码 | 远程（Worker） |
| `send_message` | 发消息给其他 Agent | 本地（请求 Coordinator 转发） |

---

### 工具执行差异

| 工具类型 | 执行方式 | 调用模型？ |
|----------|----------|------------|
| 本地轻量工具 | Agent 直接执行 | 否 |
| 远程工具 | Agent 发送指令 → Worker 执行 → 返回结果 | 否 |
| Coordinator 全局工具 | Coordinator 调用 → 操作数据库 → 返回结果 | 否 |

---

## 六、上下文注入设计

### 为什么需要注入上下文？

模型决策需要知道：
- 任务完成时：哪个任务、产出物是什么、下游是谁
- Agent 请求时：请求者是谁、请求内容是什么

这些信息不在对话历史里，需要系统服务注入。

---

### 注入规则

| 消息类型 | 注入什么 |
|----------|----------|
| `notify: task_complete` | task_id, output_files, downstream_tasks |
| `notify: task_failed` | task_id, error_message |
| `request: create_task` | requester_task_id, related_files |
| `question` | from_agent, their_task_context |
| `user` | 不注入（用户消息不需要） |

---

### 主动查询场景

当模型需要更多数据（如"整体进度"），调用工具查询：

```
用户："进度怎么样？"
    ↓
模型调用 query_tasks 工具
    ↓
工具返回：[{id: "001", status: "done"}, {id: "002", status: "running"}]
    ↓
模型整理后回复用户
```

---

## 七、数据模型

### agents 表（任务 + Agent 统一）

```sql
CREATE TABLE agents (
    id VARCHAR(64) PRIMARY KEY,
    root_id VARCHAR(64),           -- 原始任务 ID
    parent_id VARCHAR(64),         -- 恢复代理的父 ID
    template_id VARCHAR(64),       -- 模板 ID
    status ENUM('pending', 'running', 'done', 'failed'),
    title VARCHAR(255),
    description TEXT,
    depends_on JSON,               -- 依赖的任务 ID 列表
    workspace_path VARCHAR(255),   -- 工作目录
    node_id VARCHAR(64),           -- 分配的节点
    progress INT,                  -- 进度百分比
    created_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);
```

---

### agent_templates 表

```sql
CREATE TABLE agent_templates (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255),
    description TEXT,
    base_prompt TEXT,              -- 角色提示词
    allowed_tools JSON,            -- 工具权限列表
    restricted_tools JSON,         -- 禁止工具列表
    default_model VARCHAR(64),     -- 默认模型
    required_capabilities JSON,    -- 需要的节点能力
    is_system BOOLEAN,
    created_at TIMESTAMP
);
```

---

### messages 表

```sql
CREATE TABLE messages (
    id VARCHAR(64) PRIMARY KEY,
    from_agent VARCHAR(64),
    proxy_for VARCHAR(64),         -- 代理的原始 Agent
    to_agent VARCHAR(64),
    type ENUM('user', 'notify', 'question', 'request'),
    content TEXT,
    context JSON,                  -- 注入的上下文
    status ENUM('pending', 'delivered', 'read'),
    created_at TIMESTAMP
);
```

---

## 八、代码结构简化

### 简化前（4000 行）

```
internal/coordinator/agent/
├── coordinator.go      (538行) - 对话流程 + 工具执行 + 任务拆解
├── function_calling.go (831行) - API调用 + 工具执行
├── scheduler.go        (472行) - 远程工具队列
├── task_decomposer.go  (569行) - 任务拆解
└── template.go         (177行) - 模板管理
```

---

### 简化后（约 1200 行）

```
internal/coordinator/agent/
├── agent.go            (~400行) - 统一 Agent 结构
│   ├── Agent 结构体
│   ├── ProcessMessage() - 核心：调用模型 + 执行工具
│   ├── ExecuteTool() - 工具执行（本地/远程）
│   ├── CallLLM() - 模型调用
│   └── LoadTemplate() - 加载模板
│
├── llm_client.go       (~300行) - API 调用细节
│   ├── OpenAI/Anthropic/GLM 请求构建
│   ├── Call() / Stream()
│   ├── ParseResponse()
│   └── BuildToolDefinitions()
│
└── template.go         (~100行) - 模板管理（保留）

internal/coordinator/service/
├── task_scheduler.go   (~150行) - 依赖满足时自动调度
├── node_assigner.go    (~100行) - 节点分配
├── message_router.go   (~100行) - 消息路由 + 恢复代理
├── context_injector.go (~80行)  - 上下文注入
└── progress_monitor.go (~70行)  - 进度推送
```

---

### 删除的组件

| 原组件 | 原职责 | 新归属 |
|--------|--------|--------|
| TaskDecomposer | 任务拆解 | 删除，拆解 = Agent 调用 create_task 工具 |
| ToolScheduler | 远程工具队列 | 删除，远程工具直接走 Worker 机制 |
| FunctionCallingEngine（部分） | API 调用 | 合并到 llm_client.go |
| FunctionCallingEngine（部分） | 工具执行 | 合并到 agent.go ExecuteTool() |

---

## 九、实现优先级

### Phase 1: Agent 统一结构
- 实现 `agent.go` 核心结构
- 实现 `llm_client.go` API 调用
- **注意**：Phase 1 完成后再删除 TaskDecomposer、ToolScheduler，确保功能不中断

### Phase 2: 系统服务
- 实现 TaskScheduler（依赖调度）
- 实现 NodeAssigner（节点分配）
- 实现 MessageRouter（消息路由）
- 实现 ContextInjector（上下文注入）

### Phase 3: Coordinator 特化
- 多信道监听
- 全局工具权限
- 持续在线模式

### Phase 4: 前端适配
- 进度展示适配新消息格式
- WebSocket 推送适配

---

## 十、关键决策总结

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Agent 统一结构 | ✅ | Coordinator 和执行者本质相同，差异由模板定义 |
| 拆解走工具流程 | ✅ | 拆解 = Agent 调用 create_task 工具，行为统一 |
| 系统服务层 | ✅ | 固定规则自动化，不调用模型 |
| 上下文注入 | ✅ | 模型决策需要上下文，由系统服务注入 |
| 删除 TaskDecomposer | ✅ | 拆解不再需要单独组件 |
| 删除 ToolScheduler | ✅ | 远程工具直接走现有 Worker 机制 |