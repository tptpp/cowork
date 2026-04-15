# Cowork 任务协作平台重构设计

## 一、项目背景

### 原始诉求

工作中需要处理大量任务，希望交给 AI 并行处理。AI 拿到任务后可能继续拆解为子任务。为避免各任务/子任务并行运行时相互干扰，需要隔离机制（部署到不同节点）。但任务之间可能需要相互配合（开发节点、测试节点），需要通信机制。

### 现有系统问题

- 底层无法实现任务间通信
- 界面操作成本高
- 用户无法感知任务进度
- 用户无法感知子任务依赖关系
- Agent 与 Task 系统割裂

---

## 二、整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      用户界面层                               │
│  - 对话发起任务                                               │
│  - 任务树可视化（依赖关系、进度）                              │
│  - 用户介入（监控、审批、对话、控制）                          │
└─────────────────────────────────────────────────────────────┘
                          ↓ 对话
┌─────────────────────────────────────────────────────────────┐
│                  Coordinator Agent                           │
│  - 接收用户意图，拆解任务                                      │
│  - 调度节点，分配任务                                         │
│  - 监控全局进度                                               │
│  - 处理完成后上浮消息                                         │
│  - 是 Agent，与其他 Agent 通信协议一致                        │
└─────────────────────────────────────────────────────────────┘
                          ↓ 分配任务
┌─────────────────────────────────────────────────────────────┐
│                    节点资源池                                 │
│                                                              │
│  N1: {browser, docker}    N2: {gpu}      N3: {browser}       │
│  (沙箱)                    (物理机)        (云服务器)          │
│                                                              │
│  每个节点上运行独立 Agent 实例                                 │
│  - 接收任务，自主执行                                         │
│  - 与其他 Agent 通信                                          │
│  - 汇报进度                                                   │
│                                                              │
│  节点所有权转移：任务完成后释放，workspace 清空                 │
│  共享挂载：静态文件持久化                                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 三、核心概念

### 3.1 Agent ID = Task ID

Agent ID 与 Task ID 使用同一标识符。

```
Task-001 的 Agent 实例：
├── Agent ID: Agent-001（即 Task-001）
├── Agent = Agent模板 + 任务上下文实例化
├── 工作目录: workspace/001/
```

### 3.2 Agent 模板

Agent 模板定义 Agent 的能力。

```
模板结构：
├── template_id: "dev-template"
├── prompts: {base_prompt, system_prompt}
├── tools: {allowed, restricted}
├── capabilities: 需要的节点能力
├── approval_level: 默认审批级别

系统预设模板：
├── CoordinatorTemplate - 拆解、调度、监控
├── DevTemplate - 代码实现
├── TestTemplate - 测试验证
├── ReviewTemplate - 代码审查
├── DeployTemplate - 部署发布
├── ResearchTemplate - 调研分析

用户可自定义模板，可基于系统模板继承 + 覆盖。
```

### 3.3 继承关系

恢复代理通过 root_id 和 parent_id 维护继承链。

```
数据模型：
├── root_id: 原始任务 ID（自己时等于 id）
├── parent_id: 父 Agent ID（恢复代理时指向上一条）

示例：
Agent-001: root_id=001, parent_id=NULL    # 原始任务
Agent-006: root_id=001, parent_id=001      # 第一次恢复
Agent-007: root_id=001, parent_id=006      # 第二次恢复
Agent-008: root_id=001, parent_id=007      # 第三次恢复

查询继承链：SQL 递归查询 parent_id 链。
```

---

## 四、任务依赖管理

### 4.1 依赖类型

混合方案：静态依赖 + 数据流依赖。

```
静态依赖：解决"什么时候能开始"
├── depends_on: ["Task-002", "Task-003"]
├── Coordinator 计算调度顺序

数据流依赖：解决"需要什么输入"
├── requires: ["Task-001/output/api_spec.json"]
├── Agent 启动时加载必要文件
```

### 4.2 动态依赖

修复-测试循环等场景会动态创建新任务，依赖关系动态增长。

```
Round 1:
├── Task-004 测试完成 → 发现问题
├── 创建 Task-005 修复
├── Task-005 完成后 → 创建 Task-006 再次测试

每次循环创建新任务 ID，历史清晰可追溯。
```

---

## 五、Agent 间通信

### 5.1 通信场景

