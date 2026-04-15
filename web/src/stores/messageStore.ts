// web/src/stores/messageStore.ts
import { create } from 'zustand';

// Message types from backend
type MessageType = 'notify' | 'question' | 'data' | 'request_approval';

// Message status from backend
type MessageStatus = 'pending' | 'delivered' | 'read' | 'responded';

// Agent message from backend
interface AgentMessage {
  id: string;
  fromAgent: string;
  proxyFor?: string; // Recovery agent identity
  toAgent: string;
  type: MessageType;
  content: string;
  requiresResponse: boolean;
  status: MessageStatus;
  response?: string;
  createdAt: string;
  deliveredAt?: string;
  respondedAt?: string;
}

interface MessageState {
  messages: AgentMessage[];
  selectedMessage: AgentMessage | null;
  isLoading: boolean;
  error: string | null;

  // Actions
  fetchMessages: (agentId: string) => Promise<void>;
  setSelectedMessage: (message: AgentMessage | null) => void;
  sendMessage: (fromAgent: string, toAgent: string, type: MessageType, content: string, requiresResponse?: boolean) => Promise<void>;
  respondToMessage: (messageId: string, response: string) => Promise<void>;
  markAsRead: (messageId: string) => Promise<void>;
}

export const useMessageStore = create<MessageState>((set, get) => ({
  messages: [],
  selectedMessage: null,
  isLoading: false,
  error: null,

  fetchMessages: async (agentId) => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch(`/api/messages/${agentId}`);
      if (!response.ok) throw new Error('Failed to fetch messages');
      const data: AgentMessage[] = await response.json();
      set({ messages: data, isLoading: false });
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
    }
  },

  setSelectedMessage: (message) => set({ selectedMessage: message }),

  sendMessage: async (fromAgent, toAgent, type, content, requiresResponse = false) => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch('/api/messages', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          from_agent: fromAgent,
          to_agent: toAgent,
          type,
          content,
          requires_response: requiresResponse,
        }),
      });
      if (!response.ok) throw new Error('Failed to send message');
      const data: AgentMessage = await response.json();
      set((state) => ({
        messages: [...state.messages, data],
        isLoading: false,
      }));
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
    }
  },

  respondToMessage: async (messageId, response) => {
    set({ isLoading: true, error: null });
    try {
      const msgResponse = await fetch(`/api/messages/${messageId}/respond`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ response }),
      });
      if (!msgResponse.ok) throw new Error('Failed to respond');
      const data: AgentMessage = await msgResponse.json();
      set((state) => ({
        messages: state.messages.map((m) => (m.id === messageId ? data : m)),
        selectedMessage: null,
        isLoading: false,
      }));
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
    }
  },

  markAsRead: async (messageId) => {
    try {
      const response = await fetch(`/api/messages/${messageId}/read`, {
        method: 'POST',
      });
      if (!response.ok) throw new Error('Failed to mark as read');
      set((state) => ({
        messages: state.messages.map((m) =>
          m.id === messageId ? { ...m, status: 'read' as MessageStatus } : m
        ),
      }));
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error' });
    }
  },
}));