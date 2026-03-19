import { useEffect, useState } from 'react'
import {
  ArrowLeft,
  Clock,
  AlertCircle,
  CheckCircle,
  XCircle,
  Loader2,
  Play,
  Tag,
  Calendar,
  FileText,
  Terminal,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useTaskStore, type TaskLog } from '@/stores/taskStore'
import { useWebSocket } from '@/hooks/useWebSocket'
import type { Task, WSMessage } from '@/types'

interface TaskDetailProps {
  taskId: string
  onBack?: () => void
}

const getStatusIcon = (status: string) => {
  switch (status) {
    case 'pending':
      return <Clock className="w-5 h-5" />
    case 'running':
      return <Loader2 className="w-5 h-5 animate-spin" />
    case 'completed':
      return <CheckCircle className="w-5 h-5" />
    case 'failed':
      return <AlertCircle className="w-5 h-5" />
    case 'cancelled':
      return <XCircle className="w-5 h-5" />
    default:
      return null
  }
}

const getStatusBadgeVariant = (status: string): 'default' | 'secondary' | 'success' | 'destructive' | 'warning' => {
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

const getLogLevelColor = (level: string) => {
  switch (level.toLowerCase()) {
    case 'error':
      return 'text-red-500'
    case 'warn':
    case 'warning':
      return 'text-yellow-500'
    case 'info':
      return 'text-blue-500'
    case 'debug':
      return 'text-gray-500'
    default:
      return 'text-foreground'
  }
}

export function TaskDetail({ taskId, onBack }: TaskDetailProps) {
  const { currentTask, logs, fetchTask, fetchLogs, cancelTask } = useTaskStore()
  const [cancelling, setCancelling] = useState(false)
  const [activeTab, setActiveTab] = useState<'info' | 'logs'>('info')

  // WebSocket for real-time updates
  useWebSocket({
    onMessage: (message: WSMessage) => {
      if (message.type === 'task_update') {
        const updatedTask = message.payload as Task
        if (updatedTask.id === taskId) {
          useTaskStore.setState({ currentTask: updatedTask })
        }
      } else if (message.type === 'task_log') {
        const log = message.payload as TaskLog
        if (log.task_id === taskId) {
          useTaskStore.setState((state) => ({
            logs: [...state.logs, log],
          }))
        }
      }
    },
  })

  useEffect(() => {
    fetchTask(taskId)
    fetchLogs(taskId)
  }, [taskId, fetchTask, fetchLogs])

  const handleCancel = async () => {
    setCancelling(true)
    await cancelTask(taskId)
    setCancelling(false)
  }

  const formatDate = (dateString: string | null) => {
    if (!dateString) return 'N/A'
    return new Date(dateString).toLocaleString('zh-CN')
  }

  const formatDuration = () => {
    if (!currentTask) return 'N/A'
    const start = currentTask.started_at ? new Date(currentTask.started_at).getTime() : null
    const end = currentTask.completed_at ? new Date(currentTask.completed_at).getTime() : Date.now()
    if (!start) return 'N/A'
    const duration = Math.floor((end - start) / 1000)
    if (duration < 60) return `${duration}s`
    if (duration < 3600) return `${Math.floor(duration / 60)}m ${duration % 60}s`
    return `${Math.floor(duration / 3600)}h ${Math.floor((duration % 3600) / 60)}m`
  }

  if (!currentTask) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center py-12">
          <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          {onBack && (
            <Button variant="ghost" size="icon" onClick={onBack}>
              <ArrowLeft className="w-5 h-5" />
            </Button>
          )}
          <div>
            <h2 className="text-xl font-semibold">{currentTask.type}</h2>
            <p className="text-sm text-muted-foreground">{currentTask.id}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={getStatusBadgeVariant(currentTask.status)}>
            <span className="flex items-center gap-1">
              {getStatusIcon(currentTask.status)}
              {currentTask.status}
            </span>
          </Badge>
          {(currentTask.status === 'pending' || currentTask.status === 'running') && (
            <Button
              variant="destructive"
              size="sm"
              onClick={handleCancel}
              disabled={cancelling}
            >
              {cancelling ? (
                <Loader2 className="w-4 h-4 animate-spin mr-1" />
              ) : (
                <XCircle className="w-4 h-4 mr-1" />
              )}
              Cancel
            </Button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-2">
        <Button
          variant={activeTab === 'info' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setActiveTab('info')}
        >
          <FileText className="w-4 h-4 mr-1" />
          Info
        </Button>
        <Button
          variant={activeTab === 'logs' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setActiveTab('logs')}
        >
          <Terminal className="w-4 h-4 mr-1" />
          Logs
          {logs.length > 0 && (
            <Badge variant="secondary" className="ml-1">
              {logs.length}
            </Badge>
          )}
        </Button>
      </div>

      {/* Content */}
      {activeTab === 'info' ? (
        <div className="grid gap-4">
          {/* Progress */}
          {currentTask.status === 'running' && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">Progress</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex items-center gap-4">
                  <div className="flex-1 bg-muted rounded-full h-3">
                    <div
                      className="bg-primary h-3 rounded-full transition-all"
                      style={{ width: `${currentTask.progress}%` }}
                    />
                  </div>
                  <span className="text-sm font-medium">{currentTask.progress}%</span>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Details */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Task Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {currentTask.description && (
                <div>
                  <label className="text-sm text-muted-foreground">Description</label>
                  <p className="mt-1">{currentTask.description}</p>
                </div>
              )}

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-muted-foreground flex items-center gap-1">
                    <Play className="w-3 h-3" /> Priority
                  </label>
                  <p className="mt-1 font-medium capitalize">{currentTask.priority}</p>
                </div>
                <div>
                  <label className="text-sm text-muted-foreground flex items-center gap-1">
                    <Clock className="w-3 h-3" /> Duration
                  </label>
                  <p className="mt-1 font-medium">{formatDuration()}</p>
                </div>
              </div>

              {currentTask.required_tags.length > 0 && (
                <div>
                  <label className="text-sm text-muted-foreground flex items-center gap-1">
                    <Tag className="w-3 h-3" /> Required Tags
                  </label>
                  <div className="flex gap-1 mt-1 flex-wrap">
                    {currentTask.required_tags.map((tag) => (
                      <Badge key={tag} variant="outline">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}

              {currentTask.preferred_model && (
                <div>
                  <label className="text-sm text-muted-foreground">Preferred Model</label>
                  <p className="mt-1">{currentTask.preferred_model}</p>
                </div>
              )}

              {currentTask.worker_id && (
                <div>
                  <label className="text-sm text-muted-foreground">Worker ID</label>
                  <p className="mt-1 font-mono text-sm">{currentTask.worker_id}</p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Timeline */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Calendar className="w-4 h-4" />
                Timeline
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Created</span>
                  <span>{formatDate(currentTask.created_at)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Started</span>
                  <span>{formatDate(currentTask.started_at)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Completed</span>
                  <span>{formatDate(currentTask.completed_at)}</span>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Error */}
          {currentTask.error && (
            <Card className="border-destructive">
              <CardHeader>
                <CardTitle className="text-base text-destructive flex items-center gap-2">
                  <AlertCircle className="w-4 h-4" />
                  Error
                </CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="text-sm text-destructive whitespace-pre-wrap font-mono">
                  {currentTask.error}
                </pre>
              </CardContent>
            </Card>
          )}

          {/* Output */}
          {currentTask.output && Object.keys(currentTask.output).length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Output</CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="text-sm bg-muted p-4 rounded-lg overflow-auto">
                  {JSON.stringify(currentTask.output, null, 2)}
                </pre>
              </CardContent>
            </Card>
          )}
        </div>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Task Logs</CardTitle>
            <CardDescription>Real-time logs from task execution</CardDescription>
          </CardHeader>
          <CardContent>
            {logs.length === 0 ? (
              <p className="text-muted-foreground text-center py-8">No logs available</p>
            ) : (
              <div className="bg-muted rounded-lg p-4 font-mono text-sm max-h-96 overflow-auto">
                {logs.map((log) => (
                  <div key={log.id} className="flex gap-2 py-0.5">
                    <span className="text-muted-foreground shrink-0">
                      {new Date(log.timestamp).toLocaleTimeString('zh-CN')}
                    </span>
                    <span className={`shrink-0 uppercase ${getLogLevelColor(log.level)}`}>
                      [{log.level}]
                    </span>
                    <span className="break-all">{log.message}</span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}