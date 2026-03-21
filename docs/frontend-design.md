# Cowork 前端设计文档

## 1. 技术栈

### 1.1 核心技术

| 技术 | 版本 | 用途 |
|------|------|------|
| React | 18.x | UI 框架 |
| TypeScript | 5.x | 类型安全 |
| Vite | 5.x | 构建工具 |
| Zustand | 4.x | 状态管理 |
| TanStack Query | 5.x | 数据获取 |
| react-grid-layout | 1.x | 拖拽布局 |
| Tailwind CSS | 3.x | 样式 |
| Shadcn/ui | latest | UI 组件库 |

### 1.2 项目结构

```
web/
├── public/
│   ├── favicon.ico
│   └── assets/
├── src/
│   ├── components/
│   │   ├── ui/                    # Shadcn/ui 组件
│   │   │   ├── button.tsx
│   │   │   ├── card.tsx
│   │   │   ├── dialog.tsx
│   │   │   └── ...
│   │   ├── layout/
│   │   │   ├── Header.tsx
│   │   │   ├── Sidebar.tsx
│   │   │   └── Layout.tsx
│   │   ├── dashboard/
│   │   │   ├── Dashboard.tsx      # Dashboard 容器
│   │   │   ├── WidgetStore.tsx    # Widget 商店
│   │   │   └── GridLayout.tsx     # 拖拽布局
│   │   └── widgets/
│   │       ├── index.ts           # Widget 导出
│   │       ├── TaskQueue/
│   │       │   ├── TaskQueue.tsx
│   │       │   ├── TaskCard.tsx
│   │       │   └── config.ts
│   │       ├── AgentChat/
│   │       │   ├── AgentChat.tsx
│   │       │   ├── MessageList.tsx
│   │       │   ├── InputBox.tsx
│   │       │   └── config.ts
│   │       ├── TodoList/
│   │       ├── TaskDetail/
│   │       ├── Notifications/
│   │       └── Stats/
│   ├── stores/
│   │   ├── taskStore.ts
│   │   ├── agentStore.ts
│   │   ├── systemStore.ts
│   │   ├── layoutStore.ts
│   │   └── index.ts
│   ├── hooks/
│   │   ├── useWebSocket.ts
│   │   ├── useTask.ts
│   │   ├── useAgent.ts
│   │   └── useLayout.ts
│   ├── services/
│   │   ├── api.ts                 # HTTP API 客户端
│   │   ├── websocket.ts           # WebSocket 客户端
│   │   └── storage.ts             # 本地存储
│   ├── types/
│   │   ├── task.ts
│   │   ├── agent.ts
│   │   ├── system.ts
│   │   ├── widget.ts
│   │   └── api.ts
│   ├── utils/
│   │   ├── format.ts
│   │   ├── date.ts
│   │   └── cn.ts                  # className 合并
│   ├── styles/
│   │   └── globals.css
│   ├── App.tsx
│   └── main.tsx
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
└── tailwind.config.js
```

---

## 2. 类型定义

### 2.1 任务类型

```typescript
// types/task.ts

export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
export type Priority = 'low' | 'medium' | 'high';

export interface Task {
  id: string;
  type: string;
  description: string;
  status: TaskStatus;
  progress: number;           // 0-100
  priority: Priority;
  
  worker_id: string | null;
  required_tags: string[];

  input: Record<string, unknown>;
  output: Record<string, unknown> | null;
  error: string | null;
  
  config: TaskConfig;
  work_dir: string;
  
  created_at: string;
  updated_at: string;
  started_at: string | null;
  completed_at: string | null;
  
  files?: TaskFile[];
}

export interface TaskConfig {
  timeout?: number;
  retry?: number;
  notify_on_complete?: boolean;
}

export interface TaskFile {
  id: number;
  name: string;
  size: number;
  mime_type: string;
}

export interface TaskLog {
  id: number;
  task_id: string;
  time: string;
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  metadata?: Record<string, unknown>;
}

export interface CreateTaskInput {
  type: string;
  description?: string;
  priority?: Priority;
  required_tags?: string[];
  input?: Record<string, unknown>;
  config?: TaskConfig;
}
```

### 2.2 Worker 类型

