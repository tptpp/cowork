# Cowork 架构设计文档

## 1. 系统架构

### 1.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Frontend (React)                           │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      Presentation Layer                        │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐ │  │
│  │  │ Dashboard   │ │ Widget 1    │ │ Widget N               │ │  │
│  │  │ Container   │ │ (TaskQueue) │ │ (AgentChat)            │ │  │
│  │  └─────────────┘ └─────────────┘ └─────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                        State Layer (Zustand)                   │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐ │  │
│  │  │ TaskStore   │ │ AgentStore  │ │ SystemStore            │ │  │
│  │  └──────┬──────┘ └──────┬──────┘ └──────────┬──────────────┘ │  │
│  └─────────┼───────────────┼───────────────────┼────────────────┘  │
│            └───────────────┴───────────────────┘                   │
│                            │                                        │
│              ┌─────────────▼─────────────┐                         │
│              │    WebSocket Client       │                         │
│              │    + HTTP API Client      │                         │
│              └─────────────┬─────────────┘                         │
└────────────────────────────┼────────────────────────────────────────┘
                             │
                             │ WebSocket + REST
                             │
┌────────────────────────────▼────────────────────────────────────────┐
│                           Coordinator (Go)                            │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                         API Layer                              │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐ │  │
│  │  │ REST Router │ │ WebSocket   │ │ Static File             │ │  │
│  │  │ (Gin/Echo)  │ │ Hub         │ │ Server                  │ │  │
│  │  └──────┬──────┘ └──────┬──────┘ └──────────┬──────────────┘ │  │
│  └─────────┼───────────────┼───────────────────┼────────────────┘  │
│            │               │                   │                    │
│  ┌─────────▼───────────────▼───────────────────▼────────────────┐  │
│  │                       Service Layer                           │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐ │  │
│  │  │ TaskService │ │ AgentService│ │ SystemService          │ │  │
│  │  │             │ │             │ │                         │ │  │
│  │  │ • Create    │ │ • Chat      │ │ • Worker Registry       │ │  │
│  │  │ • Update    │ │ • Stream    │ │ • Notifications         │ │  │
│  │  │ • Cancel    │ │ • Context   │ │ • Statistics            │ │  │
│  │  └──────┬──────┘ └──────┬──────┘ └──────────┬──────────────┘ │  │
│  └─────────┼───────────────┼───────────────────┼────────────────┘  │
│            │               │                   │                    │
│  ┌─────────▼───────────────▼───────────────────▼────────────────┐  │
│  │                       Scheduler Layer                         │  │
│  │  ┌─────────────────────────────────────────────────────────┐ │  │
│  │  │                   Task Scheduler                         │ │  │
│  │  │  • Worker 匹配（标签、模型）                             │ │  │
│  │  │  • 负载均衡                                             │ │  │
│  │  │  • 任务分发                                             │ │  │
│  │  └─────────────────────────────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                        Data Layer                              │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐ │  │
│  │  │ SQLite      │ │ Cache       │ │ File Storage           │ │  │
│  │  │ (GORM)      │ │ (Ristretto) │ │ (Local/S3)             │ │  │
│  │  └─────────────┘ └─────────────┘ └─────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             │ gRPC / HTTP
                             │
┌────────────────────────────▼────────────────────────────────────────┐
│                           Workers                                    │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────────┐   │
│  │ Worker 1    │ │ Worker 2    │ │ Worker N                   │   │
│  │             │ │             │ │                             │   │
│  │ Tags: dev   │ │ Tags: test  │ │ Tags: docker, isolated     │   │
│  │ Model: GPT-4│ │ Model: Claude│ │ Model: GLM-4              │   │
│  │             │ │             │ │                             │   │
│  │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌───────────────────────┐ │   │
│  │ │Executor │ │ │ │Executor │ │ │ │ Executor              │ │   │
│  │ ├─────────┤ │ │ ├─────────┤ │ │ ├───────────────────────┤ │   │
│  │ │WorkDir  │ │ │ │WorkDir  │ │ │ │ Container Per Task    │ │   │
│  │ │/tmp/101 │ │ │ │/tmp/102 │ │ │ │ docker run ...        │ │   │
│  │ └─────────┘ │ │ └─────────┘ │ │ └───────────────────────┘ │   │
│  └─────────────┘ └─────────────┘ └─────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 组件职责

