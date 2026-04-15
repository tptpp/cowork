// web/src/components/widgets/NodeStatusWidget.tsx
import { useEffect } from 'react';
import { useNodeStore } from '@/stores/nodeStore';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import {
  Server,
  Box,
  Cloud,
  Monitor,
  Circle,
  Loader2,
  Globe,
  Cpu,
  Container,
} from 'lucide-react';

const NODE_TYPE_CONFIG = {
  sandbox: { label: '沙箱', icon: <Box className="w-4 h-4" />, color: 'text-purple-500' },
  docker: { label: '容器', icon: <Container className="w-4 h-4" />, color: 'text-blue-500' },
  physical: { label: '物理机', icon: <Monitor className="w-4 h-4" />, color: 'text-green-500' },
  cloud: { label: '云服务器', icon: <Cloud className="w-4 h-4" />, color: 'text-orange-500' },
};

const NODE_STATUS_CONFIG = {
  idle: { label: '空闲', color: 'bg-gray-200 dark:bg-gray-700' },
  busy: { label: '繁忙', color: 'bg-blue-500' },
  offline: { label: '离线', color: 'bg-red-500' },
};

const CAPABILITY_ICONS: Record<string, React.ReactNode> = {
  browser: <Globe className="w-3 h-3" />,
  gpu: <Cpu className="w-3 h-3" />,
  docker: <Container className="w-3 h-3" />,
};

export function NodeStatusWidget() {
  const { nodes, isLoading, error, fetchNodes } = useNodeStore();

  useEffect(() => {
    fetchNodes();
    // Poll for updates every 10 seconds
    const interval = setInterval(fetchNodes, 10000);
    return () => clearInterval(interval);
  }, [fetchNodes]);

  const getNodeAge = (lastSeen: string) => {
    const last = new Date(lastSeen);
    const age = Date.now() - last.getTime();
    if (age < 60000) return '刚刚';
    if (age < 3600000) return `${Math.floor(age / 60000)}分钟前`;
    return `${Math.floor(age / 3600000)}小时前`;
  };

  const isNodeStale = (lastSeen: string) => {
    const last = new Date(lastSeen);
    return Date.now() - last.getTime() > 300000; // 5 minutes
  };

  return (
    <Card className="h-full">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2">
          <Server className="w-5 h-5" />
          计算节点
          {nodes.filter((n) => n.status !== 'offline').length > 0 && (
            <Badge variant="outline">
              {nodes.filter((n) => n.status !== 'offline').length} / {nodes.length}
            </Badge>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {error && <div className="text-sm text-red-500">{error}</div>}

        {isLoading && nodes.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        ) : nodes.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            暂无注册节点
          </div>
        ) : (
          <div className="space-y-2">
            {nodes.map((node) => {
              const typeConfig = NODE_TYPE_CONFIG[node.type];
              const statusConfig = NODE_STATUS_CONFIG[node.status];
              const stale = isNodeStale(node.lastSeen);

              return (
                <div
                  key={node.id}
                  className={cn(
                    'p-3 rounded-lg border transition-opacity',
                    stale && 'opacity-60'
                  )}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className={typeConfig.color}>{typeConfig.icon}</span>
                      <span className="font-medium">{node.name}</span>
                      <Badge variant="secondary" className="text-xs">
                        {typeConfig.label}
                      </Badge>
                    </div>
                    <div className="flex items-center gap-2">
                      <Circle
                        className={cn('w-3 h-3', statusConfig.color, 'fill-current')}
                      />
                      <span className="text-xs text-muted-foreground">
                        {statusConfig.label}
                      </span>
                    </div>
                  </div>

                  {/* Capabilities */}
                  {node.capabilities && Object.keys(node.capabilities).length > 0 && (
                    <div className="flex gap-1 mb-2">
                      {Object.entries(node.capabilities)
                        .filter(([, enabled]) => enabled)
                        .map(([cap]) => (
                          <Badge
                            key={cap}
                            variant="outline"
                            className="text-xs flex items-center gap-1"
                          >
                            {CAPABILITY_ICONS[cap] || null}
                            {cap}
                          </Badge>
                        ))}
                    </div>
                  )}

                  {/* Current agent */}
                  {node.currentAgentId && (
                    <div className="text-xs text-muted-foreground mb-1">
                      正在处理: {node.currentAgentId}
                    </div>
                  )}

                  {/* Last seen */}
                  <div className="text-xs text-muted-foreground">
                    最后活动: {getNodeAge(node.lastSeen)}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}