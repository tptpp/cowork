import { CheckCircle, Clock, AlertTriangle, Server, Activity } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useSystemStore } from '@/stores/systemStore'

export function SystemStatsWidget() {
  const { stats, loading } = useSystemStore()

  const statItems = [
    {
      title: 'Total Tasks',
      value: stats?.tasks.total ?? 0,
      subtext: `${stats?.tasks.running ?? 0} running`,
      icon: CheckCircle,
      iconColor: 'text-muted-foreground',
    },
    {
      title: 'Pending',
      value: stats?.tasks.pending ?? 0,
      subtext: 'Waiting for worker',
      icon: Clock,
      iconColor: 'text-muted-foreground',
    },
    {
      title: 'Completed',
      value: stats?.tasks.completed ?? 0,
      subtext: 'Successfully finished',
      icon: CheckCircle,
      iconColor: 'text-green-500',
    },
    {
      title: 'Failed',
      value: stats?.tasks.failed ?? 0,
      subtext: 'Failed tasks',
      icon: AlertTriangle,
      iconColor: 'text-destructive',
    },
    {
      title: 'Online Workers',
      value: `${stats?.workers.online ?? 0} / ${stats?.workers.total ?? 0}`,
      subtext: `${stats?.workers.offline ?? 0} offline`,
      icon: Server,
      iconColor: 'text-muted-foreground',
    },
    {
      title: 'System Uptime',
      value: stats?.system.uptime ?? '...',
      subtext: `v${stats?.system.version ?? '-'}`,
      icon: Activity,
      iconColor: 'text-green-500',
    },
  ]

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
      {statItems.map((item) => (
        <Card key={item.title} className={loading ? 'animate-pulse' : ''}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">
              {item.title}
            </CardTitle>
            <item.icon className={`h-4 w-4 ${item.iconColor}`} />
          </CardHeader>
          <CardContent className="pt-0">
            <div className="text-xl font-bold">{item.value}</div>
            <p className="text-xs text-muted-foreground mt-0.5">{item.subtext}</p>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}