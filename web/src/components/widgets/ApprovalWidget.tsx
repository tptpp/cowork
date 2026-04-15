// web/src/components/widgets/ApprovalWidget.tsx
import { useEffect } from 'react';
import { useApprovalStore } from '@/stores/approvalStore';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import {
  AlertTriangle,
  Clock,
  CheckCircle2,
  XCircle,
  Loader2,
  Shield,
  ShieldAlert,
  ShieldCheck,
} from 'lucide-react';

const RISK_CONFIG = {
  low: {
    label: '低风险',
    color: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    icon: <ShieldCheck className="w-4 h-4" />,
    autoApproveTimeout: 60, // seconds
  },
  medium: {
    label: '中风险',
    color: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    icon: <Shield className="w-4 h-4" />,
    autoApproveTimeout: 300, // 5 minutes
  },
  high: {
    label: '高风险',
    color: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    icon: <ShieldAlert className="w-4 h-4" />,
    autoApproveTimeout: null, // no auto-approve
  },
};

export function ApprovalWidget() {
  const {
    pendingApprovals,
    selectedApproval,
    isLoading,
    error,
    fetchPendingApprovals,
    setSelectedApproval,
    approveRequest,
    rejectRequest,
  } = useApprovalStore();

  useEffect(() => {
    fetchPendingApprovals();
    // Poll for new approvals every 30 seconds
    const interval = setInterval(fetchPendingApprovals, 30000);
    return () => clearInterval(interval);
  }, [fetchPendingApprovals]);

  const handleApprove = async (id: string) => {
    await approveRequest(id, 'user'); // TODO: Get actual user ID from auth
  };

  const handleReject = async (id: string) => {
    await rejectRequest(id, 'user');
  };

  const getTimeRemaining = (createdAt: string, timeoutSeconds?: number) => {
    if (!timeoutSeconds) return null;
    const created = new Date(createdAt);
    const deadline = new Date(created.getTime() + timeoutSeconds * 1000);
    const remaining = Math.max(0, deadline.getTime() - Date.now());
    return Math.floor(remaining / 1000);
  };

  return (
    <Card className="h-full">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2">
          <Shield className="w-5 h-5" />
          审批队列
          {pendingApprovals.length > 0 && (
            <Badge variant="destructive">{pendingApprovals.length}</Badge>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {error && (
          <div className="text-sm text-red-500">{error}</div>
        )}

        {isLoading && pendingApprovals.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        ) : pendingApprovals.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            暂无待审批请求
          </div>
        ) : (
          <div className="space-y-2">
            {pendingApprovals.map((approval) => {
              const config = RISK_CONFIG[approval.riskLevel];
              const timeRemaining = getTimeRemaining(
                approval.createdAt,
                approval.timeoutSeconds
              );
              const isExpired = timeRemaining !== null && timeRemaining <= 0;

              return (
                <div
                  key={approval.id}
                  className={cn(
                    'p-3 rounded-lg border cursor-pointer transition-colors',
                    selectedApproval?.id === approval.id
                      ? 'border-primary bg-primary/5'
                      : 'hover:bg-muted/50',
                    isExpired && 'opacity-50'
                  )}
                  onClick={() => setSelectedApproval(approval)}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <Badge className={config.color}>
                        {config.icon}
                        <span className="ml-1">{config.label}</span>
                      </Badge>
                      <span className="font-medium text-sm">{approval.action}</span>
                    </div>
                    {timeRemaining !== null && (
                      <div className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Clock className="w-3 h-3" />
                        {isExpired ? '已过期' : `${timeRemaining}秒`}
                      </div>
                    )}
                  </div>

                  <div className="text-xs text-muted-foreground mb-2">
                    Agent: {approval.agentId}
                  </div>

                  {/* Action details preview */}
                  <div className="text-xs bg-muted/50 rounded p-2 overflow-hidden">
                    <pre className="whitespace-pre-wrap truncate">
                      {JSON.stringify(approval.actionDetail, null, 2).slice(0, 100)}
                    </pre>
                  </div>

                  {/* Quick approve/reject buttons */}
                  {!isExpired && (
                    <div className="flex gap-2 mt-2">
                      <Button
                        size="sm"
                        variant="outline"
                        className="text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleReject(approval.id);
                        }}
                        disabled={isLoading}
                      >
                        <XCircle className="w-3 h-3 mr-1" />
                        拒绝
                      </Button>
                      <Button
                        size="sm"
                        className="bg-green-600 hover:bg-green-700"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleApprove(approval.id);
                        }}
                        disabled={isLoading}
                      >
                        <CheckCircle2 className="w-3 h-3 mr-1" />
                        批准
                      </Button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// Expanded approval detail modal
export function ApprovalDetailModal() {
  const { selectedApproval, approveRequest, rejectRequest, isLoading } = useApprovalStore();

  if (!selectedApproval) return null;

  const config = RISK_CONFIG[selectedApproval.riskLevel];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={() => useApprovalStore.getState().setSelectedApproval(null)}
      />

      <Card className="relative max-w-lg w-full mx-4 overflow-hidden">
        <CardHeader className={cn('border-b', config.color)}>
          <CardTitle className="flex items-center gap-2">
            {config.icon}
            审批详情 - {config.label}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-4 space-y-4">
          <div>
            <div className="text-sm font-medium mb-1">操作类型</div>
            <div className="text-lg">{selectedApproval.action}</div>
          </div>

          <div>
            <div className="text-sm font-medium mb-1">发起 Agent</div>
            <div className="text-sm">{selectedApproval.agentId}</div>
          </div>

          <div>
            <div className="text-sm font-medium mb-2">操作详情</div>
            <div className="bg-muted/50 rounded-lg p-3 overflow-auto max-h-60">
              <pre className="text-sm font-mono whitespace-pre-wrap">
                {JSON.stringify(selectedApproval.actionDetail, null, 2)}
              </pre>
            </div>
          </div>

          {selectedApproval.riskLevel === 'high' && (
            <div className="p-3 rounded-lg bg-red-50 border border-red-200 dark:bg-red-900/20 dark:border-red-800">
              <div className="flex items-start gap-2">
                <AlertTriangle className="w-5 h-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
                <div className="text-sm text-red-800 dark:text-red-300">
                  此操作为高风险操作，必须人工审批。请仔细检查所有细节后再做决定。
                </div>
              </div>
            </div>
          )}

          <div className="flex justify-end gap-3 pt-4 border-t">
            <Button
              variant="outline"
              className="text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
              onClick={() => rejectRequest(selectedApproval.id, 'user')}
              disabled={isLoading}
            >
              {isLoading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : <XCircle className="w-4 h-4 mr-2" />}
              拒绝
            </Button>
            <Button
              className="bg-green-600 hover:bg-green-700"
              onClick={() => approveRequest(selectedApproval.id, 'user')}
              disabled={isLoading}
            >
              {isLoading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : <CheckCircle2 className="w-4 h-4 mr-2" />}
              批准
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}