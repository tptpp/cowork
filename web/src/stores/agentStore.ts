import { create } from 'zustand'
import type {
  AgentSession,
  AgentMessage,
  ToolCall,
  ToolExecution,
  ToolDefinition
} from '@/types'

// API base URL
const API_BASE = import.meta.env.VITE_API_BASE || ''

// SSE Response types
interface SSETokenResponse {
  type: 'token'
  content: string
}

interface SSEToolCallsResponse {
  type: 'tool_calls'
  tool_calls: ToolCall[]
}

interface SSEToolUpdateResponse {
  type: 'tool_update'
  tool_execution: ToolExecution
}

interface SSEDoneResponse {
  type: 'done'
  content: string
}

interface SSEErrorResponse {
  type: 'error'
  content: string
}

type SSEResponse =
  | SSETokenResponse
  | SSEToolCallsResponse
  | SSEToolUpdateResponse
  | SSEDoneResponse
  | SSEErrorResponse

interface AgentState {
  // State
  sessions: AgentSession[]
  currentSession: AgentSession | null
  messages: AgentMessage[]
  isLoading: boolean
  isStreaming: boolean
  streamingContent: string
  error: string | null

  // Tool Calling State
  pendingToolCalls: ToolCall[]
  toolExecutions: ToolExecution[]
  availableTools: ToolDefinition[]
  isExecutingTool: boolean

  // Actions
  fetchSessions: () => Promise<void>
  createSession: (model?: string, systemPrompt?: string) => Promise<AgentSession>
  selectSession: (id: string) => Promise<void>
  deleteSession: (id: string) => Promise<void>
  sendMessage: (content: string) => Promise<void>
  sendMessageWithTools: (
    content: string,
    tools?: string[],
    autoExecute?: boolean
  ) => Promise<void>
  approveToolCall: (toolCallId: string, approved: boolean) => Promise<void>
  fetchToolExecutions: (sessionId: string) => Promise<void>
  fetchAvailableTools: () => Promise<void>
  clearError: () => void
  clearToolState: () => void
}

