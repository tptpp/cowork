import { CheckCircle, Clock, AlertTriangle, Server, Activity, TrendingUp, Zap } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useSystemStore } from '@/stores/systemStore'
import { cn } from '@/lib/utils'

export function SystemStatsWidget() {
  const { stats, loading } = useSystemStore()

  const statItems = [
    {
      title: 'Total Tasks',
      value: stats?.tasks.total ?? 0,
      subtext: `${stats?.tasks.running ?? 0} running`,
      icon: CheckCircle,
      color: 'text-primary',
      bg: 'bg-primary/10',
    },
    {
      title: 'Pending',
      value: stats?.tasks.pending ?? 0,
      subtext: 'Waiting for worker',
      icon: Clock,
      color: 'text-amber-500',
      bg: 'bg-amber-500/10',
    },
    {
      title: 'Completed',
      value: stats?.tasks.completed ?? 0,
      subtext: 'Successfully finished',
      icon: TrendingUp,
      color: 'text-green-500',
      bg: 'bg-green-500/10',
    },
    {
      title: 'Failed',
      value: stats?.tasks.failed ?? 0,
      subtext: 'Failed tasks',
      icon: AlertTriangle,
      color: 'text-destructive',
      bg: 'bg-destructive/10',
    },
    {
      title: 'Online Workers',
      value: `${stats?.workers.online ?? 0} / ${stats?.workers.total ?? 0}`,
      subtext: `${stats?.workers.offline ?? 0} offline`,
      icon: Server,
      color: 'text-blue-500',
      bg: 'bg-blue-500/10',
    },
    {
      title: 'System Uptime',
      value: stats?.system.uptime ?? '...',
      subtext: `v${stats?.system.version ?? '-'}`,
      icon: Activity,
      color: 'text-emerald-500',
      bg: 'bg-emerald-500/10',
    },
  ]

  return (
    <div className="h-full flex flex-col p-4">
      <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3">
        {statItems.map((item) => (
          <div
            key={item.title}
            className={cn(
              'p-3 rounded-xl bg-muted/30 border border-border/50 transition-all hover:bg-muted/50',
              loading && 'animate-pulse'
            )}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium text-muted-foreground truncate">
                {item.title}
              </span>
              <div className={cn('p-1.5 rounded-lg shrink-0', item.bg)}>
                <item.icon className={cn('h-3.5 w-3.5', item.color)} />
              </div>
            </div>
            <div className="text-xl font-bold truncate">{item.value}</div>
            <p className="text-xs text-muted-foreground mt-0.5 truncate">{item.subtext}</p>
          </div>
        ))}
      </div>
    </div>
  )
}