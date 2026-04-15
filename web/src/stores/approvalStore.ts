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

interface ApprovalState {
  pendingApprovals: ApprovalRequest[];
  selectedApproval: ApprovalRequest | null;
  isLoading: boolean;
  error: string | null;

  // Actions
  fetchPendingApprovals: () => Promise<void>;
  setSelectedApproval: (approval: ApprovalRequest | null) => void;
  approveRequest: (id: string, userId: string) => Promise<void>;
  rejectRequest: (id: string, userId: string) => Promise<void>;
  createRequest: (agentId: string, action: string, detail: Record<string, unknown>) => Promise<ApprovalRequest>;
}

export const useApprovalStore = create<ApprovalState>((set, get) => ({
  pendingApprovals: [],
  selectedApproval: null,
  isLoading: false,
  error: null,

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