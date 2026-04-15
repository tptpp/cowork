// web/src/stores/taskTreeStore.ts
import { create } from 'zustand';

interface TaskNode {
  id: string;
  rootId: string;
  parentId: string | null;
  templateId: string;
  title: string;
  description: string;
  status: 'pending' | 'running' | 'done' | 'failed';
  progress: number;
  milestone: Record<string, 'done' | 'running' | 'pending'>;
  dependsOn: string[];
  requires: string[];
  children: string[];
}

interface TaskTreeState {
  tasks: Record<string, TaskNode>;
  rootTaskId: string | null;
  expandedNodes: Set<string>;

  // Actions
  setTasks: (tasks: TaskNode[]) => void;
  setRootTaskId: (id: string | null) => void;
  updateTask: (id: string, updates: Partial<TaskNode>) => void;
  toggleExpand: (id: string) => void;
  expandAll: () => void;
  collapseAll: () => void;

  // Computed
  getTaskTree: () => TaskNode | null;
  getTaskById: (id: string) => TaskNode | undefined;
  getChildTasks: (parentId: string) => TaskNode[];
  getDependencyGraph: () => { nodes: TaskNode[]; edges: { from: string; to: string }[] };
}

export const useTaskTreeStore = create<TaskTreeState>((set, get) => ({
  tasks: {},
  rootTaskId: null,
  expandedNodes: new Set(),

  setTasks: (tasks) => {
    const taskMap: Record<string, TaskNode> = {};
    tasks.forEach(t => taskMap[t.id] = t);
    const rootId = tasks.find(t => t.rootId === t.id && !t.parentId)?.id || null;
    set({ tasks: taskMap, rootTaskId: rootId, expandedNodes: new Set([rootId || '']) });
  },

  setRootTaskId: (id) => set({ rootTaskId: id }),

  updateTask: (id, updates) => set((state) => ({
    tasks: {
      ...state.tasks,
      [id]: { ...state.tasks[id], ...updates } as TaskNode,
    },
  })),

  toggleExpand: (id) => set((state) => {
    const newExpanded = new Set(state.expandedNodes);
    if (newExpanded.has(id)) {
      newExpanded.delete(id);
    } else {
      newExpanded.add(id);
    }
    return { expandedNodes: newExpanded };
  }),

  expandAll: () => set((state) => ({
    expandedNodes: new Set(Object.keys(state.tasks)),
  })),

  collapseAll: () => set({ expandedNodes: new Set([get().rootTaskId || '']) }),

  getTaskTree: () => {
    const state = get();
    if (!state.rootTaskId) return null;
    return state.tasks[state.rootTaskId];
  },

  getTaskById: (id) => get().tasks[id],

  getChildTasks: (parentId) => {
    const state = get();
    return Object.values(state.tasks).filter(t => t.parentId === parentId);
  },

  getDependencyGraph: () => {
    const state = get();
    const nodes = Object.values(state.tasks);
    const edges: { from: string; to: string }[] = [];

    nodes.forEach(node => {
      node.dependsOn.forEach(depId => {
        edges.push({ from: node.id, to: depId });
      });
    });

    return { nodes, edges };
  },
}));