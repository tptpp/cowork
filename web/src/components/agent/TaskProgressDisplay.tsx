import { useEffect, useState } from 'react'
import {
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
  Play,
  Pause,
  AlertCircle,
  ChevronDown,
  ChevronRight,
  ExternalLink,
} from 'lucide-react'
import type { Task, TaskStatus } from '@/types'
import { cn } from '@/lib/utils'

// API base URL
const API_BASE = import.meta.env.VITE_API_BASE || ''

// Status configuration
const STATUS_CONFIG: Record<
  TaskStatus,
  {
    icon: React.ReactNode
    label: string
    color: string
    bgColor: string
  }
> = {
  pending: {
    icon: <Clock className="w-4 h-4" />,
    label: 'Pending',
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900/30',
  },
  running: {
    icon: <Loader2 className="w-4 h-4 animate-spin" />,
    label: 'Running',
    color: 'text-blue-600 dark:text-blue-400',
    bgColor: 'bg-blue-100 dark:bg-blue-900/30',
  },
  completed: {
    icon: <CheckCircle2 className="w-4 h-4" />,
    label: 'Completed',
    color: 'text-green-600 dark:text-green-400',
    bgColor: 'bg-green-100 dark:bg-green-900/30',
  },
  failed: {
    icon: <XCircle className="w-4 h-4" />,
    label: 'Failed',
    color: 'text-red-600 dark:text-red-400',
    bgColor: 'bg-red-100 dark:bg-red-900/30',
  },
  cancelled: {
    icon: <Pause className="w-4 h-4" />,
    label: 'Cancelled',
    color: 'text-gray-600 dark:text-gray-400',
    bgColor: 'bg-gray-100 dark:bg-gray-900/30',
  },
}

// Progress bar component
function ProgressBar({
  progress,
  status,
}: {
  progress: number
  status: TaskStatus
}) {
  const getBarColor = () => {
    switch (status) {
      case 'completed':
        return 'bg-green-500'
      case 'failed':
        return 'bg-red-500'
      case 'running':
        return 'bg-blue-500'
      case 'cancelled':
        return 'bg-gray-400'
      default:
        return 'bg-yellow-500'
    }
  }

  return (
    <div className="relative h-2 bg-muted rounded-full overflow-hidden">
      <div
        className={cn(
          'absolute inset-y-0 left-0 rounded-full transition-all duration-300',
          getBarColor()
        )}
        style={{ width: `${Math.min(100, Math.max(0, progress))}%` }}
      />
      {status === 'running' && (
        <div
          className={cn(
            'absolute inset-y-0 left-0 rounded-full animate-pulse',
            getBarColor(),
            'opacity-50'
          )}
          style={{ width: `${Math.min(100, Math.max(0, progress))}%` }}
        />
      )}
    </div>
  )
}

// Task card component
interface TaskCardProps {
  task: Task
  expanded?: boolean
  onToggleExpand?: () => void
  onClick?: () => void
}

export function TaskCard({
  task,
  expanded = false,
  onToggleExpand,
  onClick,
}: TaskCardProps) {
  const config = STATUS_CONFIG[task.status]

  return (
    <div
      className={cn(
        'border rounded-lg overflow-hidden transition-all',
        task.status === 'running' && 'border-blue-300 dark:border-blue-700',
        task.status === 'failed' && 'border-red-300 dark:border-red-700'
      )}
    >
      {/* Header */}
      <div
        className={cn(
          'flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-muted/50 transition-colors',
          config.bgColor
        )}
        onClick={onToggleExpand}
      >
        <div className={cn('flex-shrink-0', config.color)}>{config.icon}</div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-sm truncate">{task.type}</span>
            <span
              className={cn(
                'text-xs px-2 py-0.5 rounded-full',
                config.bgColor,
                config.color
              )}
            >
              {config.label}
            </span>
          </div>
          <p className="text-xs text-muted-foreground truncate mt-0.5">
            {task.description}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {task.progress > 0 && (
            <span className="text-xs text-muted-foreground">
              {task.progress}%
            </span>
          )}
          {onToggleExpand && (
            <div className="text-muted-foreground">
              {expanded ? (
                <ChevronDown className="w-4 h-4" />
              ) : (
                <ChevronRight className="w-4 h-4" />
              )}
            </div>
          )}
        </div>
      </div>

      {/* Progress bar */}
      {task.status === 'running' && (
        <div className="px-4 py-2 border-t">
          <ProgressBar progress={task.progress} status={task.status} />
        </div>
      )}

      {/* Expanded content */}
      {expanded && (
        <div className="border-t px-4 py-3 space-y-3 bg-muted/30">
          {/* Task ID */}
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Task ID</span>
            <code className="font-mono bg-muted px-2 py-0.5 rounded">
              {task.id}
            </code>
          </div>

          {/* Worker */}
          {task.worker_id && (
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">Worker</span>
              <code className="font-mono bg-muted px-2 py-0.5 rounded">
                {task.worker_id}
              </code>
            </div>
          )}

          {/* Tool name */}
          {task.tool_name && (
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">Tool</span>
              <code className="font-mono bg-muted px-2 py-0.5 rounded">
                {task.tool_name}
              </code>
            </div>
          )}

          {/* Priority */}
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Priority</span>
            <span
              className={cn(
                'px-2 py-0.5 rounded-full text-xs font-medium',
                task.priority === 'high' &&
                  'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
                task.priority === 'medium' &&
                  'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
                task.priority === 'low' &&
                  'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400'
              )}
            >
              {task.priority}
            </span>
          </div>

          {/* Input */}
          {task.input && Object.keys(task.input).length > 0 && (
            <div className="text-xs">
              <div className="text-muted-foreground mb-1">Input</div>
              <pre className="bg-muted p-2 rounded text-xs font-mono overflow-x-auto">
                {JSON.stringify(task.input, null, 2)}
              </pre>
            </div>
          )}

          {/* Output */}
          {task.output && Object.keys(task.output).length > 0 && (
            <div className="text-xs">
              <div className="text-muted-foreground mb-1">Output</div>
              <pre className="bg-muted p-2 rounded text-xs font-mono overflow-x-auto">
                {JSON.stringify(task.output, null, 2)}
              </pre>
            </div>
          )}

          {/* Error */}
          {task.error && (
            <div className="flex items-start gap-2 p-2 rounded bg-red-50 border border-red-200 dark:bg-red-900/20 dark:border-red-800">
              <AlertCircle className="w-4 h-4 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
              <div className="text-xs text-red-700 dark:text-red-300">
                {task.error}
              </div>
            </div>
          )}

          {/* Action button */}
          {onClick && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                onClick()
              }}
              className="flex items-center gap-1 text-xs text-primary hover:underline"
            >
              <ExternalLink className="w-3 h-3" />
              View Details
            </button>
          )}
        </div>
      )}
    </div>
  )
}