| 场景 | 实现 |
|------|------|
| 协调通知 | 依赖完成 → Coordinator 自动通知下游 Agent 可开始 |
| 数据传递 | 共享挂载 + 数据流依赖路径，Agent 启动时加载 |
| 对话协商 | Agent 发消息 → 消息路由 → 目标 Agent |

### 5.2 消息格式

```json
{
  "message_id": "msg-xxx",
  "from": "Agent-002",
  "proxy_for": "Agent-002",    // 原始任务时 proxy_for = from
  "to": "Agent-001",
  "type": "question",          // notify/question/data/request_approval
  "content": "API文档里这个字段含义是什么？",
  "timestamp": "2026-04-15T10:32:00Z",
  "requires_response": true
}
```

### 5.3 路由规则

发送方只指定原始目标 ID，Coordinator 维护继承链并路由。

```
消息 {to: "Agent-001"}
              ↓
Coordinator：
├── 查注册表继承链
├── 始终创建新 Agent（不判断是否有存活）
├── 新 Agent 继承最新链末端
├── 加载历史数据恢复现场

恢复代理回复：
├── {from: "Agent-006", proxy_for: "Agent-001", to: "Agent-004"}
├── 声明代理身份，对话身份稳定
```

---

## 六、Coordinator Agent

### 6.1 角色

Coordinator 也是 Agent，与其他 Agent 通信协议一致。

```
Coordinator Agent 特殊能力：
├── 提示词：全局视角、拆解能力、调度决策
├── 工具：dispatch_task、assign_node、monitor_progress、create_agent...
├── 特权：能看到所有任务状态、能创建/取消任务
├── 消息订阅：监听所有任务信道（监控全局）

职责：
├── 接收用户意图，拆解任务
├── 展示拆解方案，用户确认后执行
├── 调度节点，分配任务
├── 监控全局进度
├── 处理任务完成后上浮消息
├── 决定是否创建恢复任务或直接回复
```

### 6.2 任务拆解

拆解到 Agent 可执行级（单个 Agent 能独立完成的任务单元）。

```
拆解流程：
├── Coordinator 调用 AI 分析用户意图
├── 生成任务树（含依赖关系）
├── 展示给用户确认（默认需确认，可选信任模式）
├── 确认后开始调度执行
```

---

## 七、节点资源池

### 7.1 节点模型

节点按能力标记，而非角色。

```
节点类型：
├── sandbox（沙箱）
├── docker（容器）
├── physical（物理机）
├── cloud（云服务器）

能力标签：
├── {browser, docker, gpu, large_mem...}

节点注册表：
├── Node ID
├── 类型
├── 能力标签
├── 状态：idle/busy/offline
├── 当前处理的 Agent ID
├── 通信地址
```

### 7.2 节点生命周期

```
N1 处理 Agent-002：
├── 接收指令 → workspace/002/ 创建
├── Agent-002 启动（加载模板 + 上下文）
├── 执行任务 → 调用工具、汇报进度
├── 任务完成 → workspace/002/ 持久化到共享存储
├── 节点清理 → 调用清理钩子（基础设施决定是否清空）
├── 节点释放 → N1.status = idle

任务完成后节点立即释放，所有权转移。
前后任务文件路径隔离（workspace/task-A/, workspace/task-B/）。
静态文件通过共享挂载持久化。
```

### 7.3 调度策略

默认策略：依赖优先。

```
优先调度依赖链上游的任务：
├── Task-001（无依赖）→ 最先调度
├── Task-002、003（依赖 001）→ 等待 001 完成后调度
├── Task-004（依赖 002、003）→ 等待 002、003 都完成后调度

调度流程：
├── 查询任务需求能力
├── 查询可用节点（能力匹配）
├── 选择最优节点（idle 状态）
├── 发送任务指令
├── 更新节点状态
```

---

## 八、进度感知

### 8.1 进度展示

```
Agent-002 进度：
├── 里程碑: [设计(✓) → 实现(●) → 测试(○) → 完成(○)]
│           ✓ = 完成    ● = 进行中    ○ = 未开始
│
├── Agent 最新汇报: "正在实现表单验证逻辑"
│
└── 最近工具调用推断

整体任务树：
登录功能 (Root)
├── Agent-001: done ✓
├── Agent-002: 60% ●  [设计✓ → 实现● → 测试○ → 完成○]
├── Agent-003: 40% ●  [设计✓ → 实现● → 测试○ → 完成○]
├── Agent-004: pending ○  (等待 002, 003)
```

### 8.2 进度汇报机制

