import { Loader2, Clock } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useTaskStore } from '@/stores/taskStore'
import type { Task } from '@/types'

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
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="w-5 h-5" />
            Task Queue
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-center py-8">
            No active tasks in queue
          </p>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span className="flex items-center gap-2">
            <Clock className="w-5 h-5" />
            Task Queue
          </span>
          <Badge variant="secondary">{activeTasks.length} active</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {activeTasks.map((task) => (
            <div
              key={task.id}
              className="flex items-center justify-between p-3 rounded-lg bg-muted/50 hover:bg-muted cursor-pointer transition-colors"
              onClick={() => onSelectTask?.(task)}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  {task.status === 'running' ? (
                    <Loader2 className="w-4 h-4 animate-spin text-primary" />
                  ) : (
                    <Clock className="w-4 h-4 text-muted-foreground" />
                  )}
                  <span className="font-medium truncate">{task.type}</span>
                </div>
                <div className="text-sm text-muted-foreground truncate ml-6">
                  {task.description || `ID: ${task.id.slice(0, 8)}`}
                </div>
              </div>
              {task.status === 'running' && (
                <div className="flex items-center gap-2 ml-2">
                  <div className="w-16 bg-muted rounded-full h-2">
                    <div
                      className="bg-primary h-2 rounded-full transition-all"
                      style={{ width: `${task.progress}%` }}
                    />
                  </div>
                  <span className="text-xs text-muted-foreground w-8 text-right">
                    {task.progress}%
                  </span>
                </div>
              )}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}