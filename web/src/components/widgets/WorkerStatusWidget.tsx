import { Server, Wifi, WifiOff, Circle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { useWorkerStore } from '@/stores/workerStore'
import type { Worker } from '@/types'
import { cn } from '@/lib/utils'

const getStatusConfig = (status: string) => {
  switch (status) {
    case 'idle':
      return { color: 'text-green-500', bg: 'bg-green-500/10', dot: 'bg-green-500' }
    case 'busy':
      return { color: 'text-blue-500', bg: 'bg-blue-500/10', dot: 'bg-blue-500' }
    case 'offline':
      return { color: 'text-muted-foreground', bg: 'bg-muted', dot: 'bg-muted-foreground' }
    case 'error':
      return { color: 'text-destructive', bg: 'bg-destructive/10', dot: 'bg-destructive' }
    default:
      return { color: 'text-muted-foreground', bg: 'bg-muted', dot: 'bg-muted-foreground' }
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
      <div className="h-full flex flex-col items-center justify-center text-muted-foreground p-4">
        <div className="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mb-3">
          <Server className="w-6 h-6 opacity-50" />
        </div>
        <p className="text-sm font-medium">No workers</p>
        <p className="text-xs text-muted-foreground mb-3">Start a worker to begin processing tasks</p>
        <code className="text-xs bg-muted px-2 py-1 rounded font-mono">./worker</code>
      </div>
    )
  }

  return (
    <div className="h-full flex flex-col p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Server className="w-4 h-4 text-primary" />
          <span className="text-sm font-medium">Workers</span>
        </div>
        <div className="flex items-center gap-3 text-xs">
          <div className="flex items-center gap-1">
            <Wifi className="w-3 h-3 text-green-500" />
            <span className="font-medium">{onlineWorkers.length}</span>
          </div>
          <div className="flex items-center gap-1">
            <WifiOff className="w-3 h-3 text-muted-foreground" />
            <span className="text-muted-foreground">{offlineWorkers.length}</span>
          </div>
        </div>
      </div>

      {/* Worker List */}
      <div className="flex-1 overflow-auto space-y-2">
        {loading ? (
          <div className="text-center text-muted-foreground text-sm py-4">
            Loading...
          </div>
        ) : (
          workers.map((worker) => {
            const config = getStatusConfig(worker.status)
            return (
              <div
                key={worker.id}
                onClick={() => onSelectWorker?.(worker)}
                className={cn(
                  'flex items-center gap-3 p-3 rounded-xl border border-border/50 bg-muted/30',
                  'hover:bg-muted/50 hover:border-primary/30 cursor-pointer transition-all',
                  'active:scale-[0.98]'
                )}
              >
                <div className={cn('p-2 rounded-lg', config.bg)}>
                  <Server className={cn('w-4 h-4', config.color)} />
                </div>

                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm truncate">{worker.name}</span>
                    <Circle className={cn('w-2 h-2 fill-current', config.dot, config.color)} />
                  </div>
                  <p className="text-xs text-muted-foreground truncate">{worker.tags.join(', ')}</p>
                </div>

                <div className="flex items-center gap-2">
                  <div className="text-right">
                    <Badge variant={getStatusBadgeVariant(worker.status)} className="text-xs px-1.5 py-0 h-4">
                      {worker.status}
                    </Badge>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {worker.completed_tasks} tasks
                    </p>
                  </div>
                </div>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}