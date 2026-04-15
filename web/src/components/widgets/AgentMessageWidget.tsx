// web/src/components/widgets/AgentMessageWidget.tsx
import { useEffect, useState } from 'react';
import { useMessageStore } from '@/stores/messageStore';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import {
  MessageSquare,
  Send,
  Loader2,
  Bell,
  HelpCircle,
  FileText,
  AlertTriangle,
  CheckCircle2,
} from 'lucide-react';

const MESSAGE_TYPE_CONFIG = {
  notify: { label: '通知', icon: <Bell className="w-4 h-4" />, color: 'bg-blue-100 text-blue-800' },
  question: { label: '对话', icon: <HelpCircle className="w-4 h-4" />, color: 'bg-yellow-100 text-yellow-800' },
  data: { label: '数据', icon: <FileText className="w-4 h-4" />, color: 'bg-green-100 text-green-800' },
  request_approval: { label: '审批', icon: <AlertTriangle className="w-4 h-4" />, color: 'bg-red-100 text-red-800' },
};

const MESSAGE_STATUS_CONFIG = {
  pending: { label: '待处理', color: 'bg-gray-200' },
  delivered: { label: '已送达', color: 'bg-blue-200' },
  read: { label: '已读', color: 'bg-green-200' },
  responded: { label: '已回复', color: 'bg-purple-200' },
};

interface AgentMessageWidgetProps {
  agentId?: string; // Current agent to view messages for
}

