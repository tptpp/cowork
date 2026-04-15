// web/src/stores/approvalStore.ts
import { create } from 'zustand';

// Risk levels from backend
type RiskLevel = 'low' | 'medium' | 'high';

// Approval status from backend
type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'expired';

// Approval request from backend
interface ApprovalRequest {
  id: string;
  agentId: string;
  action: string;
  actionDetail: Record<string, unknown>;
  riskLevel: RiskLevel;
  status: ApprovalStatus;
  userId?: string;
  timeoutSeconds?: number;
  createdAt: string;
  resolvedAt?: string;
}

// WebSocket message types
interface WebSocketMessage {
  type: string;
  payload?: ApprovalRequest;
}

interface ApprovalState {
  pendingApprovals: ApprovalRequest[];
  selectedApproval: ApprovalRequest | null;
  isLoading: boolean;
  error: string | null;
  ws: WebSocket | null;
  wsConnected: boolean;

  // Actions
  fetchPendingApprovals: () => Promise<void>;
  setSelectedApproval: (approval: ApprovalRequest | null) => void;
  approveRequest: (id: string, userId: string) => Promise<void>;
  rejectRequest: (id: string, userId: string) => Promise<void>;
  createRequest: (agentId: string, action: string, detail: Record<string, unknown>) => Promise<ApprovalRequest>;
  connectWebSocket: () => void;
  disconnectWebSocket: () => void;
  handleApprovalRequest: (approval: ApprovalRequest) => void;
}

export const useApprovalStore = create<ApprovalState>((set, get) => ({
  pendingApprovals: [],
  selectedApproval: null,
  isLoading: false,
  error: null,
  ws: null,
  wsConnected: false,

  connectWebSocket: () => {
    const existingWs = get().ws;
    if (existingWs) {
      return; // Already connected
    }

    // Determine WebSocket URL based on current location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      set({ wsConnected: true });
      // Subscribe to approvals channel
      ws.send(JSON.stringify({
        type: 'subscribe',
        payload: ['approvals']
      }));
    };

    ws.onmessage = (event) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data);
        if (message.type === 'approval_request' && message.payload) {
          get().handleApprovalRequest(message.payload);
        }
      } catch {
        // Ignore parse errors
      }
    };

    ws.onclose = () => {
      set({ wsConnected: false, ws: null });
      // Attempt to reconnect after 5 seconds
      setTimeout(() => {
        if (!get().ws) {
          get().connectWebSocket();
        }
      }, 5000);
    };

    ws.onerror = () => {
      // Error occurred, will close and reconnect
    };

    set({ ws });
  },

  disconnectWebSocket: () => {
    const ws = get().ws;
    if (ws) {
      ws.close();
      set({ ws: null, wsConnected: false });
    }
  },

  handleApprovalRequest: (approval: ApprovalRequest) => {
    set((state) => {
      // Check if approval already exists
      const exists = state.pendingApprovals.some(a => a.id === approval.id);
      if (exists) {
        // Update existing approval
        return {
          pendingApprovals: state.pendingApprovals.map(a =>
            a.id === approval.id ? approval : a
          )
        };
      }
      // Add new pending approval
      return {
        pendingApprovals: [approval, ...state.pendingApprovals]
      };
    });
  },

  fetchPendingApprovals: async () => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch('/api/approvals/pending');
      if (!response.ok) throw new Error('Failed to fetch approvals');
      const data: ApprovalRequest[] = await response.json();
      set({ pendingApprovals: data, isLoading: false });
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
    }
  },

  setSelectedApproval: (approval) => set({ selectedApproval: approval }),

  approveRequest: async (id, userId) => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch(`/api/approvals/${id}/approve`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId }),
      });
      if (!response.ok) throw new Error('Failed to approve');
      // Remove from pending list
      set((state) => ({
        pendingApprovals: state.pendingApprovals.filter((a) => a.id !== id),
        selectedApproval: null,
        isLoading: false,
      }));
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
    }
  },

  rejectRequest: async (id, userId) => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch(`/api/approvals/${id}/reject`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId }),
      });
      if (!response.ok) throw new Error('Failed to reject');
      // Remove from pending list
      set((state) => ({
        pendingApprovals: state.pendingApprovals.filter((a) => a.id !== id),
        selectedApproval: null,
        isLoading: false,
      }));
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
    }
  },

  createRequest: async (agentId, action, detail) => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch('/api/approvals', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agent_id: agentId, action, detail }),
      });
      if (!response.ok) throw new Error('Failed to create approval request');
      const data: ApprovalRequest = await response.json();
      set((state) => ({
        pendingApprovals: [...state.pendingApprovals, data],
        isLoading: false,
      }));
      return data;
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
      throw err;
    }
  },
}));