| 来源 | 内容 |
|------|------|
| Agent 主动汇报 | "正在做 X"、"完成了 Y"、"遇到问题 Z" |
| 工具调用推断 | Coordinator 监听工具调用，推断当前阶段 |
| 里程碑到达 | Agent 标记里程碑完成，系统更新进度条 |

---

## 九、用户介入

### 9.1 四种介入能力

| 能力 | 入口 |
|------|------|
| 监控 | 任务树视图，实时更新状态、进度、里程碑 |
| 审批 | 高风险操作弹出审批请求，用户批准/拒绝 |
| 对话介入 | 点击任意 Agent 进入对话，发送指令或反馈 |
| 完整控制 | 暂停任务、取消任务、修改依赖、重新分配节点 |

### 9.2 审批机制

分级审批 + 用户可修改策略。

```
风险等级：
├── 低风险（自动执行）：read_file, write_file(非关键), run_local_test, browse_web
├── 中风险（自动审批带超时）：execute_shell(非破坏性), edit_shared_file, create_branch
│   超时：60秒，超时后自动批准
├── 高风险（强制人工审批）：delete_file, push_git, execute_shell(rm/sudo), deploy_production

用户可自定义审批规则：
├── "所有 git 操作需人工审批"
├── "测试环境自动批准所有操作"
├── "信任某 Agent，全部自动执行"
```

---

## 十、恢复机制

### 10.1 恢复流程

```
Agent-001 已完成，收到消息：
├── 消息 {to: "Agent-001", content: "字段含义？"}
│              ↓
├── Coordinator 查继承链
├── 创建 Agent-006
│   ├── root_id = Agent-001
│   ├── parent_id = Agent-001
│   ├── 加载 workspace/001/ 历史数据
│              ↓
├── Agent-006 启动
│   ├── 工作目录：workspace/006/（写入自己的记录）
│   ├── 参考目录：workspace/001/（只读，加载历史）
│   ├── 回答问题 → 发消息给原发送方
│   ├── 完成，Agent-006 离线
│              ↓
├── Agent-006 回复：{from: Agent-006, proxy_for: Agent-001}
```

### 10.2 历史数据内容

```
workspace/{Agent-ID}/
├── conversations/        # 对话历史
│   ├── main.json         # 与 Coordinator 的主对话
│   ├── peer.json         # 与其他 Agent 的对话
│   └── user.json         # 与用户的对话（如有介入）
│
├── artifacts/            # 产出文件
├── decisions/            # 决策记录
├── tool_calls.json       # 工具调用历史
├── milestone.json        # 里程碑完成记录
└── summary.json          # 任务总结（完成时生成）
```

---

## 十一、数据模型

### 11.1 agents 表（统一任务/Agent）

```
┌─────────────────────────────────────────────────────────────┐
│  id          VARCHAR    主键                      │
│  root_id     VARCHAR    原始任务 ID（自己时等于 id）            │
│  parent_id   VARCHAR    父 Agent ID（恢复代理时指向上一条）     │
│  template_id VARCHAR    Agent 模板 ID                         │
│  status      ENUM       pending/running/done/failed          │
│  title       VARCHAR    任务标题                              │
│  description TEXT       任务描述                              │
│  depends_on  JSON       依赖的任务 ID 列表                     │
│  requires    JSON       需要的数据流路径列表                   │
│  workspace_path VARCHAR  工作目录路径                          │
│  node_id     VARCHAR    当前分配的节点 ID                      │
│  progress    INT        进度百分比                            │
│  milestone   JSON       里程碑状态                            │
│  created_at  TIMESTAMP  创建时间                              │
│  started_at  TIMESTAMP  开始执行时间                          │
│  completed_at TIMESTAMP  完成时间                             │
└─────────────────────────────────────────────────────────────┘
```

### 11.2 nodes 表

```
┌─────────────────────────────────────────────────────────────┐
│  id          VARCHAR    主键                                  │
│  type        ENUM       sandbox/docker/physical/cloud        │
│  capabilities JSON      能力标签 {browser, gpu, docker...}   │
│  status      ENUM       idle/busy/offline                    │
│  current_agent_id VARCHAR 当前处理的 Agent ID                 │
│  endpoint    VARCHAR    节点通信地址                          │
│  created_at  TIMESTAMP  注册时间                              │
└─────────────────────────────────────────────────────────────┘
```

### 11.3 agent_templates 表