| 组件 | 职责 | 关键功能 |
|------|------|----------|
| **Frontend** | 用户界面 | Dashboard 渲染、Widget 管理、用户交互 |
| **Coordinator** | 核心服务 | API 网关、任务调度、数据存储、WebSocket 推送 |
| **Worker** | 任务执行 | 任务执行、心跳上报、工作目录管理 |
| **SQLite** | 持久化存储 | 任务、Worker、会话、布局数据 |
| **Cache** | 内存缓存 | 热数据缓存、减少数据库压力 |

---

## 2. 核心流程

### 2.1 任务生命周期

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Task Lifecycle                                │
│                                                                      │
│   [创建] → [排队] → [分发] → [运行] → [完成/失败]                    │
│     │        │        │        │           │                         │
│     │        │        │        │           │                         │
│   用户提交  等待    Scheduler  Worker    结果存储                    │
│            Worker   匹配       执行      通知推送                    │
│            空闲                                       │              │
│                                                       ▼              │
│                                              ┌──────────────┐        │
│                                              │ WebSocket    │        │
│                                              │ 推送更新     │        │
│                                              └──────────────┘        │
└─────────────────────────────────────────────────────────────────────┘

详细流程：

1. 用户通过 Frontend 提交任务
   POST /api/tasks
   { "type": "development", "description": "..." }

2. Gateway 创建任务，状态为 pending
   Task { id: "task-101", status: "pending", ... }

3. Scheduler 定期检查待分配任务
   - 查找所有 pending 状态的任务
   - 查找匹配标签的空闲 Worker
   - 分配任务，状态变为 running

4. Worker 收到任务，开始执行
   - 创建工作目录 /tmp/cowork/{task_id}
   - 执行任务逻辑
   - 定期发送心跳（含进度）

5. Gateway 收集心跳，更新任务状态
   - 更新数据库中的进度
   - 通过 WebSocket 推送给订阅的客户端

6. 任务完成
   - Worker 上报结果
   - Gateway 更新状态为 completed/failed
   - 存储输出文件
   - 推送通知
```

### 2.2 Worker 注册与心跳

```
Worker 启动流程：

1. Worker 启动，连接 Gateway
   POST /api/workers/register
   { "name": "worker-1", "tags": ["dev"], "model": "gpt-4", "max_concurrent": 3 }

2. Gateway 注册 Worker
   - 分配 Worker ID
   - 存储到数据库
   - 加入调度池

3. Worker 开始心跳循环
   每 5 秒发送一次心跳：
   POST /api/workers/{id}/heartbeat
   { 
     "status": "busy",
     "current_tasks": ["task-101"],
     "progress": { "task-101": 45 },
     "resources": { "cpu": 30, "memory": 512 }
   }

4. Gateway 处理心跳
   - 更新 Worker 状态
   - 更新关联任务的进度
   - 推送 WebSocket 更新

5. 心跳超时处理
   - 超过 30 秒无心跳 → 标记 Worker 为 offline
   - 重新分配其运行中的任务
```

### 2.3 Agent 对话流程

```
Agent 对话流程：

1. 用户创建会话
   POST /api/agent/sessions
   { "model": "gpt-4", "context": { ... } }
   
   Response: { "session_id": "sess-123" }

2. 用户发送消息
   POST /api/agent/chat
   { "session_id": "sess-123", "message": "帮我分析这段代码" }

3. Gateway 调用 LLM API（流式）
   - 路由到配置的模型 API
   - 启用流式响应 (stream: true)

4. 流式响应推送到前端
   WebSocket 推送：
   { "type": "agent_chunk", "session_id": "sess-123", "chunk": "这" }
   { "type": "agent_chunk", "session_id": "sess-123", "chunk": "段" }
   ...

