import { useState, useRef, useEffect, useCallback } from 'react'
import {
  Send,
  Plus,
  Trash2,
  MessageSquare,
  Loader2,
  Bot,
  User,
  Sparkles,
  Wrench,
  AlertCircle,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { useAgentStore } from '@/stores/agentStore'
import { cn } from '@/lib/utils'
import {
  ToolCallsList,
  PendingToolsBadge,
} from '@/components/agent/ToolCallDisplay'
import { ToolApprovalModal } from '@/components/agent/ToolApprovalModal'
import type { ToolCall, ToolExecution } from '@/types'

const MODEL_OPTIONS = [
  { value: 'default', label: 'Default (Auto)' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'glm', label: 'GLM (智谱)' },
]

// Typing indicator component
function TypingIndicator() {
  return (
    <div className="flex items-center gap-1 px-4 py-2">
      <div className="typing-dot w-2 h-2 rounded-full bg-muted-foreground" />
      <div className="typing-dot w-2 h-2 rounded-full bg-muted-foreground" />
      <div className="typing-dot w-2 h-2 rounded-full bg-muted-foreground" />
    </div>
  )
}

// Tool execution indicator
function ToolExecutionIndicator({ toolName }: { toolName: string }) {
  return (
    <div className="flex items-center gap-2 px-3 py-2 bg-blue-50 dark:bg-blue-900/20 rounded-lg text-blue-700 dark:text-blue-300 text-sm">
      <Wrench className="w-4 h-4" />
      <span className="font-medium">Executing:</span>
      <code className="px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/30 rounded text-xs font-mono">
        {toolName}
      </code>
      <Loader2 className="w-4 h-4 animate-spin ml-1" />
    </div>
  )
}

// Message bubble component
function MessageBubble({
  role,
  content,
  isStreaming,
  toolCalls,
  toolExecutions,
}: {
  role: 'user' | 'assistant' | 'system' | 'tool'
  content: string
  isStreaming?: boolean
  toolCalls?: ToolCall[]
  toolExecutions?: ToolExecution[]
}) {
  const isUser = role === 'user'
  const isTool = role === 'tool'

  // For tool role messages
  if (isTool) {
    return (
      <div className="flex gap-2 max-w-[85%]">
        <div className="flex-shrink-0 w-7 h-7 rounded-full bg-orange-100 dark:bg-orange-900/30 flex items-center justify-center">
          <Wrench className="w-3.5 h-3.5 text-orange-600 dark:text-orange-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="text-xs text-muted-foreground mb-1">Tool Result</div>
          <div className="bg-muted/50 rounded-lg p-3 text-sm font-mono overflow-x-auto">
            <pre className="whitespace-pre-wrap break-all">{content}</pre>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('flex gap-2 max-w-[85%]', isUser ? 'ml-auto flex-row-reverse' : '')}>
      {/* Avatar */}
      <div
        className={cn(
          'flex-shrink-0 w-7 h-7 rounded-full flex items-center justify-center',
          isUser
            ? 'bg-primary text-primary-foreground'
            : 'bg-gradient-to-br from-primary/80 to-primary text-primary-foreground'
        )}
      >
        {isUser ? <User className="w-3.5 h-3.5" /> : <Bot className="w-3.5 h-3.5" />}
      </div>

      {/* Message content */}
      <div className="flex-1 min-w-0 space-y-2">
        {/* Tool calls display */}
        {toolCalls && toolCalls.length > 0 && (
          <ToolCallsList
            toolCalls={toolCalls}
            executions={toolExecutions || []}
          />
        )}

        {/* Text content */}
        {content && (
          <div
            className={cn(
              'message-bubble px-3.5 py-2.5 text-sm leading-relaxed',
              isUser
                ? 'message-bubble-user shadow-md shadow-primary/20'
                : 'message-bubble-assistant',
              isStreaming && 'animate-pulse-soft'
            )}
          >
            {content}
            {isStreaming && (
              <span className="inline-block w-1.5 h-4 ml-0.5 bg-current opacity-60 animate-pulse" />
            )}
          </div>
        )}
      </div>
    </div>
  )
}

// Need to import ToolExecution type
import type { ToolExecution } from '@/types'

export function AgentChatWidget() {
  const [input, setInput] = useState('')
  const [selectedModel, setSelectedModel] = useState('default')
  const [showApprovalModal, setShowApprovalModal] = useState(false)
  const [currentApprovalToolCall, setCurrentApprovalToolCall] = useState<ToolCall | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const {
    sessions,
    currentSession,
    messages,
    isStreaming,
    streamingContent,
    isLoading,
    pendingToolCalls,
    toolExecutions,
    isExecutingTool,
    fetchSessions,
    createSession,
    selectSession,
    deleteSession,
    sendMessageWithTools,
    approveToolCall,
    fetchAvailableTools,
    clearToolState,
    error,
  } = useAgentStore()

  // Fetch sessions and tools on mount
  useEffect(() => {
    fetchSessions()
    fetchAvailableTools()
  }, [fetchSessions, fetchAvailableTools])

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent, pendingToolCalls])

  // Handle pending tool calls for approval
  useEffect(() => {
    if (pendingToolCalls.length > 0 && !showApprovalModal) {
      // Show approval modal for the first pending tool call
      setCurrentApprovalToolCall(pendingToolCalls[0])
      setShowApprovalModal(true)
    }
  }, [pendingToolCalls, showApprovalModal])

  const handleSend = async () => {
    if (!input.trim() || isStreaming) return
    const content = input.trim()
    setInput('')
    await sendMessageWithTools(content)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleNewSession = async () => {
    clearToolState()
    await createSession(selectedModel)
  }

  const handleModelChange = async (newModel: string) => {
    setSelectedModel(newModel)
    clearToolState()
    await createSession(newModel)
  }

  const handleDeleteSession = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    await deleteSession(id)
  }

  const handleApprove = useCallback(
    async (approved: boolean) => {
      if (!currentApprovalToolCall) return

      await approveToolCall(currentApprovalToolCall.id, approved)
      setShowApprovalModal(false)
      setCurrentApprovalToolCall(null)
    },
    [currentApprovalToolCall, approveToolCall]
  )

  const handleCloseApproval = useCallback(() => {
    setShowApprovalModal(false)
    setCurrentApprovalToolCall(null)
  }, [])

  // Get tool executions for a specific message
  const getToolExecutionsForMessage = (toolCalls: ToolCall[]): ToolExecution[] => {
    return toolExecutions.filter((e) =>
      toolCalls.some((tc) => tc.id === e.tool_call_id)
    )
  }

  return (
    <Card className="h-full flex flex-col border-0 shadow-soft bg-card/50 backdrop-blur-sm">
      <CardHeader className="flex-shrink-0 pb-2 space-y-0">
        <CardTitle className="flex items-center justify-between">
          <span className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary to-primary/60 flex items-center justify-center">
              <Sparkles className="w-4 h-4 text-primary-foreground" />
            </div>
            <div className="flex flex-col">
              <span className="text-base font-semibold">Agent Chat</span>
              <span className="text-xs font-normal text-muted-foreground">
                AI-powered assistant with tools
              </span>
            </div>
          </span>
          <div className="flex items-center gap-2">
            {/* Pending tools badge */}
            <PendingToolsBadge count={pendingToolCalls.length} />
            <Select
              value={selectedModel}
              onChange={(e) => handleModelChange(e.target.value)}
              options={MODEL_OPTIONS}
              className="h-8 w-32 text-xs bg-background"
              disabled={isStreaming}
            />
            <Button
              variant="outline"
              size="icon"
              onClick={handleNewSession}
              title="New Session"
              className="h-8 w-8"
              disabled={isStreaming}
            >
              <Plus className="w-4 h-4" />
            </Button>
          </div>
        </CardTitle>
      </CardHeader>

      <CardContent className="flex-1 flex flex-col gap-3 min-h-0 p-4 pt-0">
        {/* Session List */}
        {sessions.length > 0 && (
          <div className="flex-shrink-0 flex gap-1.5 overflow-x-auto pb-1 scrollbar-thin">
            {sessions.map((session) => (
              <button
                key={session.id}
                onClick={() => selectSession(session.id)}
                className={cn(
                  'group flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium whitespace-nowrap transition-all',
                  currentSession?.id === session.id
                    ? 'bg-primary text-primary-foreground shadow-md shadow-primary/20'
                    : 'bg-muted/50 hover:bg-muted text-foreground'
                )}
              >
                <span className="truncate max-w-[80px]">
                  {session.model || 'Session'}
                </span>
                <Trash2
                  className={cn(
                    'w-3 h-3 opacity-0 group-hover:opacity-100 transition-opacity',
                    currentSession?.id === session.id
                      ? 'text-primary-foreground/70 hover:text-primary-foreground'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                  onClick={(e) => handleDeleteSession(session.id, e)}
                />
              </button>
            ))}
          </div>
        )}

        {/* Error display */}
        {error && (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-red-50 border border-red-200 dark:bg-red-900/20 dark:border-red-800 text-sm">
            <AlertCircle className="w-4 h-4 text-red-600 dark:text-red-400 flex-shrink-0" />
            <span className="text-red-700 dark:text-red-300">{error}</span>
          </div>
        )}

        {/* Messages Area */}
        <div className="flex-1 min-h-0 border rounded-xl bg-background/50 overflow-y-auto">
          <div className="p-4 space-y-4">
            {messages.length === 0 && !isLoading && (
              <div className="flex flex-col items-center justify-center text-muted-foreground py-12">
                <div className="w-14 h-14 rounded-2xl bg-muted flex items-center justify-center mb-3">
                  <MessageSquare className="w-7 h-7 opacity-50" />
                </div>
                <p className="font-medium text-sm">Start a conversation</p>
                <p className="text-xs text-muted-foreground mt-1">
                  Select a model and send your first message
                </p>
                <div className="mt-4 flex items-center gap-2 text-xs text-muted-foreground">
                  <Wrench className="w-4 h-4" />
                  <span>Tool calling enabled</span>
                </div>
              </div>
            )}

            {messages.map((message) => (
              <MessageBubble
                key={message.id}
                role={message.role}
                content={message.content}
                toolCalls={message.tool_calls}
                toolExecutions={
                  message.tool_calls
                    ? getToolExecutionsForMessage(message.tool_calls)
                    : undefined
                }
              />
            ))}

            {/* Streaming Content */}
            {isStreaming && streamingContent && (
              <MessageBubble
                role="assistant"
                content={streamingContent}
                isStreaming
              />
            )}

            {/* Tool execution indicator */}
            {isExecutingTool && toolExecutions.some((e) => e.status === 'running') && (
              <ToolExecutionIndicator
                toolName={
                  toolExecutions.find((e) => e.status === 'running')?.tool_name || 'tool'
                }
              />
            )}

            {isStreaming && !streamingContent && !isExecutingTool && (
              <div className="flex gap-2">
                <div className="flex-shrink-0 w-7 h-7 rounded-full bg-gradient-to-br from-primary/80 to-primary text-primary-foreground flex items-center justify-center">
                  <Bot className="w-3.5 h-3.5" />
                </div>
                <div className="message-bubble-assistant px-3.5 py-2.5 rounded-xl">
                  <TypingIndicator />
                </div>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>
        </div>

        {/* Input Area */}
        <div className="flex-shrink-0 flex gap-2">
          <div className="relative flex-1">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={
                currentSession
                  ? 'Type your message...'
                  : 'Create a session to start chatting'
              }
              disabled={!currentSession || isStreaming}
              className="flex-1 pr-10 h-10 bg-background"
            />
          </div>
          <Button
            onClick={handleSend}
            disabled={!input.trim() || !currentSession || isStreaming}
            size="icon"
            className="h-10 w-10 shadow-lg shadow-primary/20"
          >
            {isStreaming ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Send className="w-4 h-4" />
            )}
          </Button>
        </div>
      </CardContent>

      {/* Tool Approval Modal */}
      {showApprovalModal && currentApprovalToolCall && (
        <ToolApprovalModal
          toolCall={currentApprovalToolCall}
          onApprove={handleApprove}
          onClose={handleCloseApproval}
        />
      )}
    </Card>
  )
}