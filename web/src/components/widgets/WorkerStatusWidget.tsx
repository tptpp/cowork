import { Server, Wifi, WifiOff } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useWorkerStore } from '@/stores/workerStore'
import type { Worker } from '@/types'

const getStatusColor = (status: string) => {
  switch (status) {
    case 'idle':
      return 'text-green-500'
    case 'busy':
      return 'text-blue-500'
    case 'offline':
      return 'text-muted-foreground'
    case 'error':
      return 'text-destructive'
    default:
      return 'text-muted-foreground'
  }
}

const getStatusBadgeVariant = (status: string): 'default' | 'secondary' | 'success' | 'destructive' | 'warning' => {
  switch (status) {
    case 'idle':
      return 'success'
    case 'busy':
      return 'default'
    case 'offline':
      return 'secondary'
    case 'error':
      return 'destructive'
    default:
      return 'secondary'
  }
}

interface WorkerStatusWidgetProps {
  onSelectWorker?: (worker: Worker) => void
}

export function WorkerStatusWidget({ onSelectWorker }: WorkerStatusWidgetProps) {
  const { workers, loading } = useWorkerStore()

  const onlineWorkers = workers.filter((w) => w.status !== 'offline')
  const offlineWorkers = workers.filter((w) => w.status === 'offline')

  if (workers.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Server className="w-5 h-5" />
            Workers
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-center py-8">
            No workers registered
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
            <Server className="w-5 h-5" />
            Workers
          </span>
          <div className="flex items-center gap-2 text-sm font-normal">
            <Wifi className="w-4 h-4 text-green-500" />
            <span>{onlineWorkers.length}</span>
            <WifiOff className="w-4 h-4 text-muted-foreground ml-2" />
            <span>{offlineWorkers.length}</span>
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {loading ? (
            <p className="text-muted-foreground text-center py-4">Loading...</p>
          ) : (
            workers.map((worker) => (
              <div
                key={worker.id}
                className="flex items-center justify-between p-3 rounded-lg bg-muted/50 hover:bg-muted cursor-pointer transition-colors"
                onClick={() => onSelectWorker?.(worker)}
              >
                <div className="flex items-center gap-3">
                  <Server className={`w-4 h-4 ${getStatusColor(worker.status)}`} />
                  <div>
                    <div className="font-medium">{worker.name}</div>
                    <div className="text-sm text-muted-foreground">
                      {worker.model}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <div className="text-right">
                    <Badge variant={getStatusBadgeVariant(worker.status)}>
                      {worker.status}
                    </Badge>
                    <div className="text-xs text-muted-foreground mt-1">
                      {worker.completed_tasks} tasks
                    </div>
                  </div>
                  {worker.tags.length > 0 && (
                    <div className="flex gap-1 flex-wrap max-w-32">
                      {worker.tags.slice(0, 2).map((tag) => (
                        <Badge key={tag} variant="outline" className="text-xs">
                          {tag}
                        </Badge>
                      ))}
                      {worker.tags.length > 2 && (
                        <Badge variant="outline" className="text-xs">
                          +{worker.tags.length - 2}
                        </Badge>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  )
}