export function AgentMessageWidget({ agentId }: AgentMessageWidgetProps) {
  const {
    messages,
    selectedMessage,
    isLoading,
    error,
    fetchMessages,
    setSelectedMessage,
    sendMessage,
    respondToMessage,
  } = useMessageStore();

  const [newMessage, setNewMessage] = useState('');
  const [responseText, setResponseText] = useState('');
  const [targetAgent, setTargetAgent] = useState('');

  useEffect(() => {
    if (agentId) {
      fetchMessages(agentId);
    }
  }, [agentId, fetchMessages]);

  const handleSendMessage = async () => {
    if (!agentId || !targetAgent || !newMessage) return;
    await sendMessage(agentId, targetAgent, 'question', newMessage, true);
    setNewMessage('');
    setTargetAgent('');
  };

  const handleRespond = async () => {
    if (!selectedMessage || !responseText) return;
    await respondToMessage(selectedMessage.id, responseText);
    setResponseText('');
  };

  return (
    <Card className="h-full">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2">
          <MessageSquare className="w-5 h-5" />
          Agent 消息
          {messages.filter((m) => m.status === 'pending').length > 0 && (
            <Badge variant="destructive">
              {messages.filter((m) => m.status === 'pending').length}
            </Badge>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {error && <div className="text-sm text-red-500">{error}</div>}

        {!agentId ? (
          <div className="text-center py-8 text-muted-foreground">
            请选择 Agent 查看消息
          </div>
        ) : isLoading && messages.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <>
            {/* Send new message */}
            <div className="border rounded-lg p-3 mb-3">
              <div className="text-sm font-medium mb-2">发送消息</div>
              <div className="flex gap-2 mb-2">
                <input
                  type="text"
                  value={targetAgent}
                  onChange={(e) => setTargetAgent(e.target.value)}
                  placeholder="目标 Agent ID"
                  className="flex-1 px-2 py-1 text-sm border rounded"
                />
                <select
                  value="question"
                  className="px-2 py-1 text-sm border rounded"
                  disabled
                >
                  <option value="question">对话</option>
                </select>
              </div>
              <Textarea
                value={newMessage}
                onChange={(e) => setNewMessage(e.target.value)}
                placeholder="消息内容..."
                className="min-h-40 text-sm"
              />
              <Button
                size="sm"
                className="mt-2"
                onClick={handleSendMessage}
                disabled={!targetAgent || !newMessage || isLoading}
              >
                <Send className="w-4 h-4 mr-1" />
                发送
              </Button>
            </div>

            {/* Message list */}
            <div className="space-y-2">
              {messages.map((message) => {
                const typeConfig = MESSAGE_TYPE_CONFIG[message.type];
                const statusConfig = MESSAGE_STATUS_CONFIG[message.status];

                return (
                  <div
                    key={message.id}
                    className={cn(
                      'p-3 rounded-lg border cursor-pointer transition-colors',
                      selectedMessage?.id === message.id
                        ? 'border-primary bg-primary/5'
                        : 'hover:bg-muted/50'
                    )}
                    onClick={() => setSelectedMessage(message)}
                  >
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <Badge className={cn('text-xs', typeConfig.color)}>
                          {typeConfig.icon}
                          <span className="ml-1">{typeConfig.label}</span>
                        </Badge>
                        <span className="text-sm font-medium">
                          {message.fromAgent}
                          {message.proxyFor && (
                            <span className="text-xs text-muted-foreground ml-1">
                              (代理 {message.proxyFor})
                            </span>
                          )}
                          → {message.toAgent}
                        </span>
                      </div>
                      <Badge className={cn('text-xs', statusConfig.color)}>
                        {statusConfig.label}
                      </Badge>
                    </div>

                    <div className="text-sm text-muted-foreground truncate">
                      {message.content.slice(0, 100)}
                      {message.content.length > 100 && '...'}
                    </div>

                    {message.requiresResponse && message.status !== 'responded' && (
                      <div className="mt-2 flex items-center gap-1 text-xs text-yellow-600">
                        <HelpCircle className="w-3 h-3" />
                        需要回复
                      </div>
                    )}

                    {/* Response preview */}
                    {message.response && (
                      <div className="mt-2 bg-muted/50 rounded p-2">
                        <div className="text-xs font-medium mb-1">回复:</div>
                        <div className="text-xs">{message.response.slice(0, 50)}...</div>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

// Response modal for selected message
export function MessageResponseModal() {
  const { selectedMessage, respondToMessage, isLoading } = useMessageStore();
  const [responseText, setResponseText] = useState('');

  if (!selectedMessage || !selectedMessage.requiresResponse) return null;

  const handleRespond = async () => {
    await respondToMessage(selectedMessage.id, responseText);
    setResponseText('');
  };

  const typeConfig = MESSAGE_TYPE_CONFIG[selectedMessage.type];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={() => useMessageStore.getState().setSelectedMessage(null)}
      />

      <Card className="relative max-w-lg w-full mx-4 overflow-hidden">
        <CardHeader className="border-b">
          <CardTitle className="flex items-center gap-2">
            {typeConfig.icon}
            回复消息
          </CardTitle>
        </CardHeader>
        <CardContent className="p-4 space-y-4">
          <div>
            <div className="text-sm font-medium mb-1">来自</div>
            <div className="text-sm">
              {selectedMessage.fromAgent}
              {selectedMessage.proxyFor && (
                <span className="text-xs text-muted-foreground ml-1">
                  (代理 {selectedMessage.proxyFor})
                </span>
              )}
            </div>
          </div>

          <div>
            <div className="text-sm font-medium mb-2">消息内容</div>
            <div className="bg-muted/50 rounded-lg p-3">
              <pre className="text-sm whitespace-pre-wrap">
                {selectedMessage.content}
              </pre>
            </div>
          </div>

          <div>
            <div className="text-sm font-medium mb-2">你的回复</div>
            <Textarea
              value={responseText}
              onChange={(e) => setResponseText(e.target.value)}
              placeholder="输入回复..."
              className="min-h-80"
            />
          </div>

          <div className="flex justify-end gap-3 pt-4 border-t">
            <Button
              variant="outline"
              onClick={() => useMessageStore.getState().setSelectedMessage(null)}
            >
              取消
            </Button>
            <Button
              onClick={handleRespond}
              disabled={!responseText || isLoading}
            >
              {isLoading ? (
                <Loader2 className="w-4 h-4 animate-spin mr-2" />
              ) : (
                <CheckCircle2 className="w-4 h-4 mr-2" />
              )}
              发送回复
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}