```typescript
// types/system.ts

export type WorkerStatus = 'idle' | 'busy' | 'offline' | 'error';

export interface Worker {
  id: string;
  name: string;
  tags: string[];
  model: string;
  status: WorkerStatus;
  max_concurrent: number;
  
  capabilities: {
    docker?: boolean;
    gpu?: boolean;
    work_dir?: string;
  };
  
  metadata: {
    hostname?: string;
    os?: string;
    version?: string;
  };
  
  completed_tasks: number;
  failed_tasks: number;
  
  created_at: string;
  last_seen: string;
  
  current_tasks?: { id: string; progress: number }[];
}

export interface Notification {
  id: number;
  type: string;
  title: string;
  message: string;
  data?: Record<string, unknown>;
  read: boolean;
  created_at: string;
}

export interface SystemStats {
  tasks: {
    total: number;
    pending: number;
    running: number;
    completed: number;
    failed: number;
  };
  workers: {
    total: number;
    online: number;
    offline: number;
  };
  system: {
    uptime: string;
    version: string;
    go_version: string;
  };
}
```

### 2.3 Agent 类型

```typescript
// types/agent.ts

export interface AgentSession {
  id: string;
  model: string;
  system_prompt?: string;
  context?: Record<string, unknown>;
  task_id?: string;
  config?: AgentConfig;
  created_at: string;
  updated_at: string;
  messages?: AgentMessage[];
}

export interface AgentMessage {
  id: number;
  session_id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  tokens?: number;
  created_at: string;
}

export interface AgentConfig {
  temperature?: number;
  max_tokens?: number;
  top_p?: number;
}

export interface ChatInput {
  session_id: string;
  message: string;
  files?: string[];
  stream?: boolean;
}
```

### 2.4 Widget 类型

```typescript
// types/widget.ts

export type WidgetType = 
  | 'task-queue'
  | 'task-detail'
  | 'agent-chat'
  | 'todo-list'
  | 'notifications'
  | 'stats'
  | 'worker-list'
  | 'file-preview';

export interface GridLayoutItem {
  x: number;
  y: number;
  w: number;
  h: number;
  minW?: number;
  minH?: number;
  maxW?: number;
  maxH?: number;
  static?: boolean;
}

export interface WidgetConfig {
  id: string;
  type: WidgetType;
  title: string;
  layout: GridLayoutItem;
  settings: Record<string, unknown>;
}

export interface WidgetDefinition {
  type: WidgetType;
  name: string;
  icon: string;
  description: string;
  category: 'system' | 'agent' | 'personal';
  defaultSize: { w: number; h: number };
  settingsSchema: SettingSchema[];
}

export interface SettingSchema {
  key: string;
  type: 'string' | 'number' | 'boolean' | 'select' | 'task-select' | 'multi-select';
  label: string;
  default?: unknown;
  options?: { label: string; value: unknown }[];
  min?: number;
  max?: number;
}
```

---

## 3. 状态管理

### 3.1 TaskStore

```typescript
// stores/taskStore.ts
import { create } from 'zustand';
import { devtools, persist } from 'zustand/middleware';
import type { Task, CreateTaskInput, TaskFilter } from '@/types/task';

interface TaskState {
  tasks: Task[];
  selectedTask: Task | null;
  filters: TaskFilter;
  
  // Actions
  setTasks: (tasks: Task[]) => void;
  addTask: (task: Task) => void;
  updateTask: (id: string, updates: Partial<Task>) => void;
  removeTask: (id: string) => void;
  selectTask: (task: Task | null) => void;
  setFilters: (filters: TaskFilter) => void;
  
  // WebSocket handlers
  onTaskUpdate: (task: Task) => void;
  onTaskLog: (taskId: string, log: TaskLog) => void;
}

export const useTaskStore = create<TaskState>()(
  devtools(
    persist(
      (set, get) => ({
        tasks: [],
        selectedTask: null,
        filters: {},
        
        setTasks: (tasks) => set({ tasks }),
        
        addTask: (task) => set((state) => ({
          tasks: [task, ...state.tasks]
        })),
        
        updateTask: (id, updates) => set((state) => ({
          tasks: state.tasks.map((t) =>
            t.id === id ? { ...t, ...updates } : t
          ),
          selectedTask: state.selectedTask?.id === id
            ? { ...state.selectedTask, ...updates }
            : state.selectedTask,
        })),
        
        removeTask: (id) => set((state) => ({
          tasks: state.tasks.filter((t) => t.id !== id),
          selectedTask: state.selectedTask?.id === id ? null : state.selectedTask,
        })),
        
        selectTask: (task) => set({ selectedTask: task }),
        
        setFilters: (filters) => set({ filters }),
        
        onTaskUpdate: (task) => {
          const exists = get().tasks.some((t) => t.id === task.id);
          if (exists) {
            get().updateTask(task.id, task);
          } else {
            get().addTask(task);
          }
        },
        
        onTaskLog: (taskId, log) => {
          // 可以选择存储日志或直接忽略（由 TaskDetail Widget 自己处理）
        },
      }),
      { name: 'cowork-tasks' }
    )
  )
);
```

