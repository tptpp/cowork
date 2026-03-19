import { useEffect, useState } from 'react'
import { Clock, AlertCircle, CheckCircle, XCircle, Loader2 } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useTaskStore } from '@/stores/taskStore'
import type { Task, TaskStatus } from '@/types'

interface TaskListProps {
  onSelectTask?: (task: Task) => void
}

const statusFilters: { value: TaskStatus | 'all'; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'running', label: 'Running' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'cancelled', label: 'Cancelled' },
]

const priorityColors: Record<string, string> = {
  high: 'text-red-500',
  medium: 'text-yellow-500',
  low: 'text-green-500',
}

export function TaskList({ onSelectTask }: TaskListProps) {
  const { tasks, loading, error, fetchTasks, pagination } = useTaskStore()
  const [statusFilter, setStatusFilter] = useState<TaskStatus | 'all'>('all')

  useEffect(() => {
    fetchTasks(statusFilter === 'all' ? undefined : { status: statusFilter })
  }, [statusFilter, fetchTasks])

  const getStatusIcon = (status: TaskStatus) => {
    switch (status) {
      case 'pending':
        return <Clock className="w-4 h-4" />
      case 'running':
        return <Loader2 className="w-4 h-4 animate-spin" />
      case 'completed':
        return <CheckCircle className="w-4 h-4" />
      case 'failed':
        return <AlertCircle className="w-4 h-4" />
      case 'cancelled':
        return <XCircle className="w-4 h-4" />
      default:
        return null
    }
  }

  const getStatusBadgeVariant = (status: TaskStatus): 'default' | 'secondary' | 'success' | 'destructive' | 'warning' => {
    switch (status) {
      case 'pending':
        return 'secondary'
      case 'running':
        return 'default'
      case 'completed':
        return 'success'
      case 'failed':
        return 'destructive'
      case 'cancelled':
        return 'warning'
      default:
        return 'secondary'
    }
  }

  const formatDate = (dateString: string) => {
    const date = new Date(dateString)
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span>Tasks</span>
          <span className="text-sm font-normal text-muted-foreground">
            {pagination.total} total
          </span>
        </CardTitle>
        <div className="flex gap-2 mt-2 flex-wrap">
          {statusFilters.map((filter) => (
            <Button
              key={filter.value}
              variant={statusFilter === filter.value ? 'default' : 'outline'}
              size="sm"
              onClick={() => setStatusFilter(filter.value)}
            >
              {filter.label}
            </Button>
          ))}
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <div className="text-destructive text-center py-8">{error}</div>
        ) : tasks.length === 0 ? (
          <p className="text-muted-foreground text-center py-8">No tasks found</p>
        ) : (
          <div className="space-y-2">
            {tasks.map((task) => (
              <div
                key={task.id}
                className="flex items-center justify-between p-3 rounded-lg bg-muted/50 hover:bg-muted cursor-pointer transition-colors"
                onClick={() => onSelectTask?.(task)}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{task.type}</span>
                    <span className={`text-xs ${priorityColors[task.priority]}`}>
                      {task.priority.toUpperCase()}
                    </span>
                  </div>
                  <div className="text-sm text-muted-foreground truncate">
                    {task.description || `ID: ${task.id.slice(0, 8)}`}
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  {task.status === 'running' && (
                    <div className="flex items-center gap-2">
                      <div className="w-20 bg-muted rounded-full h-2">
                        <div
                          className="bg-primary h-2 rounded-full transition-all"
                          style={{ width: `${task.progress}%` }}
                        />
                      </div>
                      <span className="text-xs text-muted-foreground w-8">
                        {task.progress}%
                      </span>
                    </div>
                  )}
                  <Badge variant={getStatusBadgeVariant(task.status)}>
                    <span className="flex items-center gap-1">
                      {getStatusIcon(task.status)}
                      {task.status}
                    </span>
                  </Badge>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {formatDate(task.created_at)}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}

        {pagination.total_pages > 1 && (
          <div className="flex items-center justify-center gap-2 mt-4">
            <Button
              variant="outline"
              size="sm"
              disabled={pagination.page <= 1}
              onClick={() => fetchTasks(statusFilter === 'all' ? undefined : { status: statusFilter, page: pagination.page - 1 })}
            >
              Previous
            </Button>
            <span className="text-sm text-muted-foreground">
              Page {pagination.page} of {pagination.total_pages}
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={pagination.page >= pagination.total_pages}
              onClick={() => fetchTasks(statusFilter === 'all' ? undefined : { status: statusFilter, page: pagination.page + 1 })}
            >
              Next
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}