// Task progress list component
interface TaskProgressListProps {
  conversationId: string
  onTaskClick?: (taskId: string) => void
}

export function TaskProgressList({
  conversationId,
  onTaskClick,
}: TaskProgressListProps) {
  const [tasks, setTasks] = useState<Task[]>([])
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Fetch tasks for conversation
  const fetchTasks = async () => {
    if (!conversationId) return

    setIsLoading(true)
    setError(null)
    try {
      const response = await fetch(
        `${API_BASE}/api/agent/conversations/${conversationId}/tasks`
      )
      if (!response.ok) throw new Error('Failed to fetch tasks')

      const data = await response.json()
      setTasks(data.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch tasks')
    } finally {
      setIsLoading(false)
    }
  }

  // Initial fetch
  useEffect(() => {
    fetchTasks()
  }, [conversationId])

  // Poll for updates if there are running tasks
  useEffect(() => {
    const hasRunning = tasks.some((t) => t.status === 'running')
    if (!hasRunning) return

    const interval = setInterval(fetchTasks, 2000)
    return () => clearInterval(interval)
  }, [tasks])

  if (isLoading && tasks.length === 0) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center gap-2 p-4 rounded-lg bg-red-50 dark:bg-red-900/20 text-sm text-red-700 dark:text-red-300">
        <AlertCircle className="w-4 h-4" />
        {error}
      </div>
    )
  }

  if (tasks.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground text-sm">
        No tasks for this conversation
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {tasks.map((task) => (
        <TaskCard
          key={task.id}
          task={task}
          expanded={expandedId === task.id}
          onToggleExpand={() =>
            setExpandedId(expandedId === task.id ? null : task.id)
          }
          onClick={onTaskClick ? () => onTaskClick(task.id) : undefined}
        />
      ))}
    </div>
  )
}

// Mini task status for inline display
export function MiniTaskStatus({ task }: { task: Task }) {
  const config = STATUS_CONFIG[task.status]

  return (
    <div
      className={cn(
        'inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium',
        config.bgColor,
        config.color
      )}
    >
      {config.icon}
      <span>{config.label}</span>
      {task.status === 'running' && task.progress > 0 && (
        <span>({task.progress}%)</span>
      )}
    </div>
  )
}

// Task stats summary
export function TaskStatsSummary({ tasks }: { tasks: Task[] }) {
  const stats = {
    total: tasks.length,
    pending: tasks.filter((t) => t.status === 'pending').length,
    running: tasks.filter((t) => t.status === 'running').length,
    completed: tasks.filter((t) => t.status === 'completed').length,
    failed: tasks.filter((t) => t.status === 'failed').length,
  }

  if (stats.total === 0) return null

  return (
    <div className="flex items-center gap-3 text-xs text-muted-foreground">
      <span>{stats.total} tasks</span>
      {stats.running > 0 && (
        <span className="flex items-center gap-1 text-blue-600 dark:text-blue-400">
          <Loader2 className="w-3 h-3 animate-spin" />
          {stats.running} running
        </span>
      )}
      {stats.completed > 0 && (
        <span className="flex items-center gap-1 text-green-600 dark:text-green-400">
          <CheckCircle2 className="w-3 h-3" />
          {stats.completed} done
        </span>
      )}
      {stats.failed > 0 && (
        <span className="flex items-center gap-1 text-red-600 dark:text-red-400">
          <XCircle className="w-3 h-3" />
          {stats.failed} failed
        </span>
      )}
    </div>
  )
}