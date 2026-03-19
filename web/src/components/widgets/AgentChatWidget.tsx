import { useState, useRef, useEffect } from 'react'
import { Send, Plus, Trash2, MessageSquare, Loader2 } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { useAgentStore } from '@/stores/agentStore'
import { cn } from '@/lib/utils'

const MODEL_OPTIONS = [
  { value: 'default', label: 'Default' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'glm', label: 'GLM' },
]

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
    // Create a new session with the selected model
    await createSession(newModel)
  }

  const handleDeleteSession = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    await deleteSession(id)
  }

  // Sync selected model with current session
  useEffect(() => {
    if (currentSession?.model) {
      setSelectedModel(currentSession.model)
    }
  }, [currentSession?.model])

  return (
    <Card className="h-full flex flex-col">
      <CardHeader className="flex-shrink-0 pb-2">
        <CardTitle className="flex items-center justify-between">
          <span className="flex items-center gap-2">
            <MessageSquare className="w-5 h-5" />
            Agent Chat
          </span>
          <div className="flex items-center gap-2">
            <Select
              value={selectedModel}
              onChange={(e) => handleModelChange(e.target.value)}
              options={MODEL_OPTIONS}
              className="h-8 w-28 text-xs"
              disabled={isStreaming}
            />
            <Button variant="ghost" size="icon" onClick={handleNewSession} title="New Session">
              <Plus className="w-4 h-4" />
            </Button>
          </div>
        </CardTitle>
      </CardHeader>

      <CardContent className="flex-1 flex flex-col gap-2 min-h-0 p-4 pt-0">
        {/* Session List */}
        {sessions.length > 0 && (
          <div className="flex-shrink-0 flex gap-1 overflow-x-auto pb-2">
            {sessions.map((session) => (
              <div
                key={session.id}
                className={cn(
                  'flex items-center gap-1 px-2 py-1 rounded-md text-sm cursor-pointer whitespace-nowrap',
                  currentSession?.id === session.id
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted hover:bg-muted/80'
                )}
                onClick={() => selectSession(session.id)}
              >
                <span className="truncate max-w-[100px]">
                  Session {session.id.slice(0, 4)}
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-4 w-4 p-0 opacity-60 hover:opacity-100"
                  onClick={(e) => handleDeleteSession(session.id, e)}
                >
                  <Trash2 className="w-3 h-3" />
                </Button>
              </div>
            ))}
          </div>
        )}

        {/* Messages Area */}
        <div className="flex-1 min-h-0 border rounded-md overflow-y-auto">
          <div className="p-3 space-y-3">
            {messages.length === 0 && !isLoading && (
              <div className="text-center text-muted-foreground py-8">
                <MessageSquare className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p>No messages yet</p>
                <p className="text-sm">Start a conversation with the agent</p>
              </div>
            )}

            {messages.map((message) => (
              <div
                key={message.id}
                className={cn(
                  'flex',
                  message.role === 'user' ? 'justify-end' : 'justify-start'
                )}
              >
                <div
                  className={cn(
                    'max-w-[80%] rounded-lg px-3 py-2 text-sm',
                    message.role === 'user'
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-muted'
                  )}
                >
                  {message.content}
                </div>
              </div>
            ))}

            {/* Streaming Content */}
            {isStreaming && streamingContent && (
              <div className="flex justify-start">
                <div className="max-w-[80%] rounded-lg px-3 py-2 text-sm bg-muted">
                  {streamingContent}
                  <span className="animate-pulse">|</span>
                </div>
              </div>
            )}

            {isStreaming && !streamingContent && (
              <div className="flex justify-start">
                <div className="max-w-[80%] rounded-lg px-3 py-2 text-sm bg-muted">
                  <Loader2 className="w-4 h-4 animate-spin" />
                </div>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>
        </div>

        {/* Input Area */}
        <div className="flex-shrink-0 flex gap-2">
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={
              currentSession
                ? 'Type a message...'
                : 'Create or select a session first'
            }
            disabled={!currentSession || isStreaming}
            className="flex-1"
          />
          <Button
            onClick={handleSend}
            disabled={!input.trim() || !currentSession || isStreaming}
            size="icon"
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