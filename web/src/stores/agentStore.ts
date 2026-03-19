import { create } from 'zustand'
import type { AgentSession, AgentMessage } from '@/types'

// API base URL
const API_BASE = import.meta.env.VITE_API_BASE || ''

interface AgentState {
  // State
  sessions: AgentSession[]
  currentSession: AgentSession | null
  messages: AgentMessage[]
  isLoading: boolean
  isStreaming: boolean
  streamingContent: string
  error: string | null

  // Actions
  fetchSessions: () => Promise<void>
  createSession: (model?: string, systemPrompt?: string) => Promise<AgentSession>
  selectSession: (id: string) => Promise<void>
  deleteSession: (id: string) => Promise<void>
  sendMessage: (content: string) => Promise<void>
  clearError: () => void
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

  // Send a message and handle streaming response
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
              const data = JSON.parse(line.slice(5).trim())
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

  // Clear error
  clearError: () => set({ error: null }),
}))