### 3.2 AgentStore

```typescript
// stores/agentStore.ts
import { create } from 'zustand';
import type { AgentSession, AgentMessage } from '@/types/agent';

interface AgentState {
  sessions: AgentSession[];
  currentSession: AgentSession | null;
  streamingMessage: string;
  isStreaming: boolean;
  
  // Actions
  setSessions: (sessions: AgentSession[]) => void;
  addSession: (session: AgentSession) => void;
  setCurrentSession: (session: AgentSession | null) => void;
  addMessage: (sessionId: string, message: AgentMessage) => void;
  
  // Streaming
  startStreaming: () => void;
  appendChunk: (chunk: string) => void;
  finishStreaming: (message: AgentMessage) => void;
}

export const useAgentStore = create<AgentState>((set, get) => ({
  sessions: [],
  currentSession: null,
  streamingMessage: '',
  isStreaming: false,
  
  setSessions: (sessions) => set({ sessions }),
  
  addSession: (session) => set((state) => ({
    sessions: [...state.sessions, session],
  })),
  
  setCurrentSession: (session) => set({ 
    currentSession: session,
    streamingMessage: '',
    isStreaming: false,
  }),
  
  addMessage: (sessionId, message) => set((state) => ({
    sessions: state.sessions.map((s) =>
      s.id === sessionId
        ? { ...s, messages: [...(s.messages || []), message] }
        : s
    ),
    currentSession: state.currentSession?.id === sessionId
      ? { ...state.currentSession, messages: [...(state.currentSession.messages || []), message] }
      : state.currentSession,
  })),
  
  startStreaming: () => set({ isStreaming: true, streamingMessage: '' }),
  
  appendChunk: (chunk) => set((state) => ({
    streamingMessage: state.streamingMessage + chunk,
  })),
  
  finishStreaming: (message) => {
    get().addMessage(message.session_id, message);
    set({ isStreaming: false, streamingMessage: '' });
  },
}));
```

### 3.3 LayoutStore

```typescript
// stores/layoutStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { WidgetConfig, WidgetType } from '@/types/widget';

interface LayoutState {
  widgets: WidgetConfig[];
  isEditing: boolean;
  
  // Actions
  setWidgets: (widgets: WidgetConfig[]) => void;
  addWidget: (type: WidgetType) => void;
  updateWidget: (id: string, updates: Partial<WidgetConfig>) => void;
  removeWidget: (id: string) => void;
  updateLayout: (id: string, layout: GridLayoutItem) => void;
  toggleEditing: () => void;
  
  // Persistence
  saveLayout: () => Promise<void>;
  loadLayout: () => Promise<void>;
}

export const useLayoutStore = create<LayoutState>()(
  persist(
    (set, get) => ({
      widgets: [],
      isEditing: false,
      
      setWidgets: (widgets) => set({ widgets }),
      
      addWidget: (type) => {
        const definition = WIDGET_REGISTRY[type];
        const id = `widget-${Date.now()}`;
        const widget: WidgetConfig = {
          id,
          type,
          title: definition.name,
          layout: { x: 0, y: 0, ...definition.defaultSize },
          settings: definition.settingsSchema.reduce((acc, s) => ({
            ...acc,
            [s.key]: s.default,
          }), {}),
        };
        set((state) => ({ widgets: [...state.widgets, widget] }));
      },
      
      updateWidget: (id, updates) => set((state) => ({
        widgets: state.widgets.map((w) =>
          w.id === id ? { ...w, ...updates } : w
        ),
      })),
      
      removeWidget: (id) => set((state) => ({
        widgets: state.widgets.filter((w) => w.id !== id),
      })),
      
      updateLayout: (id, layout) => set((state) => ({
        widgets: state.widgets.map((w) =>
          w.id === id ? { ...w, layout } : w
        ),
      })),
      
      toggleEditing: () => set((state) => ({ isEditing: !state.isEditing })),
      
      saveLayout: async () => {
        const { widgets } = get();
        await fetch('/api/user/layout', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ widgets }),
        });
      },
      
      loadLayout: async () => {
        const res = await fetch('/api/user/layout');
        const { data } = await res.json();
        if (data?.widgets) {
          set({ widgets: data.widgets });
        }
      },
    }),
    { name: 'cowork-layout' }
  )
);
```