export const useAgentStore = create<AgentState>((set, get) => ({
  // Initial state
  sessions: [],
  currentSession: null,
  messages: [],
  isLoading: false,
  isStreaming: false,
  streamingContent: '',
  error: null,

  // Tool Calling Initial State
  pendingToolCalls: [],
  toolExecutions: [],
  availableTools: [],
  isExecutingTool: false,

  // Fetch all sessions
  fetchSessions: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await fetch(`${API_BASE}/api/agent/sessions`)
      if (!response.ok) throw new Error('Failed to fetch sessions')

      const data = await response.json()
      set({ sessions: data.data || [], isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch sessions',
        isLoading: false
      })
    }
  },

  // Create a new session
  createSession: async (model = 'default', systemPrompt = '') => {
    set({ isLoading: true, error: null })
    try {
      const response = await fetch(`${API_BASE}/api/agent/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model,
          system_prompt: systemPrompt,
        }),
      })

      if (!response.ok) throw new Error('Failed to create session')

      const data = await response.json()
      const session = data.data as AgentSession

      set((state) => ({
        sessions: [session, ...state.sessions],
        currentSession: session,
        messages: [],
        isLoading: false,
        pendingToolCalls: [],
        toolExecutions: [],
      }))

      return session
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to create session',
        isLoading: false
      })
      throw error
    }
  },

  // Select a session and load its messages
  selectSession: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const response = await fetch(`${API_BASE}/api/agent/sessions/${id}`)
      if (!response.ok) throw new Error('Failed to fetch session')

      const data = await response.json()
      set({
        currentSession: data.data.session,
        messages: data.data.messages || [],
        isLoading: false,
      })

      // Also fetch tool executions for this session
      get().fetchToolExecutions(id)
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch session',
        isLoading: false
      })
    }
  },

  // Delete a session
  deleteSession: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const response = await fetch(`${API_BASE}/api/agent/sessions/${id}`, {
        method: 'DELETE',
      })

      if (!response.ok) throw new Error('Failed to delete session')

      set((state) => ({
        sessions: state.sessions.filter((s) => s.id !== id),
        currentSession: state.currentSession?.id === id ? null : state.currentSession,
        messages: state.currentSession?.id === id ? [] : state.messages,
        isLoading: false,
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete session',
        isLoading: false
      })
    }
  },

  // Send a message and handle streaming response (legacy, no tools)
  sendMessage: async (content: string) => {
    const { currentSession } = get()
    if (!currentSession) {
      set({ error: 'No active session' })
      return
    }

    // Add user message optimistically
    const userMessage: AgentMessage = {
      id: Date.now(),
      session_id: currentSession.id,
      role: 'user',
      content,
      tokens: 0,
      created_at: new Date().toISOString(),
    }

    set((state) => ({
      messages: [...state.messages, userMessage],
      isStreaming: true,
      streamingContent: '',
      error: null,
    }))

    try {
      const response = await fetch(
        `${API_BASE}/api/agent/sessions/${currentSession.id}/messages`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ content }),
        }
      )

      if (!response.ok) throw new Error('Failed to send message')

      // Handle SSE stream
      const reader = response.body?.getReader()
      const decoder = new TextDecoder()

      if (!reader) {
        throw new Error('No response stream')
      }

      let fullContent = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        const chunk = decoder.decode(value, { stream: true })
        const lines = chunk.split('\n')

        for (const line of lines) {
          if (line.startsWith('data:')) {
            try {
              const data = JSON.parse(line.slice(5).trim()) as SSEResponse
              if (data.type === 'token') {
                fullContent += data.content
                set({ streamingContent: fullContent })
              } else if (data.type === 'done') {
                // Add assistant message
                const assistantMessage: AgentMessage = {
                  id: Date.now() + 1,
                  session_id: currentSession.id,
                  role: 'assistant',
                  content: data.content,
                  tokens: 0,
                  created_at: new Date().toISOString(),
                }

                set((state) => ({
                  messages: [...state.messages, assistantMessage],
                  isStreaming: false,
                  streamingContent: '',
                }))
              }
            } catch {
              // Skip invalid JSON
            }
          }
        }
      }
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to send message',
        isStreaming: false,
        streamingContent: '',
      })
    }
  },

  // Send a message with tool support (Function Calling)
  sendMessageWithTools: async (
    content: string,
    tools?: string[],
    autoExecute: boolean = true
  ) => {
    const { currentSession } = get()
    if (!currentSession) {
      set({ error: 'No active session' })
      return
    }

    // Add user message optimistically
    const userMessage: AgentMessage = {
      id: Date.now(),
      session_id: currentSession.id,
      role: 'user',
      content,
      tokens: 0,
      created_at: new Date().toISOString(),
    }

    set((state) => ({
      messages: [...state.messages, userMessage],
      isStreaming: true,
      streamingContent: '',
      pendingToolCalls: [],
      error: null,
    }))

    try {
      const response = await fetch(
        `${API_BASE}/api/agent/sessions/${currentSession.id}/messages/tools`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            content,
            tools,
            auto_execute_tools: autoExecute,
          }),
        }
      )

      if (!response.ok) throw new Error('Failed to send message')

      // Handle SSE stream
      const reader = response.body?.getReader()
      const decoder = new TextDecoder()

      if (!reader) {
        throw new Error('No response stream')
      }

      let fullContent = ''
      let pendingToolCalls: ToolCall[] = []

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        const chunk = decoder.decode(value, { stream: true })
        const lines = chunk.split('\n')

        for (const line of lines) {
          if (line.startsWith('data:')) {
            try {
              const data = JSON.parse(line.slice(5).trim()) as SSEResponse

              switch (data.type) {
                case 'token':
                  fullContent += data.content
                  set({ streamingContent: fullContent })
                  break

                case 'tool_calls':
                  pendingToolCalls = data.tool_calls
                  set({ pendingToolCalls: data.tool_calls, isExecutingTool: true })

                  // Add assistant message with tool calls
                  const assistantMessage: AgentMessage = {
                    id: Date.now() + 1,
                    session_id: currentSession.id,
                    role: 'assistant',
                    content: '',
                    tool_calls: data.tool_calls,
                    tokens: 0,
                    created_at: new Date().toISOString(),
                  }
                  set((state) => ({
                    messages: [...state.messages, assistantMessage],
                  }))
                  break

                case 'tool_update':
                  // Update tool execution status
                  set((state) => {
                    const executions = [...state.toolExecutions]
                    const idx = executions.findIndex(
                      (e) => e.tool_call_id === data.tool_execution.tool_call_id
                    )
                    if (idx >= 0) {
                      executions[idx] = data.tool_execution
                    } else {
                      executions.push(data.tool_execution)
                    }
                    return { toolExecutions: executions }
                  })
                  break

                case 'done':
                  // Add final assistant message if there's content
                  if (data.content) {
                    const finalMessage: AgentMessage = {
                      id: Date.now() + 2,
                      session_id: currentSession.id,
                      role: 'assistant',
                      content: data.content,
                      tokens: 0,
                      created_at: new Date().toISOString(),
                    }
                    set((state) => ({
                      messages: [...state.messages, finalMessage],
                      isStreaming: false,
                      streamingContent: '',
                      isExecutingTool: false,
                    }))
                  } else {
                    set({
                      isStreaming: false,
                      streamingContent: '',
                      isExecutingTool: false,
                    })
                  }
                  break

                case 'error':
                  set({
                    error: data.content,
                    isStreaming: false,
                    streamingContent: '',
                    isExecutingTool: false,
                  })
                  break
              }
            } catch {
              // Skip invalid JSON
            }
          }
        }
      }
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to send message',
        isStreaming: false,
        streamingContent: '',
        isExecutingTool: false,
      })
    }
  },

  // Approve or reject a tool call (Human-in-loop)
  approveToolCall: async (toolCallId: string, approved: boolean) => {
    const { currentSession } = get()
    if (!currentSession) return

    try {
      const response = await fetch(
        `${API_BASE}/api/agent/sessions/${currentSession.id}/tools/execute`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            tool_call_id: toolCallId,
            approved,
          }),
        }
      )

      if (!response.ok) throw new Error('Failed to execute tool')

      // Remove from pending list
      set((state) => ({
        pendingToolCalls: state.pendingToolCalls.filter(
          (tc) => tc.id !== toolCallId
        ),
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to approve tool call',
      })
    }
  },

  // Fetch tool executions for a session
  fetchToolExecutions: async (sessionId: string) => {
    try {
      const response = await fetch(
        `${API_BASE}/api/agent/sessions/${sessionId}/tools/executions`
      )
      if (!response.ok) throw new Error('Failed to fetch tool executions')

      const data = await response.json()
      set({ toolExecutions: data.data || [] })
    } catch (error) {
      console.error('Failed to fetch tool executions:', error)
    }
  },

  // Fetch available tools
  fetchAvailableTools: async () => {
    try {
      const response = await fetch(`${API_BASE}/api/tools`)
      if (!response.ok) throw new Error('Failed to fetch tools')

      const data = await response.json()
      set({ availableTools: data.data || [] })
    } catch (error) {
      console.error('Failed to fetch available tools:', error)
    }
  },

  // Clear error
  clearError: () => set({ error: null }),

  // Clear tool state
  clearToolState: () => set({
    pendingToolCalls: [],
    toolExecutions: [],
    isExecutingTool: false,
  }),
}))