5. 对话完成
   WebSocket 推送：
   { "type": "agent_done", "session_id": "sess-123" }

6. 会话历史存储
   - 保存到 SQLite
   - 支持后续继续对话
```

---

## 3. 数据流设计

### 3.1 实时数据流

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Real-time Data Flow                              │
│                                                                      │
│   Worker                                                             │
│     │                                                                │
│     │ 心跳 (HTTP)                                                   │
│     ▼                                                                │
│   Gateway ──────────────────────────────────────────────────────┐   │
│     │                                                            │   │
│     │ 1. 更新数据库                                              │   │
│     │ 2. 推送到 WebSocket Hub                                    │   │
│     ▼                                                            │   │
│   ┌─────────────┐                                                │   │
│   │ WebSocket   │                                                │   │
│   │ Hub         │                                                │   │
│   └──────┬──────┘                                                │   │
│          │                                                        │   │
│          │ 根据订阅分发                                          │   │
│          │                                                        │   │
│   ┌──────┴──────┬──────────────┬──────────────┐                 │   │
│   ▼             ▼              ▼              ▼                 │   │
│ Client 1    Client 2       Client 3       Client N              │   │
│ (订阅 tasks) (订阅 worker-1) (订阅 sess-123) (订阅 notifications)│   │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 订阅机制

```typescript
// 客户端订阅
ws.send(JSON.stringify({
  type: 'subscribe',
  channels: ['tasks', 'worker-1', 'session-123', 'notifications']
}));

// 可订阅的频道
type Channel = 
  | 'tasks'              // 所有任务更新
  | `task-${string}`     // 特定任务更新
  | `worker-${string}`   // 特定 Worker 状态
  | `session-${string}`  // Agent 会话消息
  | 'notifications'      // 系统通知
  | 'system';            // 系统级事件
```

---

## 4. 扩展性设计

### 4.1 水平扩展

```
单机部署：
┌─────────────────────────────────────────────┐
│  Gateway + SQLite + 多个本地 Worker         │
└─────────────────────────────────────────────┘

多机部署：
┌──────────────┐
│   Frontend   │
└──────┬───────┘
       │
┌──────▼───────┐
│ Load Balancer│
└──────┬───────┘
       │
┌──────┴───────┬──────────────┬──────────────┐
▼              ▼              ▼              ▼
Gateway 1   Gateway 2    Gateway 3    Gateway N
    │            │            │            │
    └────────────┴────────────┴────────────┘
                        │
              ┌─────────▼─────────┐
              │  PostgreSQL       │
              │  (共享数据库)      │
              └───────────────────┘
                        │
              ┌─────────▼─────────┐
              │  Redis            │
              │  (Pub/Sub + Cache)│
              └───────────────────┘

Worker 集群：
┌──────────┐ ┌──────────┐ ┌──────────┐
│ Worker 1 │ │ Worker 2 │ │ Worker N │
│ (本地)   │ │ (Docker) │ │ (云端)   │
└──────────┘ └──────────┘ └──────────┘
```

### 4.2 模块化设计

```
Coordinator 模块化：

cmd/coordinator/main.go
├── 内置模块（默认启用）
│   ├── TaskModule      // 任务管理
│   ├── AgentModule     // Agent 对话
│   ├── SystemModule    // 系统管理
│   └── StaticModule    // 静态文件服务
│
└── 可选模块（配置启用）
    ├── AuthModule      // 认证授权
    ├── MetricsModule   // Prometheus 指标
    └── TracingModule   // 分布式追踪

Worker 扩展：

internal/worker/executor/
├── base.go          // 基础执行器接口
├── shell.go         // Shell 命令执行器
├── docker.go        // Docker 容器执行器
└── custom.go        // 自定义执行器（用户实现）

// 用户可实现自己的执行器
type Executor interface {
    Execute(ctx context.Context, task *Task) (*Result, error)
    Cancel(taskID string) error
    Status(taskID string) (*Status, error)
}
```

---

## 5. 安全设计

### 5.1 认证与授权

```
认证方案（可选）：

