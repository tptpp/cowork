# Cowork API 设计文档

## 1. API 概述

### 1.1 基础信息

- **Base URL**: `http://localhost:8080/api`
- **协议**: HTTP/1.1, WebSocket
- **数据格式**: JSON
- **字符编码**: UTF-8
- **时间格式**: ISO 8601 (`2024-01-01T00:00:00Z`)

### 1.2 通用响应格式

```typescript
// 成功响应
{
  "success": true,
  "data": { ... }
}

// 错误响应
{
  "success": false,
  "error": {
    "code": "TASK_NOT_FOUND",
    "message": "Task with ID 'task-123' not found",
    "details": { ... }  // 可选
  }
}

// 分页响应
{
  "success": true,
  "data": [...],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

### 1.3 通用错误码

| 错误码 | HTTP 状态码 | 说明 |
|--------|-------------|------|
| `INVALID_REQUEST` | 400 | 请求参数无效 |
| `UNAUTHORIZED` | 401 | 未认证 |
| `FORBIDDEN` | 403 | 无权限 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 资源冲突 |
| `INTERNAL_ERROR` | 500 | 内部错误 |

---

## 2. 任务 API (Tasks)

### 2.1 创建任务

```http
POST /api/tasks
Content-Type: application/json

{
  "type": "development",           // 必填：任务类型
  "description": "实现用户登录API", // 任务描述
  "priority": "high",              // 可选：low, medium, high (默认 medium)
  "required_tags": ["dev", "coding"], // 可选：需要的 Worker 标签
  "preferred_model": "gpt-4",      // 可选：偏好模型
  "input": {                       // 任务输入数据
    "repo_url": "https://github.com/...",
    "branch": "main"
  },
  "config": {                      // 任务配置
    "timeout": 3600,               // 超时时间（秒）
    "retry": 2,                    // 重试次数
    "notify_on_complete": true     // 完成时通知
  }
}
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": "task-101",
    "type": "development",
    "status": "pending",
    "priority": "high",
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

### 2.2 获取任务列表

```http
GET /api/tasks?page=1&page_size=20&status=running&type=development&sort=-created_at
```

**查询参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码，默认 1 |
| `page_size` | int | 每页数量，默认 20，最大 100 |
| `status` | string | 状态筛选：pending, running, completed, failed, cancelled |
| `type` | string | 类型筛选 |
| `worker_id` | string | Worker 筛选 |
| `sort` | string | 排序字段，`-` 前缀表示降序 |

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "task-101",
      "type": "development",
      "status": "running",
      "progress": 45,
      "priority": "high",
      "worker_id": "worker-1",
      "created_at": "2024-01-01T10:00:00Z",
      "started_at": "2024-01-01T10:01:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

### 2.3 获取任务详情

```http
GET /api/tasks/:id
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": "task-101",
    "type": "development",
    "description": "实现用户登录API",
    "status": "running",
    "progress": 45,
    "priority": "high",
    "worker_id": "worker-1",
    "required_tags": ["dev", "coding"],
    "preferred_model": "gpt-4",
    "input": {
      "repo_url": "https://github.com/...",
      "branch": "main"
    },
    "output": null,
    "error": null,
    "config": {
      "timeout": 3600,
      "retry": 2
    },
    "work_dir": "/tmp/cowork/task-101",
    "files": [
      { "name": "output.txt", "size": 1024, "modified_at": "..." }
    ],
    "created_at": "2024-01-01T10:00:00Z",
    "started_at": "2024-01-01T10:01:00Z",
    "updated_at": "2024-01-01T10:05:00Z",
    "completed_at": null
  }
}
```

### 2.4 取消任务

```http
DELETE /api/tasks/:id
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": "task-101",
    "status": "cancelled"
  }
}
```

### 2.5 获取任务日志

```http
GET /api/tasks/:id/logs?lines=100&offset=0
```

**查询参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| `lines` | int | 返回行数，默认 100 |
| `offset` | int | 偏移量，用于分页 |
| `follow` | bool | 是否流式返回（SSE） |

