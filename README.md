# Cowork - 个人多任务处理平台

一个支持多 Worker 并行执行、可扩展的个人任务调度平台。

## 概述

Cowork 是一个分布式任务处理平台，支持：

- **多任务并行**：多个 Worker 同时处理不同任务
- **灵活调度**：基于标签的任务分发，支持模型偏好
- **实时监控**：WebSocket 推送任务状态、进度
- **可扩展布局**：拖拽式 Dashboard，Widget 自定义
- **多模型支持**：不同 Worker 可配置不同 AI 模型

## 架构

```
┌─────────────────────────────────────────────────────┐
│                    Web Frontend                       │
│                 (React + TypeScript)                 │
└───────────────────────┬─────────────────────────────┘
                        │ WebSocket + REST
┌───────────────────────▼─────────────────────────────┐
│                      Gateway                          │
│  • REST API + WebSocket                               │
│  • 任务调度 / 负载均衡                                │
│  • Worker 注册/心跳                                  │
│  • Agent 对话路由                                    │
└───────────────────────┬─────────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
   ┌─────────┐    ┌─────────┐    ┌─────────┐
   │ Worker 1│    │ Worker 2│    │ Worker N│
   │ (本地)   │    │ (Docker)│    │ (云上)   │
   └─────────┘    └─────────┘    └─────────┘
```

## 技术栈

| 组件 | 技术 | 理由 |
|------|------|------|
| Gateway | Go | 高性能、单二进制部署 |
| Worker | Go | 与 Gateway 同语言 |
| Frontend | React + TypeScript | 生态成熟、类型安全 |
| 状态管理 | Zustand | 轻量、简单 |
| 布局引擎 | react-grid-layout | 可拖拽 Dashboard |
| 数据库 | SQLite | 零配置、易迁移 |
| 通信 | REST + WebSocket | 简单直接 |

## 项目结构

```
cowork/
├── cmd/
│   ├── gateway/          # Gateway 主程序入口
│   └── worker/           # Worker 主程序入口
├── internal/
│   ├── gateway/
│   │   ├── api/          # REST API 处理
│   │   ├── ws/           # WebSocket 处理
│   │   ├── scheduler/    # 任务调度器
│   │   ├── store/        # 数据存储层
│   │   ├── agent/        # Agent 对话路由
│   │   └── system/       # 系统管理（Worker、通知）
│   ├── worker/
│   │   ├── executor/     # 任务执行器
│   │   ├── heartbeat/    # 心跳上报
│   │   └── isolate/      # 工作目录隔离
│   └── shared/
│       ├── protocol/     # 通信协议定义
│       └── models/       # 共享数据模型
├── web/                  # React 前端
│   ├── src/
│   │   ├── components/   # React 组件
│   │   ├── stores/       # Zustand Stores
│   │   ├── hooks/        # 自定义 Hooks
│   │   └── types/        # TypeScript 类型
│   └── public/
├── docker/               # Docker 配置
│   ├── Dockerfile.gateway
│   ├── Dockerfile.worker
│   └── docker-compose.yml
├── docs/                 # 项目文档
└── scripts/              # 构建脚本
```

## 快速开始

```bash
# 启动 Gateway
go run ./cmd/gateway

# 启动 Worker（另一个终端）
go run ./cmd/worker --name=worker-1 --tags=dev,coding --model=gpt-4

# 启动前端
cd web && npm run dev
```

## 文档

- [架构设计](./docs/architecture.md)
- [API 设计](./docs/api-design.md)
- [数据模型](./docs/data-model.md)
- [前端设计](./docs/frontend-design.md)
- [部署指南](./docs/deployment.md)
- [开发路线图](./docs/roadmap.md)

## License

MIT