1. API Key 认证
   - Gateway 和 Worker 之间
   - 请求头: Authorization: Bearer <api_key>

2. JWT 认证（多用户）
   - 用户登录获取 JWT
   - 请求携带 JWT
   - Gateway 验证 JWT 并提取用户信息

授权模型：

用户角色：
- admin: 全部权限
- user: 创建任务、查看自己的任务
- viewer: 只读访问

资源权限：
- 任务: 创建者可查看/取消
- Worker: admin 可管理
- 布局: 用户只能访问自己的布局
```

### 5.2 数据隔离

```
任务隔离：

1. 文件系统隔离
   - 每个任务独立工作目录
   - /tmp/cowork/{task_id}/
   - 任务完成后可选择保留或清理

2. 容器隔离（Docker Worker）
   - 每个任务一个容器
   - 限制 CPU/内存
   - 网络隔离（可选）

3. 上下文隔离
   - 任务间不共享状态
   - Agent 会话独立
```

---

## 6. 监控与日志

### 6.1 日志设计

```go
// 结构化日志
log.Info("task_created",
    "task_id", task.ID,
    "type", task.Type,
    "user_id", userID,
)

log.Info("worker_heartbeat",
    "worker_id", worker.ID,
    "status", worker.Status,
    "tasks", worker.CurrentTasks,
)

log.Error("task_failed",
    "task_id", task.ID,
    "error", err,
    "stack", debug.Stack(),
)
```

### 6.2 指标收集

```
Prometheus 指标示例：

# 任务指标
cowork_tasks_total{status="completed"} 123
cowork_tasks_total{status="failed"} 5
cowork_tasks_duration_seconds{type="development"} 0.5

# Worker 指标
cowork_workers_total{status="online"} 3
cowork_workers_tasks_running{id="worker-1"} 2

# 系统指标
cowork_http_requests_total{method="GET", path="/api/tasks"} 1000
cowork_websocket_connections_active 5
```

---

## 7. 容错设计

### 7.1 故障恢复

```
Gateway 故障：
Coordinator 故障：
- 无状态设计，可快速重启
- SQLite 数据持久化
- 定期备份

Worker 故障：
- 心跳超时自动标记 offline
- 运行中的任务重新分配
- 任务支持断点续传（可选）

网络故障：
- WebSocket 自动重连
- 心跳失败重试
- 请求幂等设计
```

### 7.2 优雅关闭

```go
// Gateway 优雅关闭
func (g *Gateway) Shutdown(ctx context.Context) error {
    // 1. 停止接受新连接
    g.httpServer.Shutdown(ctx)
    
    // 2. 通知所有 Worker 暂停
    g.broadcastWorkerPause()
    
    // 3. 等待运行中任务完成或超时
    g.waitForTasks(ctx, 30*time.Second)
    
    // 4. 保存状态
    g.store.Save()
    
    return nil
}

// Worker 优雅关闭
func (w *Worker) Shutdown(ctx context.Context) error {
    // 1. 通知 Coordinator 即将下线
    w.coordinatorClient.NotifyShutdown()

    // 2. 完成当前任务
    w.waitForCurrentTasks(ctx)
    
    // 3. 清理工作目录
    w.cleanup()
    
    return nil
}
```

---

## 8. 性能考量

### 8.1 性能目标

| 指标 | 目标值 |
|------|--------|
| API 响应时间 | P99 < 100ms |
| WebSocket 延迟 | < 50ms |
| 任务调度延迟 | < 1s |
| 并发连接数 | 100+ |
| 任务吞吐量 | 100+ tasks/min |

### 8.2 优化策略

```
1. 数据库优化
   - SQLite WAL 模式
   - 合理索引
   - 批量写入

2. 缓存策略
   - Worker 状态缓存
   - 热点任务缓存
   - 布局缓存

3. WebSocket 优化
   - 连接池
   - 消息批量发送
   - 压缩（可选）

4. 任务调度优化
   - 批量分配
   - 优先级队列
   - 抢占式调度（可选）
```