**响应**:
```json
{
  "success": true,
  "data": {
    "lines": [
      { "time": "2024-01-01T10:01:00Z", "level": "info", "message": "Task started" },
      { "time": "2024-01-01T10:02:00Z", "level": "info", "message": "Cloning repository..." }
    ],
    "total": 50,
    "has_more": false
  }
}
```

### 2.6 获取任务输出文件

```http
GET /api/tasks/:id/files
```

**响应**:
```json
{
  "success": true,
  "data": {
    "files": [
      { "name": "output.txt", "size": 1024, "type": "text/plain" },
      { "name": "report.pdf", "size": 2048, "type": "application/pdf" }
    ]
  }
}
```

### 2.7 下载任务输出文件

```http
GET /api/tasks/:id/files/:filename
```

**响应**: 文件流（Content-Type 根据文件类型设置）

---

## 3. Worker API

### 3.1 注册 Worker

```http
POST /api/workers/register
Content-Type: application/json

{
  "name": "worker-dev-01",           // 必填：Worker 名称
  "tags": ["dev", "coding"],         // 必填：能力标签
  "model": "gpt-4",                  // 可选：默认模型
  "max_concurrent": 3,               // 可选：最大并发任务数，默认 1
  "capabilities": {                  // 可选：能力配置
    "docker": false,
    "gpu": false,
    "work_dir": "/tmp/cowork"
  },
  "metadata": {                      // 可选：元数据
    "hostname": "laptop-01",
    "os": "linux"
  }
}
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": "worker-uuid-123",
    "name": "worker-dev-01",
    "status": "idle",
    "heartbeat_interval": 5,
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

### 3.2 发送心跳

```http
POST /api/workers/:id/heartbeat
Content-Type: application/json

{
  "status": "busy",                  // idle, busy, error
  "current_tasks": ["task-101", "task-102"],
  "progress": {                      // 任务进度
    "task-101": 45,
    "task-102": 80
  },
  "resources": {                     // 可选：资源使用
    "cpu_percent": 30,
    "memory_mb": 512,
    "disk_mb": 1024
  }
}
```

**响应**:
```json
{
  "success": true,
  "data": {
    "assigned_tasks": ["task-103"],  // 新分配的任务
    "cancelled_tasks": [],           // 需要取消的任务
    "commands": []                   // 待执行命令
  }
}
```

### 3.3 获取 Worker 列表

```http
GET /api/workers?status=online
```

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "worker-uuid-123",
      "name": "worker-dev-01",
      "tags": ["dev", "coding"],
      "model": "gpt-4",
      "status": "busy",
      "current_tasks": ["task-101"],
      "max_concurrent": 3,
      "last_seen": "2024-01-01T10:05:00Z",
      "created_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

### 3.4 获取 Worker 详情

```http
GET /api/workers/:id
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": "worker-uuid-123",
    "name": "worker-dev-01",
    "tags": ["dev", "coding"],
    "model": "gpt-4",
    "status": "busy",
    "current_tasks": [
      { "id": "task-101", "progress": 45 }
    ],
    "completed_tasks": 15,
    "max_concurrent": 3,
    "capabilities": {
      "docker": false,
      "gpu": false
    },
    "resources": {
      "cpu_percent": 30,
      "memory_mb": 512
    },
    "last_seen": "2024-01-01T10:05:00Z",
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

### 3.5 注销 Worker

```http
DELETE /api/workers/:id
```

---

## 4. Agent API

### 4.1 创建会话

```http
POST /api/agent/sessions
Content-Type: application/json

{
  "model": "gpt-4",                  // 可选：模型选择
  "system_prompt": "你是一个代码助手", // 可选：系统提示
  "context": {                       // 可选：初始上下文
    "files": ["main.go", "utils.go"],
    "task_id": "task-101"            // 关联任务
  },
  "config": {                        // 可选：配置
    "temperature": 0.7,
    "max_tokens": 4096
  }
}
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": "session-123",
    "model": "gpt-4",
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

### 4.2 发送消息（流式）

```http
POST /api/agent/chat
Content-Type: application/json