```
┌─────────────────────────────────────────────────────────────┐
│  id          VARCHAR    主键                                  │
│  name        VARCHAR    模板名称                              │
│  description TEXT       模板描述                              │
│  prompts     JSON       {base_prompt, system_prompt}         │
│  tools       JSON       {allowed, restricted}                │
│  capabilities JSON      需要的节点能力                        │
│  is_system   BOOLEAN    是否系统预设                          │
│  created_by  VARCHAR    创建者（NULL 表示系统预设）            │
└─────────────────────────────────────────────────────────────┘
```

### 11.4 messages 表

```
┌─────────────────────────────────────────────────────────────┐
│  id          VARCHAR    主键                                  │
│  from_agent  VARCHAR    发送方 Agent ID                      │
│  proxy_for   VARCHAR    代理的原始 Agent ID                   │
│  to_agent    VARCHAR    接收方 Agent ID                      │
│  type        ENUM       notify/question/data/request_approval│
│  content     TEXT       消息内容                              │
│  status      ENUM       pending/delivered/read/responded    │
│  response    TEXT       回复内容（如果有）                     │
│  created_at  TIMESTAMP  发送时间                              │
│  delivered_at TIMESTAMP 送达时间                              │
└─────────────────────────────────────────────────────────────┘
```

### 11.5 approval_requests 表

```
┌─────────────────────────────────────────────────────────────┐
│  id          VARCHAR    主键                                  │
│  agent_id    VARCHAR    发起请求的 Agent ID                   │
│  action      VARCHAR    请求执行的操作                        │
│  action_detail JSON     操作详情                              │
│  risk_level  ENUM       low/medium/high                      │
│  status      ENUM       pending/approved/rejected/expired    │
│  user_id     VARCHAR    审批用户                              │
│  timeout_seconds INT     超时时间（高风险为 NULL）             │
│  created_at  TIMESTAMP  创建时间                              │
│  resolved_at TIMESTAMP  审批时间                              │
└─────────────────────────────────────────────────────────────┘
```

### 11.6 approval_policies 表

```
┌─────────────────────────────────────────────────────────────┐
│  id          VARCHAR    主键                                  │
│  user_id     VARCHAR    所属用户                              │
│  policy_type ENUM       default/custom                       │
│  rules       JSON       审批规则定义                          │
└─────────────────────────────────────────────────────────────┘
```

---

## 十二、技术选型

```
后端：Go（保持现有技术栈）
├── MessageRouter 服务
├── CoordinatorAgent 结构体
├── Worker 改造为 Agent
├── Node Registry + 调度器
├── WebSocket 双向通信

前端：React 19 + TypeScript（保持现有技术栈）
├── TaskTreeView 组件
├── AgentChatPanel 组件
├── ApprovalModal 组件
├── ProgressDashboard 组件
├── TemplateManager 组件
├── Zustand 状态管理扩展
├── WebSocket 实时更新

数据存储：SQLite（保持现有技术栈）
├── 新增表：agents, nodes, agent_templates, messages,
│           approval_requests, approval_policies
├── workspace：共享挂载路径

AI API：保持现有 ModelRouter
├── OpenAI / Anthropic / GLM
├── SSE 流式响应
├── Function Calling 支持
├── 新增：模板提示词注入、上下文加载
```

---

## 十三、实现优先级

```
Phase 1: 核心身份模型
├── 数据表：agents, nodes, agent_templates
├── Agent ID = Task ID 逻辑
├── root_id / parent_id 继承关系

Phase 2: Coordinator Agent
├── CoordinatorAgent 结构体
├── 任务拆解逻辑
├── 依赖解析与调度决策
├── 用户确认流程

Phase 3: 节点 Agent
├── Worker 改造为 Agent
├── 模板加载
├── workspace 管理
├── 进度汇报

Phase 4: 消息路由
├── 数据表：messages
├── MessageRouter 服务
├── 消息持久化
├── WebSocket 推送

Phase 5: 调度系统
├── 节点能力匹配
├── 任务分配流程
├── 节点生命周期管理

Phase 6: 前端可视化
├── TaskTreeView 组件
├── 进度展示
├── WebSocket 实时更新

Phase 7: 审批机制
├── 数据表：approval_requests, approval_policies
├── 分级审批逻辑
├── 用户审批界面

Phase 8: 恢复机制
├── workspace 历史数据记录
├── 继承链查询
├── 恢复任务创建

Phase 9: 用户介入
├── 与任意 Agent 对话
├── 暂停/取消任务
├── 节点重新分配

Phase 10: 模板管理
├── 系统预设模板
├── 用户自定义模板
├── 模板编辑界面
```

