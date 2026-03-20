import { create } from 'zustand'

export interface Notification {
  id: number
  type: string
  title: string
  message: string
  data?: Record<string, unknown>
  read: boolean
  created_at: string
}

interface NotificationState {
  notifications: Notification[]
  unreadCount: number
  isLoading: boolean

  setNotifications: (notifications: Notification[]) => void
  addNotification: (notification: Notification) => void
  markAsRead: (ids: number[]) => Promise<void>
  markAllAsRead: () => Promise<void>
  fetchNotifications: () => Promise<void>
  clear: () => void
}

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api'

export const useNotificationStore = create<NotificationState>((set, get) => ({
  notifications: [],
  unreadCount: 0,
  isLoading: false,

  setNotifications: (notifications) => {
    const unreadCount = notifications.filter(n => !n.read).length
    set({ notifications, unreadCount })
  },

  addNotification: (notification) => {
    set(state => {
      const exists = state.notifications.some(n => n.id === notification.id)
      if (exists) return state

      const notifications = [notification, ...state.notifications].slice(0, 100)
      const unreadCount = notifications.filter(n => !n.read).length
      return { notifications, unreadCount }
    })
  },

  markAsRead: async (ids) => {
    try {
      await fetch(`${API_URL}/notifications/read`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids }),
      })

      set(state => {
        const notifications = state.notifications.map(n =>
          ids.includes(n.id) ? { ...n, read: true } : n
        )
        const unreadCount = notifications.filter(n => !n.read).length
        return { notifications, unreadCount }
      })
    } catch (error) {
      console.error('Failed to mark notifications as read:', error)
    }
  },

  markAllAsRead: async () => {
    try {
      await fetch(`${API_URL}/notifications/read-all`, {
        method: 'PUT',
      })

      set(state => ({
        notifications: state.notifications.map(n => ({ ...n, read: true })),
        unreadCount: 0,
      }))
    } catch (error) {
      console.error('Failed to mark all notifications as read:', error)
    }
  },

  fetchNotifications: async () => {
    set({ isLoading: true })
    try {
      const response = await fetch(`${API_URL}/notifications`)
      const data = await response.json()
      if (data.success) {
        get().setNotifications(data.data)
      }
    } catch (error) {
      console.error('Failed to fetch notifications:', error)
    } finally {
      set({ isLoading: false })
    }
  },

  clear: () => {
    set({ notifications: [], unreadCount: 0 })
  },
}))