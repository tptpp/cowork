import { create } from 'zustand'
import type { Task, TaskStatus } from '../types'

export interface TaskLog {
  id: string
  task_id: string
  level: string
  message: string
  timestamp: string
}

interface TaskState {
  tasks: Task[]
  currentTask: Task | null
  logs: TaskLog[]
  loading: boolean
  error: string | null
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
  fetchTasks: (params?: { status?: string; page?: number }) => Promise<void>
  fetchTask: (id: string) => Promise<Task | null>
  createTask: (task: Partial<Task>) => Promise<Task | null>
  cancelTask: (id: string) => Promise<boolean>
  fetchLogs: (taskId: string) => Promise<void>
  updateTask: (task: Task) => void
}

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080'

export const useTaskStore = create<TaskState>((set, get) => ({
  tasks: [],
  currentTask: null,
  logs: [],
  loading: false,
  error: null,
  pagination: {
    page: 1,
    page_size: 20,
    total: 0,
    total_pages: 0,
  },

  fetchTasks: async (params) => {
    set({ loading: true, error: null })
    try {
      const searchParams = new URLSearchParams()
      if (params?.status) searchParams.set('status', params.status)
      if (params?.page) searchParams.set('page', params.page.toString())

      const response = await fetch(`${API_BASE}/api/tasks?${searchParams}`)
      const data = await response.json()
      if (data.success) {
        set({
          tasks: data.data,
          pagination: {
            page: data.pagination?.page || 1,
            page_size: data.pagination?.page_size || 20,
            total: data.pagination?.total || 0,
            total_pages: data.pagination?.total_pages || 0,
          },
          loading: false,
        })
      } else {
        set({ error: data.error?.message || 'Failed to fetch tasks', loading: false })
      }
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Network error', loading: false })
    }
  },

  fetchTask: async (id) => {
    try {
      const response = await fetch(`${API_BASE}/api/tasks/${id}`)
      const data = await response.json()
      if (data.success) {
        const task = data.data as Task
        set({ currentTask: task })
        return task
      }
      return null
    } catch {
      return null
    }
  },

  createTask: async (task) => {
    try {
      const response = await fetch(`${API_BASE}/api/tasks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(task),
      })
      const data = await response.json()
      if (data.success) {
        const newTask = data.data as Task
        set({ tasks: [newTask, ...get().tasks] })
        return newTask
      }
      return null
    } catch {
      return null
    }
  },

  cancelTask: async (id) => {
    try {
      const response = await fetch(`${API_BASE}/api/tasks/${id}/cancel`, {
        method: 'POST',
      })
      const data = await response.json()
      if (data.success) {
        // Update the task in the list
        const task = get().tasks.find((t) => t.id === id)
        if (task) {
          set({
            tasks: get().tasks.map((t) =>
              t.id === id ? { ...t, status: 'cancelled' as TaskStatus } : t
            ),
          })
        }
        if (get().currentTask?.id === id) {
          set({ currentTask: { ...get().currentTask!, status: 'cancelled' as TaskStatus } })
        }
        return true
      }
      return false
    } catch {
      return false
    }
  },

  fetchLogs: async (taskId) => {
    try {
      const response = await fetch(`${API_BASE}/api/tasks/${taskId}/logs`)
      const data = await response.json()
      if (data.success) {
        set({ logs: data.data || [] })
      } else {
        set({ logs: [] })
      }
    } catch {
      set({ logs: [] })
    }
  },

  updateTask: (task) => {
    set((state) => ({
      tasks: state.tasks.map((t) => (t.id === task.id ? task : t)),
    }))
  },
}))