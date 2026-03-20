# Cowork - 个人多任务处理平台

[![Status](https://img.shields.io/badge/status-active--development-green)](https://github.com/tptpp/cowork)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.x-3178C6?logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue)](LICENSE)

一个支持多 Worker 并行执行、可扩展的个人任务调度平台。

## 功能特性

| 功能模块 | 状态 | 描述 |
|----------|------|------|
| **任务系统** | ✅ 100% | 任务创建、调度、执行、取消、日志 |
| **Worker 管理** | ✅ 100% | 注册、心跳、状态监控、故障转移 |
| **Dashboard** | ✅ 100% | 可拖拽布局、Widget 管理、布局持久化 |
| **Agent Chat** | ✅ 100% | 多模型对话、SSE 流式响应、历史持久化 |
| **Settings** | ✅ 100% | 模型配置、主题切换（深色/浅色/系统） |
| **通知系统** | ✅ 100% | 任务完成/失败通知、实时推送 |
| **Todo List** | ✅ 100% | 个人待办清单 Widget |
| **Docker 隔离** | 🔄 25% | 容器执行、资源限制（开发中） |

**总体完成度: 90%** - 核心功能已完整实现，可用于生产环境。

### 核心能力

- **多任务并行**：多个 Worker 同时处理不同任务
- **灵活调度**：基于标签的任务分发，支持模型偏好
- **实时监控**：WebSocket 推送任务状态、进度
- **可扩展布局**：拖拽式 Dashboard，Widget 自定义
- **多模型支持**：OpenAI/Anthropic/GLM 等 AI 模型路由

## 架构

```
┌─────────────────────────────────────────────────────┐
│                    Web Frontend                      │
│                 (React + TypeScript)                 │
└───────────────────────┬─────────────────────────────┘
                        │ WebSocket + REST
┌───────────────────────▼─────────────────────────────┐
│                    Coordinator                       │
│  • REST API + WebSocket                              │
│  • 任务调度 / 负载均衡                               │
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
| Coordinator | Go | 高性能、单二进制部署 |
| Worker | Go | 与 Coordinator 同语言 |
| Frontend | React 19 + TypeScript | 生态成熟、类型安全 |
| 状态管理 | Zustand | 轻量、简单 |
| 布局引擎 | react-grid-layout | 可拖拽 Dashboard |
| UI 组件 | Radix UI + shadcn/ui | 现代化、可定制 |
| 数据库 | SQLite | 零配置、易迁移 |
| 通信 | REST + WebSocket | 简单直接 |

## 项目结构

```
cowork/
├── cmd/
│   ├── coordinator/       # Coordinator 主程序入口
│   └── worker/            # Worker 主程序入口
├── internal/
│   ├── coordinator/
│   │   ├── api/           # REST API 处理
│   │   ├── ws/            # WebSocket 处理
│   │   ├── scheduler/     # 任务调度器
│   │   ├── store/         # 数据存储层
│   │   ├── handler/       # 请求处理器
│   │   │   ├── agent.go   # Agent 对话路由
│   │   │   └── ...
│   │   └── system/        # 系统管理（Worker、通知）
│   ├── worker/
│   │   ├── executor/      # 任务执行器
│   │   ├── heartbeat/     # 心跳上报
│   │   └── isolate/       # 工作目录隔离
│   └── shared/
│       ├── protocol/      # 通信协议定义
│       └── models/        # 共享数据模型
├── web/                   # React 前端
│   ├── src/
│   │   ├── components/    # React 组件
│   │   │   ├── dashboard/ # Dashboard 组件
│   │   │   └── widgets/   # Widget 组件
│   │   ├── stores/        # Zustand Stores
│   │   ├── hooks/         # 自定义 Hooks
│   │   └── types/         # TypeScript 类型
│   └── public/
├── docker/                # Docker 配置
│   ├── Dockerfile.coordinator
│   ├── Dockerfile.worker
│   └── docker-compose.yml
├── docs/                  # 项目文档
│   ├── architecture.md
│   ├── api-design.md
│   ├── user-scenarios.md
│   ├── status.md
│   ├── roadmap.md
│   └── docker-implementation-plan.md
└── scripts/               # 构建脚本
```

## 快速开始

```bash
# 启动 Coordinator
go run ./cmd/coordinator

# 启动 Worker（另一个终端）
go run ./cmd/worker --name=worker-1 --tags=dev,coding --model=gpt-4

# 启动前端
cd web && npm run dev
```

## 文档

- [架构设计](./docs/architecture.md)
- [API 设计](./docs/api-design.md)
- [用户场景](./docs/user-scenarios.md)
- [功能状态](./docs/status.md)
- [开发路线图](./docs/roadmap.md)
- [Docker 实现计划](./docs/docker-implementation-plan.md)

## License

MIT