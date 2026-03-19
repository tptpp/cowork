import { create } from 'zustand'
import type { Worker } from '../types'

interface WorkerState {
  workers: Worker[]
  loading: boolean
  error: string | null
  fetchWorkers: () => Promise<void>
  updateWorker: (worker: Worker) => void
}

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080'

export const useWorkerStore = create<WorkerState>((set) => ({
  workers: [],
  loading: false,
  error: null,

  fetchWorkers: async () => {
    set({ loading: true, error: null })
    try {
      const response = await fetch(`${API_BASE}/api/workers`)
      const data = await response.json()
      if (data.success) {
        set({ workers: data.data, loading: false })
      } else {
        set({ error: data.error?.message || 'Failed to fetch workers', loading: false })
      }
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Network error', loading: false })
    }
  },

  updateWorker: (worker) => {
    set((state) => ({
      workers: state.workers.map((w) => (w.id === worker.id ? worker : w)),
    }))
  },
}))