import { create } from 'zustand'
import type { TaskFile, UploadedFile } from '@/types'

// API base URL
const API_BASE = import.meta.env.VITE_API_BASE || ''

interface FileState {
  // State
  uploadingFiles: Map<string, number> // file name -> progress (0-100)
  error: string | null

  // Actions
  uploadFile: (file: File) => Promise<UploadedFile | null>
  uploadTaskFile: (taskId: string, file: File) => Promise<UploadedFile | null>
  uploadAgentFile: (sessionId: string, file: File) => Promise<UploadedFile | null>
  downloadFile: (id: number | string) => Promise<void>
  fetchTaskFiles: (taskId: string) => Promise<TaskFile[]>
  deleteFile: (id: number | string) => Promise<boolean>
  clearError: () => void
}

export const useFileStore = create<FileState>((set, get) => ({
  // Initial state
  uploadingFiles: new Map(),
  error: null,

  // Upload a general file
  uploadFile: async (file: File) => {
    set({ error: null })

    // Add to uploading files
    set((state) => {
      const newMap = new Map(state.uploadingFiles)
      newMap.set(file.name, 0)
      return { uploadingFiles: newMap }
    })

    try {
      const formData = new FormData()
      formData.append('file', file)

      const response = await fetch(`${API_BASE}/api/files/upload`, {
        method: 'POST',
        body: formData,
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.error?.message || 'Failed to upload file')
      }

      const data = await response.json()

      // Remove from uploading files
      set((state) => {
        const newMap = new Map(state.uploadingFiles)
        newMap.delete(file.name)
        return { uploadingFiles: newMap }
      })

      return data.data as UploadedFile
    } catch (error) {
      // Remove from uploading files
      set((state) => {
        const newMap = new Map(state.uploadingFiles)
        newMap.delete(file.name)
        return { uploadingFiles: newMap }
      })

      set({
        error: error instanceof Error ? error.message : 'Failed to upload file',
      })
      return null
    }
  },

  // Upload a file for a specific task
  uploadTaskFile: async (taskId: string, file: File) => {
    set({ error: null })

    // Add to uploading files
    set((state) => {
      const newMap = new Map(state.uploadingFiles)
      newMap.set(file.name, 0)
      return { uploadingFiles: newMap }
    })

    try {
      const formData = new FormData()
      formData.append('file', file)

      const response = await fetch(`${API_BASE}/api/tasks/${taskId}/files`, {
        method: 'POST',
        body: formData,
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.error?.message || 'Failed to upload file')
      }

      const data = await response.json()

      // Remove from uploading files
      set((state) => {
        const newMap = new Map(state.uploadingFiles)
        newMap.delete(file.name)
        return { uploadingFiles: newMap }
      })

      return data.data as UploadedFile
    } catch (error) {
      // Remove from uploading files
      set((state) => {
        const newMap = new Map(state.uploadingFiles)
        newMap.delete(file.name)
        return { uploadingFiles: newMap }
      })

      set({
        error: error instanceof Error ? error.message : 'Failed to upload file',
      })
      return null
    }
  },

  // Upload a file for an agent session
  uploadAgentFile: async (sessionId: string, file: File) => {
    set({ error: null })

    // Add to uploading files
    set((state) => {
      const newMap = new Map(state.uploadingFiles)
      newMap.set(file.name, 0)
      return { uploadingFiles: newMap }
    })

    try {
      const formData = new FormData()
      formData.append('file', file)

      const response = await fetch(
        `${API_BASE}/api/agent/sessions/${sessionId}/files`,
        {
          method: 'POST',
          body: formData,
        }
      )

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.error?.message || 'Failed to upload file')
      }

      const data = await response.json()

      // Remove from uploading files
      set((state) => {
        const newMap = new Map(state.uploadingFiles)
        newMap.delete(file.name)
        return { uploadingFiles: newMap }
      })

      return data.data as UploadedFile
    } catch (error) {
      // Remove from uploading files
      set((state) => {
        const newMap = new Map(state.uploadingFiles)
        newMap.delete(file.name)
        return { uploadingFiles: newMap }
      })

      set({
        error: error instanceof Error ? error.message : 'Failed to upload file',
      })
      return null
    }
  },

  // Download a file
  downloadFile: async (id: number | string) => {
    try {
      const response = await fetch(`${API_BASE}/api/files/${id}`)
      if (!response.ok) throw new Error('Failed to download file')

      // Get filename from Content-Disposition header
      const contentDisposition = response.headers.get('Content-Disposition')
      let filename = `file-${id}`
      if (contentDisposition) {
        const match = contentDisposition.match(/filename=(.+)/)
        if (match) {
          filename = match[1]
        }
      }

      // Create blob and download
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to download file',
      })
    }
  },

  // Fetch files for a task
  fetchTaskFiles: async (taskId: string) => {
    try {
      const response = await fetch(`${API_BASE}/api/tasks/${taskId}/files`)
      if (!response.ok) throw new Error('Failed to fetch task files')

      const data = await response.json()
      return data.data?.files || []
    } catch (error) {
      console.error('Failed to fetch task files:', error)
      return []
    }
  },

  // Delete a file
  deleteFile: async (id: number | string) => {
    try {
      const response = await fetch(`${API_BASE}/api/files/${id}`, {
        method: 'DELETE',
      })
      if (!response.ok) throw new Error('Failed to delete file')

      return true
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete file',
      })
      return false
    }
  },

  // Clear error
  clearError: () => set({ error: null }),
}))

// Utility function to format file size
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${units[i]}`
}