---

## 4. Widget 系统

### 4.1 Widget 注册表

```typescript
// components/widgets/index.ts
import type { WidgetDefinition } from '@/types/widget';

export const WIDGET_REGISTRY: Record<string, WidgetDefinition> = {
  'task-queue': {
    type: 'task-queue',
    name: '任务队列',
    icon: '📊',
    description: '显示运行中和排队的任务',
    category: 'system',
    defaultSize: { w: 4, h: 3 },
    settingsSchema: [
      { key: 'maxTasks', type: 'number', label: '最大显示数', default: 10, min: 1, max: 50 },
      { key: 'autoRefresh', type: 'number', label: '刷新间隔(秒)', default: 30, min: 5, max: 300 },
      { key: 'showProgress', type: 'boolean', label: '显示进度条', default: true },
    ],
  },
  
  'agent-chat': {
    type: 'agent-chat',
    name: 'Agent 对话',
    icon: '🤖',
    description: '与 AI Agent 对话',
    category: 'agent',
    defaultSize: { w: 4, h: 4 },
    settingsSchema: [
      { key: 'model', type: 'select', label: '模型', options: [
        { label: 'GPT-4', value: 'gpt-4' },
        { label: 'Claude 3', value: 'claude-3' },
        { label: 'GLM-4', value: 'glm-4' },
      ]},
      { key: 'sessionId', type: 'string', label: '会话ID（留空创建新会话）' },
    ],
  },
  
  'todo-list': {
    type: 'todo-list',
    name: 'Todo List',
    icon: '📋',
    description: '待办事项列表',
    category: 'personal',
    defaultSize: { w: 3, h: 3 },
    settingsSchema: [
      { key: 'showCompleted', type: 'boolean', label: '显示已完成', default: true },
      { key: 'maxItems', type: 'number', label: '最大显示数', default: 10 },
    ],
  },
  
  'notifications': {
    type: 'notifications',
    name: '通知中心',
    icon: '🔔',
    description: '系统通知列表',
    category: 'system',
    defaultSize: { w: 3, h: 3 },
    settingsSchema: [
      { key: 'maxItems', type: 'number', label: '最大显示数', default: 10 },
      { key: 'unreadOnly', type: 'boolean', label: '只显示未读', default: false },
    ],
  },
  
  'stats': {
    type: 'stats',
    name: '统计面板',
    icon: '📈',
    description: '系统和任务统计',
    category: 'system',
    defaultSize: { w: 4, h: 2 },
    settingsSchema: [],
  },
};

// 懒加载 Widget 组件
export const WidgetComponents = {
  'task-queue': lazy(() => import('./TaskQueue/TaskQueue')),
  'agent-chat': lazy(() => import('./AgentChat/AgentChat')),
  'todo-list': lazy(() => import('./TodoList/TodoList')),
  'notifications': lazy(() => import('./Notifications/Notifications')),
  'stats': lazy(() => import('./Stats/Stats')),
};
```

### 4.2 Widget Wrapper

```tsx
// components/dashboard/WidgetWrapper.tsx
import { useState } from 'react';
import { Card, CardHeader, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Settings, X } from 'lucide-react';
import type { WidgetConfig } from '@/types/widget';
import { useLayoutStore } from '@/stores/layoutStore';

interface WidgetWrapperProps {
  widget: WidgetConfig;
  children: React.ReactNode;
}

export function WidgetWrapper({ widget, children }: WidgetWrapperProps) {
  const { isEditing, removeWidget, updateWidget } = useLayoutStore();
  const [showConfig, setShowConfig] = useState(false);
  
  return (
    <Card className="h-full flex flex-col">
      <CardHeader className="flex-row items-center justify-between py-2 px-4 space-y-0">
        <span className="font-medium text-sm">
          {WIDGET_REGISTRY[widget.type].icon} {widget.title}
        </span>
        
        {isEditing && (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              onClick={() => setShowConfig(!showConfig)}
            >
              <Settings className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              onClick={() => removeWidget(widget.id)}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        )}
      </CardHeader>
      
      <CardContent className="flex-1 overflow-auto">
        {showConfig ? (
          <WidgetConfigPanel
            widget={widget}
            onClose={() => setShowConfig(false)}
          />
        ) : (
          children
        )}
      </CardContent>
    </Card>
  );
}
```

