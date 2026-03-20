// Task types
export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
export type Priority = 'low' | 'medium' | 'high'

export interface Task {
  id: string
  type: string
  description: string
  status: TaskStatus
  progress: number
  priority: Priority
  worker_id: string | null
  required_tags: string[]
  preferred_model: string
  input: Record<string, unknown>
  output: Record<string, unknown>
  error: string | null
  config: Record<string, unknown>
  work_dir: string
  created_at: string
  updated_at: string
  started_at: string | null
  completed_at: string | null
}

// Worker types
export type WorkerStatus = 'idle' | 'busy' | 'offline' | 'error'

export interface Worker {
  id: string
  name: string
  tags: string[]
  model: string
  status: WorkerStatus
  max_concurrent: number
  capabilities: Record<string, unknown>
  metadata: Record<string, unknown>
  completed_tasks: number
  failed_tasks: number
  created_at: string
  last_seen: string
}

// System stats
export interface TaskStats {
  total: number
  pending: number
  running: number
  completed: number
  failed: number
}

export interface WorkerStats {
  total: number
  online: number
  offline: number
}

export interface SystemInfo {
  uptime: string
  version: string
  go_version: string
}

export interface SystemStats {
  tasks: TaskStats
  workers: WorkerStats
  system: SystemInfo
}

// WebSocket message types
export type MessageType =
  | 'subscribe'
  | 'unsubscribe'
  | 'ping'
  | 'pong'
  | 'task_update'
  | 'task_log'
  | 'worker_update'
  | 'notification'
  | 'error'

export interface WSMessage {
  type: MessageType
  payload?: unknown
  channels?: string[]
}

// Widget types
export type WidgetType =
  | 'system-stats'
  | 'task-queue'
  | 'worker-status'
  | 'agent-chat'
  | 'todo-list'

export interface GridLayoutItem {
  x: number
  y: number
  w: number
  h: number
  minW?: number
  minH?: number
  maxW?: number
  maxH?: number
}

export interface WidgetConfig {
  id: string
  type: WidgetType
  title: string
  layout: GridLayoutItem
}

export interface WidgetDefinition {
  type: WidgetType
  name: string
  icon: string
  description: string
  defaultSize: { w: number; h: number }
  minSize?: { w: number; h: number }
}

// Agent types
export interface AgentSession {
  id: string
  model: string
  system_prompt: string
  context: Record<string, unknown>
  task_id: string | null
  config: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface AgentMessage {
  id: number
  session_id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  tokens: number
  created_at: string
}

// Notification types
export interface Notification {
  id: number
  type: string
  title: string
  message: string
  data?: Record<string, unknown>
  read: boolean
  created_at: string
}