import { useState, useRef, useEffect } from 'react'
import { Send, Plus, Trash2, MessageSquare, Loader2, Bot, User, Sparkles } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { useAgentStore } from '@/stores/agentStore'
import { cn } from '@/lib/utils'

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

// Message bubble component
function MessageBubble({
  role,
  content,
  isStreaming
}: {
  role: 'user' | 'assistant' | 'system'
  content: string
  isStreaming?: boolean
}) {
  const isUser = role === 'user'

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
    </div>
  )
}

export function AgentChatWidget() {
  const [input, setInput] = useState('')
  const [selectedModel, setSelectedModel] = useState('default')
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const {
    sessions,
    currentSession,
    messages,
    isStreaming,
    streamingContent,
    isLoading,
    fetchSessions,
    createSession,
    selectSession,
    deleteSession,
    sendMessage,
  } = useAgentStore()

  // Fetch sessions on mount
  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  const handleSend = async () => {
    if (!input.trim() || isStreaming) return
    const content = input.trim()
    setInput('')
    await sendMessage(content)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleNewSession = async () => {
    await createSession(selectedModel)
  }

  const handleModelChange = async (newModel: string) => {
    setSelectedModel(newModel)
    await createSession(newModel)
  }

  const handleDeleteSession = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    await deleteSession(id)
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
                AI-powered assistant
              </span>
            </div>
          </span>
          <div className="flex items-center gap-2">
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
              </div>
            )}

            {messages.map((message) => (
              <MessageBubble
                key={message.id}
                role={message.role}
                content={message.content}
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

            {isStreaming && !streamingContent && (
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
    </Card>
  )
}