### 4.3 Widget 示例：TaskQueue

```tsx
// components/widgets/TaskQueue/TaskQueue.tsx
import { useTaskStore } from '@/stores/taskStore';
import { TaskCard } from './TaskCard';
import type { WidgetProps } from '@/types/widget';

interface TaskQueueSettings {
  maxTasks: number;
  autoRefresh: boolean;
  showProgress: boolean;
}

export function TaskQueue({ settings }: WidgetProps<TaskQueueSettings>) {
  const tasks = useTaskStore((state) =>
    state.tasks
      .filter((t) => t.status === 'running' || t.status === 'pending')
      .slice(0, settings.maxTasks || 10)
  );
  
  if (tasks.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        暂无运行中的任务
      </div>
    );
  }
  
  return (
    <div className="space-y-2">
      {tasks.map((task) => (
        <TaskCard
          key={task.id}
          task={task}
          showProgress={settings.showProgress}
        />
      ))}
    </div>
  );
}
```

```tsx
// components/widgets/TaskQueue/TaskCard.tsx
import { Progress } from '@/components/ui/progress';
import type { Task } from '@/types/task';

interface TaskCardProps {
  task: Task;
  showProgress?: boolean;
}

export function TaskCard({ task, showProgress = true }: TaskCardProps) {
  const statusColor = {
    pending: 'bg-yellow-500',
    running: 'bg-blue-500',
    completed: 'bg-green-500',
    failed: 'bg-red-500',
    cancelled: 'bg-gray-500',
  }[task.status];
  
  return (
    <div className="p-3 border rounded-lg">
      <div className="flex items-center justify-between mb-2">
        <span className="font-medium text-sm truncate">{task.id}</span>
        <span className={`px-2 py-0.5 text-xs rounded text-white ${statusColor}`}>
          {task.status}
        </span>
      </div>
      
      <p className="text-xs text-muted-foreground truncate mb-2">
        {task.description || task.type}
      </p>
      
      {showProgress && task.status === 'running' && (
        <Progress value={task.progress} className="h-1" />
      )}
    </div>
  );
}
```

### 4.4 Widget 示例：AgentChat

```tsx
// components/widgets/AgentChat/AgentChat.tsx
import { useState, useRef, useEffect } from 'react';
import { useAgentStore } from '@/stores/agentStore';
import { MessageList } from './MessageList';
import { InputBox } from './InputBox';
import type { WidgetProps } from '@/types/widget';

interface AgentChatSettings {
  model: string;
  sessionId?: string;
}

export function AgentChat({ settings }: WidgetProps<AgentChatSettings>) {
  const { currentSession, streamingMessage, isStreaming, setCurrentSession, addSession } = useAgentStore();
  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  
  // 初始化会话
  useEffect(() => {
    if (settings.sessionId) {
      // 加载现有会话
      fetch(`/api/agent/sessions/${settings.sessionId}`)
        .then((res) => res.json())
        .then((data) => setCurrentSession(data.data));
    } else if (!currentSession) {
      // 创建新会话
      fetch('/api/agent/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ model: settings.model }),
      })
        .then((res) => res.json())
        .then((data) => {
          addSession(data.data);
          setCurrentSession(data.data);
        });
    }
  }, [settings.sessionId, settings.model]);
  
  // 自动滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [currentSession?.messages, streamingMessage]);
  
  const handleSend = async () => {
    if (!input.trim() || !currentSession) return;
    
    const message = input;
    setInput('');
    
    // 发送消息（流式响应通过 WebSocket 接收）
    await fetch('/api/agent/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: currentSession.id,
        message,
        stream: true,
      }),
    });
  };
  
  return (
    <div className="flex flex-col h-full">
      <MessageList
        messages={currentSession?.messages || []}
        streamingMessage={streamingMessage}
        isStreaming={isStreaming}
      />
      <div ref={messagesEndRef} />
      <InputBox
        value={input}
        onChange={setInput}
        onSend={handleSend}
        disabled={isStreaming}
      />
    </div>
  );
}
```

