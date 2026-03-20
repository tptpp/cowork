import { Loader2, Clock, Zap } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { useTaskStore } from '@/stores/taskStore'
import type { Task } from '@/types'
import { cn } from '@/lib/utils'

interface TaskQueueWidgetProps {
  onSelectTask?: (task: Task) => void
}

export function TaskQueueWidget({ onSelectTask }: TaskQueueWidgetProps) {
  const { tasks } = useTaskStore()

  // Filter running and pending tasks
  const activeTasks = tasks.filter(
    (task) => task.status === 'running' || task.status === 'pending'
  )

  if (activeTasks.length === 0) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-muted-foreground p-4">
        <div className="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mb-3">
          <Clock className="w-6 h-6 opacity-50" />
        </div>
        <p className="text-sm font-medium">No active tasks</p>
        <p className="text-xs text-muted-foreground">Queue is empty</p>
      </div>
    )
  }

  return (
    <div className="h-full flex flex-col p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Zap className="w-4 h-4 text-primary" />
          <span className="text-sm font-medium">Active Tasks</span>
        </div>
        <Badge variant="secondary" className="text-xs">
          {activeTasks.length} active
        </Badge>
      </div>

      {/* Task List */}
      <div className="flex-1 overflow-auto space-y-2">
        {activeTasks.map((task) => (
          <div
            key={task.id}
            onClick={() => onSelectTask?.(task)}
            className={cn(
              'flex items-center gap-3 p-3 rounded-xl border border-border/50 bg-muted/30',
              'hover:bg-muted/50 hover:border-primary/30 cursor-pointer transition-all',
              'active:scale-[0.98]'
            )}
          >
            <div className={cn(
              'p-2 rounded-lg',
              task.status === 'running' ? 'bg-primary/10' : 'bg-muted'
            )}>
              {task.status === 'running' ? (
                <Loader2 className="w-4 h-4 animate-spin text-primary" />
              ) : (
                <Clock className="w-4 h-4 text-muted-foreground" />
              )}
            </div>

            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-medium text-sm truncate">{task.type}</span>
                {task.status === 'running' && (
                  <Badge variant="default" className="text-xs px-1.5 py-0 h-4">
                    running
                  </Badge>
                )}
              </div>
              <p className="text-xs text-muted-foreground truncate">
                {task.description || `ID: ${task.id.slice(0, 8)}`}
              </p>
            </div>

            {task.status === 'running' && (
              <div className="flex items-center gap-2">
                <div className="w-14 bg-muted rounded-full h-1.5 overflow-hidden">
                  <div
                    className="bg-primary h-1.5 rounded-full transition-all"
                    style={{ width: `${task.progress}%` }}
                  />
                </div>
                <span className="text-xs text-muted-foreground w-7 text-right font-medium">
                  {task.progress}%
                </span>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}