{
  "session_id": "session-123",
  "message": "帮我分析这段代码的问题",
  "files": ["main.go"],              // 可选：附加文件
  "stream": true                     // 可选：是否流式，默认 true
}
```

**响应（流式）**:
```
data: {"type": "chunk", "content": "这"}
data: {"type": "chunk", "content": "段"}
data: {"type": "chunk", "content": "代码"}
data: {"type": "done", "usage": {"prompt_tokens": 100, "completion_tokens": 50}}
```

**响应（非流式）**:
```json
{
  "success": true,
  "data": {
    "message": {
      "role": "assistant",
      "content": "这段代码的问题是...",
      "created_at": "2024-01-01T10:01:00Z"
    },
    "usage": {
      "prompt_tokens": 100,
      "completion_tokens": 50
    }
  }
}
```

### 4.3 获取会话历史

```http
GET /api/agent/sessions/:id/messages?limit=50
```

**响应**:
```json
{
  "success": true,
  "data": {
    "messages": [
      { "role": "user", "content": "帮我分析代码", "created_at": "..." },
      { "role": "assistant", "content": "好的，让我看看...", "created_at": "..." }
    ],
    "total": 10
  }
}
```

### 4.4 删除会话

```http
DELETE /api/agent/sessions/:id
```

### 4.5 获取会话列表

```http
GET /api/agent/sessions
```

---

## 5. 系统 API

### 5.1 获取系统统计

```http
GET /api/system/stats
```

**响应**:
```json
{
  "success": true,
  "data": {
    "tasks": {
      "total": 100,
      "pending": 5,
      "running": 3,
      "completed": 88,
      "failed": 4
    },
    "workers": {
      "total": 3,
      "online": 2,
      "offline": 1
    },
    "system": {
      "uptime": "2h 30m",
      "version": "1.0.0",
      "go_version": "go1.21"
    }
  }
}
```

### 5.2 获取通知列表

```http
GET /api/system/notifications?unread_only=true&limit=20
```

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "notif-123",
      "type": "task_complete",
      "title": "任务完成",
      "message": "任务 task-101 已完成",
      "read": false,
      "created_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

### 5.3 标记通知已读

```http
POST /api/system/notifications/read
Content-Type: application/json

{
  "ids": ["notif-123", "notif-124"]  // 或 "all" 标记全部已读
}
```

### 5.4 获取可用模型

```http
GET /api/system/models
```

**响应**:
```json
{
  "success": true,
  "data": [
    { "id": "gpt-4", "name": "GPT-4", "provider": "openai" },
    { "id": "claude-3", "name": "Claude 3", "provider": "anthropic" },
    { "id": "glm-4", "name": "GLM-4", "provider": "zhipu" }
  ]
}
```

---

## 6. 用户布局 API

### 6.1 获取用户布局

```http
GET /api/user/layout
```

**响应**:
```json
{
  "success": true,
  "data": {
    "widgets": [
      {
        "id": "widget-1",
        "type": "task-queue",
        "title": "任务队列",
        "layout": { "x": 0, "y": 0, "w": 4, "h": 3 },
        "settings": { "max_tasks": 10, "auto_refresh": 30 }
      },
      {
        "id": "widget-2",
        "type": "agent-chat",
        "title": "Agent 对话",
        "layout": { "x": 4, "y": 0, "w": 4, "h": 4 },
        "settings": { "model": "gpt-4" }
      }
    ],
    "version": 1,
    "updated_at": "2024-01-01T10:00:00Z"
  }
}
```

### 6.2 保存用户布局

```http
POST /api/user/layout
Content-Type: application/json