---

## 5. Dashboard 组件

### 5.1 Dashboard 主组件

```tsx
// components/dashboard/Dashboard.tsx
import { Responsive, WidthProvider } from 'react-grid-layout';
import { useLayoutStore } from '@/stores/layoutStore';
import { WidgetWrapper } from './WidgetWrapper';
import { WidgetComponents, WIDGET_REGISTRY } from '@/components/widgets';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';

const ResponsiveGridLayout = WidthProvider(Responsive);

export function Dashboard() {
  const { widgets, isEditing, updateLayout, saveLayout } = useLayoutStore();
  
  const layouts = {
    lg: widgets.map((w) => ({
      i: w.id,
      ...w.layout,
    })),
  };
  
  const handleLayoutChange = (layout: any[]) => {
    layout.forEach((l) => {
      updateLayout(l.i, { x: l.x, y: l.y, w: l.w, h: l.h });
    });
    saveLayout();
  };
  
  return (
    <div className="h-full p-4">
      <ResponsiveGridLayout
        className="layout"
        layouts={layouts}
        breakpoints={{ lg: 1200, md: 996, sm: 768, xs: 480, xxs: 0 }}
        cols={{ lg: 12, md: 10, sm: 6, xs: 4, xxs: 2 }}
        rowHeight={80}
        isDraggable={isEditing}
        isResizable={isEditing}
        onLayoutChange={handleLayoutChange}
      >
        {widgets.map((widget) => (
          <div key={widget.id}>
            <WidgetWrapper widget={widget}>
              <WidgetRenderer widget={widget} />
            </WidgetWrapper>
          </div>
        ))}
      </ResponsiveGridLayout>
      
      {widgets.length === 0 && (
        <div className="flex items-center justify-center h-full text-muted-foreground">
          点击右上角 "添加 Widget" 开始配置你的 Dashboard
        </div>
      )}
    </div>
  );
}

function WidgetRenderer({ widget }: { widget: WidgetConfig }) {
  const Component = WidgetComponents[widget.type];
  
  if (!Component) {
    return <div>Unknown widget type: {widget.type}</div>;
  }
  
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <Component settings={widget.settings} />
    </Suspense>
  );
}
```

### 5.2 Widget 商店

```tsx
// components/dashboard/WidgetStore.tsx
import { useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { useLayoutStore } from '@/stores/layoutStore';
import { WIDGET_REGISTRY } from '@/components/widgets';
import type { WidgetType } from '@/types/widget';

interface WidgetStoreProps {
  open: boolean;
  onClose: () => void;
}

export function WidgetStore({ open, onClose }: WidgetStoreProps) {
  const [search, setSearch] = useState('');
  const { addWidget } = useLayoutStore();
  
  const widgets = Object.values(WIDGET_REGISTRY).filter((w) =>
    w.name.toLowerCase().includes(search.toLowerCase()) ||
    w.description.toLowerCase().includes(search.toLowerCase())
  );
  
  const handleAdd = (type: WidgetType) => {
    addWidget(type);
    onClose();
  };
  
  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>添加 Widget</DialogTitle>
        </DialogHeader>
        
        <Input
          placeholder="搜索 Widget..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="mb-4"
        />
        
        <div className="grid grid-cols-2 gap-4 max-h-96 overflow-auto">
          {widgets.map((widget) => (
            <div
              key={widget.type}
              className="p-4 border rounded-lg hover:bg-accent cursor-pointer"
              onClick={() => handleAdd(widget.type)}
            >
              <div className="flex items-center gap-2 mb-2">
                <span className="text-2xl">{widget.icon}</span>
                <span className="font-medium">{widget.name}</span>
              </div>
              <p className="text-sm text-muted-foreground">
                {widget.description}
              </p>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

---

## 6. WebSocket 集成

### 6.1 WebSocket Hook

```typescript
// hooks/useWebSocket.ts
import { useEffect, useRef, useCallback } from 'react';
import { useTaskStore } from '@/stores/taskStore';
import { useAgentStore } from '@/stores/agentStore';
import { useSystemStore } from '@/stores/systemStore';

type Channel = string;