---

## 十四、完整流程示例

用户发起："帮我实现登录功能"

```
┌─ Coordinator 接收意图，拆解任务树 ─────────────────────────────┐
│                                                               │
│  登录功能 (Root)                                               │
│  ├── Task-001: 设计登录API文档                                │
│  ├── Task-002: 实现前端登录表单 (依赖 Task-001)                │
│  ├── Task-003: 实现后端验证逻辑 (依赖 Task-001)                │
│  ├── Task-004: 测试集成功能 (依赖 Task-002, Task-003)          │
│                                                               │
│  展示给用户确认                                                │
└───────────────────────────────────────────────────────────────┘
                          ↓ 用户确认
┌─ 调度阶段 ────────────────────────────────────────────────────┐
│                                                               │
│  Task-001 无依赖 → 立即调度                                    │
│  ├── N1 {browser} 分配给 Agent-001                            │
│  ├── Agent-001 启动 (DevTemplate + Task-001 上下文)            │
│                                                               │
│  Task-002、003 等待 Task-001 完成                              │
│  Task-004 等待 Task-002、003 完成                              │
│                                                               │
└───────────────────────────────────────────────────────────────┘
                          ↓
┌─ Task-001 执行 ────────────────────────────────────────────────┐
│                                                               │
│  Agent-001 在 workspace/001/ 工作                             │
│  ├── 汇报里程碑：设计 ✓ → 实现 ●                               │
│  ├── 调用工具：edit_file, write_file                          │
│  ├── 完成产物：api_spec.json                                   │
│  ├── Agent-001 离线                                           │
│                                                               │
│  Coordinator 收到完成消息                                      │
│  ├── 更新注册表：Agent-001.status = done                      │
│  ├── 通知下游：Task-002、003 依赖满足                          │
│                                                               │
└───────────────────────────────────────────────────────────────┘
                          ↓
┌─ Task-002、003 并行执行 ────────────────────────────────────────┐
│                                                               │
│  Agent-002 (N2) 加载 workspace/001/api_spec.json              │
│  Agent-003 (N3) 加载 workspace/001/api_spec.json              │
│  同时执行，并行推进                                             │
│                                                               │
│  Agent-002 发现 API 有歧义                                     │
│  ├── 发消息：{to: Agent-001, proxy_for: Agent-002,             │
│              content: "字段X的含义？"}                          │
│  ├── Coordinator 创建 Agent-006 恢复                          │
│  ├── Agent-006 加载 workspace/001/ 回答                       │
│  ├── Agent-006 回复：{from: Agent-006, proxy_for: Agent-001}   │
│  ├── Agent-002 收到回复继续执行                                │
│                                                               │
└───────────────────────────────────────────────────────────────┘
                          ↓
┌─ Task-004 测试 ────────────────────────────────────────────────┐
│                                                               │
│  Agent-004 执行测试                                            │
│  ├── 发现 bug                                                 │
│  ├── Coordinator 创建 Task-005 修复                           │
│  ├── Agent-005 修复完成                                        │
│  ├── 创建 Task-006 再次测试                                    │
│  ├── 循环直到测试通过                                          │
│                                                               │
└───────────────────────────────────────────────────────────────┘
                          ↓
┌─ 主任务完成 ────────────────────────────────────────────────────┐
│                                                               │
│  Coordinator 总结：                                            │
│  ├── 所有子任务完成                                            │
│  ├── 产物汇总                                                  │
│  ├── 汇报用户："登录功能已实现完成"                             │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

---

## 十五、核心设计决策总结

| 决策点 | 选择 |
|--------|------|
| Agent ID 与 Task ID | 统一，同一标识符 |
| 继承关系存储 | root_id + parent_id（方案C） |
| Agent 模板来源 | 系统预设 + 用户自定义（混合） |
| Coordinator 角色 | 也是 Agent（统一架构） |
| 通信路由 | 发送方指定原始 ID，Coordinator 路由 |
| 恢复策略 | 每次新建 Agent，proxy_for 声明 |
| 节点隔离 | 按能力标记，任务完成立即释放 |
| 依赖类型 | 静态依赖 + 数据流依赖 |
| 进度机制 | Agent 汇报 + 工具推断 + 里程碑 |
| 用户介入 | 监控 + 审批 + 对话 + 控制 |
| 审批策略 | 分级审批 + 用户可修改 |
| 任务拆解确认 | 默认预览确认，可选信任模式 |