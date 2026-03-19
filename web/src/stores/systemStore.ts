import { create } from 'zustand'
import type { SystemStats } from '../types'

interface SystemState {
  stats: SystemStats | null
  loading: boolean
  error: string | null
  fetchStats: () => Promise<void>
}

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080'

export const useSystemStore = create<SystemState>((set) => ({
  stats: null,
  loading: false,
  error: null,

  fetchStats: async () => {
    set({ loading: true, error: null })
    try {
      const response = await fetch(`${API_BASE}/api/system/stats`)
      const data = await response.json()
      if (data.success) {
        set({ stats: data.data, loading: false })
      } else {
        set({ error: data.error?.message || 'Failed to fetch stats', loading: false })
      }
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Network error', loading: false })
    }
  },
}))