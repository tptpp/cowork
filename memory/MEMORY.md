# Cowork Project Memory

## Project Overview
Distributed task processing system with React frontend and Go backend. Now an AI Agent platform with unified Agent architecture and Function Calling capabilities.

## Tech Stack
- Frontend: React 19 + TypeScript + Vite + Tailwind CSS + Zustand
- UI: Radix UI components + shadcn/ui style
- Grid Layout: react-grid-layout
- Backend: Go with WebSocket support, Gin framework
- Database: SQLite (GORM)
- AI: OpenAI/Anthropic/GLM API with SSE streaming

## Architecture (Updated 2026-04-22)

### Three-Layer Architecture

```
┌─────────────────────────────────────────────┐
│              Agent Layer                      │
│  - Unified Agent structure                   │
│  - Coordinator + Workers share same pattern  │
│  - Core: call model → execute tools          │
└─────────────────────────────────────────────┘
                    ↓ calls tools
┌─────────────────────────────────────────────┐
│            System Services Layer              │
│  - TaskScheduler: dependency-based dispatch  │
│  - NodeAssigner: capability matching         │
│  - MessageRouter: agent messaging + recovery │
│  - ContextInjector: context preprocessing    │
│  - ProgressMonitor: status tracking          │
│  (Fixed rules, no model calls)              │
└─────────────────────────────────────────────┘
                    ↓ operates data
┌─────────────────────────────────────────────┐
│              Data Layer                       │
│  - agents table: tasks + agents unified      │
│  - nodes table: node registry                │
│  - messages table: agent communication       │
│  - agent_templates: template definitions     │
│  - workspace: working directories            │
└─────────────────────────────────────────────┘
```

### Agent Architecture Simplification (2026-04-16)

Simplified from ~4000 lines to ~1200 lines (70% reduction):
- **agent.go**: Unified Agent structure (~466 lines)
- **llm_client.go**: API calling details (~496 lines)
- **template.go**: Template management (~100 lines)

Deleted components:
- TaskDecomposer → replaced by Agent calling create_task tool
- ToolScheduler → replaced by direct Worker mechanism
- FunctionCallingEngine → merged into Agent + LLMClient

### System Services
- **ContextInjector**: Injects context into messages for Coordinator
- **ProgressMonitor**: Monitors task status, pushes to frontend
- **MessageRouter**: Routes agent messages, handles recovery agents

## Current Status (2026-04-22)

### Completed Features
- Phase 1-5: All core features complete
- Agent Architecture Simplification: Complete (10 tasks across 3 phases)
- Function Calling: Full support with Human-in-loop

### Key Files
- `internal/coordinator/agent/agent.go` - Unified Agent structure
- `internal/coordinator/agent/llm_client.go` - LLM API client
- `internal/coordinator/agent/coordinator.go` - Coordinator wrapper
- `internal/coordinator/service/` - System services layer
- `web/src/components/widgets/AgentChatWidget.tsx` - AI chat interface

### Built-in Tools (6 tools)
1. `execute_shell` - Execute shell commands (remote)
2. `read_file` - Read file contents (local, with path traversal protection)
3. `write_file` - Write file contents (remote)
4. `create_task` - Create async tasks (local)
5. `query_task` - Query task status (local)
6. `request_approval` - Human-in-loop approval (local)

## Design Documents
- `docs/superpowers/specs/2026-04-16-agent-architecture-simplification-design.md` - Architecture simplification design
- `docs/superpowers/plans/2026-04-16-agent-architecture-simplification.md` - Implementation plan
- `docs/architecture.md` - Overall system architecture

## Related Tools
- [Web 视觉测试 Agent](web-visual-test-agent.md) - Frontend screenshot and visual analysis

# currentDate
Today's date is 2026/04/22.