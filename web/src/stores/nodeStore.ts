// web/src/stores/nodeStore.ts
import { create } from 'zustand';

// Node types from backend
type NodeType = 'sandbox' | 'docker' | 'physical' | 'cloud';

// Node status from backend
type NodeStatus = 'idle' | 'busy' | 'offline';

// Node capabilities
interface NodeCapabilities {
  browser?: boolean;
  gpu?: boolean;
  docker?: boolean;
  [key: string]: boolean | undefined;
}

// Node from backend
interface Node {
  id: string;
  name: string;
  type: NodeType;
  capabilities: NodeCapabilities;
  status: NodeStatus;
  currentAgentId?: string;
  endpoint: string;
  metadata: Record<string, unknown>;
  createdAt: string;
  lastSeen: string;
}

interface NodeState {
  nodes: Node[];
  selectedNode: Node | null;
  isLoading: boolean;
  error: string | null;

  // Actions
  fetchNodes: () => Promise<void>;
  setSelectedNode: (node: Node | null) => void;
  registerNode: (node: Partial<Node>) => Promise<Node>;
  updateNodeHeartbeat: (nodeId: string) => Promise<void>;
}

export const useNodeStore = create<NodeState>((set) => ({
  nodes: [],
  selectedNode: null,
  isLoading: false,
  error: null,

  fetchNodes: async () => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch('/api/nodes');
      if (!response.ok) throw new Error('Failed to fetch nodes');
      const data: Node[] = await response.json();
      set({ nodes: data, isLoading: false });
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
    }
  },

  setSelectedNode: (node) => set({ selectedNode: node }),

  registerNode: async (nodeData) => {
    set({ isLoading: true, error: null });
    try {
      const response = await fetch('/api/nodes/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(nodeData),
      });
      if (!response.ok) throw new Error('Failed to register node');
      const data: Node = await response.json();
      set((state) => ({
        nodes: [...state.nodes, data],
        isLoading: false,
      }));
      return data;
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error', isLoading: false });
      throw err;
    }
  },

  updateNodeHeartbeat: async (nodeId) => {
    try {
      const response = await fetch(`/api/nodes/${nodeId}/heartbeat`, {
        method: 'POST',
      });
      if (!response.ok) throw new Error('Failed to update heartbeat');
      // Update lastSeen locally
      set((state) => ({
        nodes: state.nodes.map((n) =>
          n.id === nodeId ? { ...n, lastSeen: new Date().toISOString() } : n
        ),
      }));
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Unknown error' });
    }
  },
}));