export function useWebSocket(url: string = 'ws://localhost:8080/ws') {
  const wsRef = useRef<WebSocket | null>(null);
  const subscriptionsRef = useRef<Set<Channel>>(new Set());
  
  const { onTaskUpdate } = useTaskStore();
  const { addMessage, startStreaming, appendChunk, finishStreaming } = useAgentStore();
  const { onNotification, onWorkerUpdate } = useSystemStore();
  
  const connect = useCallback(() => {
    const ws = new WebSocket(url);
    wsRef.current = ws;
    
    ws.onopen = () => {
      console.log('WebSocket connected');
      // 重新订阅之前的频道
      if (subscriptionsRef.current.size > 0) {
        ws.send(JSON.stringify({
          type: 'subscribe',
          channels: Array.from(subscriptionsRef.current),
        }));
      }
    };
    
    ws.onmessage = (event) => {
      const { type, payload } = JSON.parse(event.data);
      
      switch (type) {
        case 'task_update':
          onTaskUpdate(payload);
          break;
        case 'agent_chunk':
          appendChunk(payload.chunk);
          break;
        case 'agent_done':
          finishStreaming(payload.message);
          break;
        case 'notification':
          onNotification(payload);
          break;
        case 'worker_update':
          onWorkerUpdate(payload);
          break;
        case 'pong':
          // Heartbeat response
          break;
      }
    };
    
    ws.onclose = () => {
      console.log('WebSocket disconnected, reconnecting...');
      setTimeout(connect, 3000);
    };
    
    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }, [url, onTaskUpdate, appendChunk, finishStreaming, onNotification, onWorkerUpdate]);
  
  useEffect(() => {
    connect();
    
    // Heartbeat
    const heartbeat = setInterval(() => {
      wsRef.current?.send(JSON.stringify({ type: 'ping' }));
    }, 30000);
    
    return () => {
      clearInterval(heartbeat);
      wsRef.current?.close();
    };
  }, [connect]);
  
  const subscribe = useCallback((channels: Channel[]) => {
    channels.forEach((c) => subscriptionsRef.current.add(c));
    wsRef.current?.send(JSON.stringify({
      type: 'subscribe',
      channels,
    }));
  }, []);
  
  const unsubscribe = useCallback((channels: Channel[]) => {
    channels.forEach((c) => subscriptionsRef.current.delete(c));
    wsRef.current?.send(JSON.stringify({
      type: 'unsubscribe',
      channels,
    }));
  }, []);
  
  return { subscribe, unsubscribe };
}
```

---

## 7. 样式设计

### 7.1 Tailwind 配置

```javascript
// tailwind.config.js
module.exports = {
  darkMode: ['class'],
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
};
```

### 7.2 全局样式

```css
/* styles/globals.css */
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --card: 0 0% 100%;
    --card-foreground: 222.2 84% 4.9%;
    --primary: 222.2 47.4% 11.2%;
    --primary-foreground: 210 40% 98%;
    --secondary: 210 40% 96.1%;
    --secondary-foreground: 222.2 47.4% 11.2%;
    --muted: 210 40% 96.1%;
    --muted-foreground: 215.4 16.3% 46.9%;
    --accent: 210 40% 96.1%;
    --accent-foreground: 222.2 47.4% 11.2%;
    --border: 214.3 31.8% 91.4%;
    --ring: 222.2 84% 4.9%;
  }

  .dark {
    --background: 222.2 84% 4.9%;
    --foreground: 210 40% 98%;
    --card: 222.2 84% 4.9%;
    --card-foreground: 210 40% 98%;
    --primary: 210 40% 98%;
    --primary-foreground: 222.2 47.4% 11.2%;
    --secondary: 217.2 32.6% 17.5%;
    --secondary-foreground: 210 40% 98%;
    --muted: 217.2 32.6% 17.5%;
    --muted-foreground: 215 20.2% 65.1%;
    --accent: 217.2 32.6% 17.5%;
    --accent-foreground: 210 40% 98%;
    --border: 217.2 32.6% 17.5%;
    --ring: 212.7 26.8% 83.9%;
  }
}
```

---

## 8. 构建配置

### 8.1 Vite 配置

```typescript
// vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
});
```

### 8.2 Package.json

```json
{
  "name": "cowork-web",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "lint": "eslint src --ext ts,tsx"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "zustand": "^4.4.0",
    "react-grid-layout": "^1.4.0",
    "lucide-react": "^0.300.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.2.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.2.0",
    "typescript": "^5.3.0",
    "vite": "^5.0.0",
    "tailwindcss": "^3.4.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0"
  }
}
```