{
  "widgets": [
    {
      "id": "widget-1",
      "type": "task-queue",
      "title": "任务队列",
      "layout": { "x": 0, "y": 0, "w": 4, "h": 3 },
      "settings": { "max_tasks": 10 }
    }
  ]
}
```

---

## 7. Widget 模板 API

### 7.1 获取可用 Widget 列表

```http
GET /api/widgets
```

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "type": "task-queue",
      "name": "任务队列",
      "icon": "📊",
      "description": "显示运行中和排队的任务",
      "category": "system",
      "default_size": { "w": 4, "h": 3 },
      "settings_schema": [
        { "key": "max_tasks", "type": "number", "label": "最大显示数", "default": 10 },
        { "key": "auto_refresh", "type": "number", "label": "刷新间隔(秒)", "default": 30 }
      ]
    },
    {
      "type": "agent-chat",
      "name": "Agent 对话",
      "icon": "🤖",
      "description": "与 AI Agent 对话",
      "category": "agent",
      "default_size": { "w": 4, "h": 4 },
      "settings_schema": [
        { "key": "model", "type": "select", "label": "模型", "options": ["gpt-4", "claude-3"] },
        { "key": "session_id", "type": "string", "label": "会话ID" }
      ]
    }
  ]
}
```

### 7.2 安装 Widget 模板

```http
POST /api/widgets/install
Content-Type: application/json

{
  "template_url": "https://example.com/widgets/custom-widget.json"
}
```

---

## 8. WebSocket API

### 8.1 连接

```
ws://localhost:8080/ws
```

### 8.2 消息格式

**客户端 → 服务端**:

```typescript
// 订阅频道
{
  "type": "subscribe",
  "channels": ["tasks", "worker-1", "session-123"]
}

// 取消订阅
{
  "type": "unsubscribe",
  "channels": ["worker-1"]
}

// 心跳
{
  "type": "ping"
}
```

**服务端 → 客户端**:

```typescript
// 任务更新
{
  "type": "task_update",
  "payload": {
    "id": "task-101",
    "status": "running",
    "progress": 45
  }
}

// 任务日志
{
  "type": "task_log",
  "payload": {
    "task_id": "task-101",
    "line": { "time": "...", "level": "info", "message": "..." }
  }
}

// Agent 消息块
{
  "type": "agent_chunk",
  "payload": {
    "session_id": "session-123",
    "chunk": "这"
  }
}

// 通知
{
  "type": "notification",
  "payload": {
    "id": "notif-123",
    "type": "task_complete",
    "title": "任务完成",
    "message": "..."
  }
}

// Worker 状态更新
{
  "type": "worker_update",
  "payload": {
    "id": "worker-1",
    "status": "offline"
  }
}

// 心跳响应
{
  "type": "pong"
}

// 错误
{
  "type": "error",
  "payload": {
    "code": "INVALID_CHANNEL",
    "message": "Unknown channel: invalid-channel"
  }
}
```

### 8.3 频道说明

| 频道 | 说明 | 推送事件 |
|------|------|----------|
| `tasks` | 所有任务更新 | task_update, task_log |
| `task-{id}` | 特定任务更新 | task_update, task_log |
| `workers` | 所有 Worker 更新 | worker_update |
| `worker-{id}` | 特定 Worker 更新 | worker_update |
| `session-{id}` | Agent 会话消息 | agent_chunk, agent_done |
| `notifications` | 系统通知 | notification |
| `system` | 系统级事件 | system_event |

---

## 9. 文件上传 API

### 9.1 上传文件

```http
POST /api/upload
Content-Type: multipart/form-data

file: <binary>
```

**响应**:
```json
{
  "success": true,
  "data": {
    "id": "file-123",
    "filename": "document.pdf",
    "size": 10240,
    "url": "/api/files/file-123"
  }
}
```

### 9.2 下载文件

```http
GET /api/files/:id
```

---

## 10. OpenAPI 规范

完整的 OpenAPI 3.0 规范将保存在 `docs/openapi.yaml`。

```yaml
# docs/openapi.yaml (待生成)
openapi: 3.0.0
info:
  title: Cowork API
  version: 1.0.0
  description: 个人多任务处理平台 API
servers:
  - url: http://localhost:8080/api
    description: 本地开发服务器
# ... 详细定义见完整文件
```