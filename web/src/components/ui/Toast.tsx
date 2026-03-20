import { useState, useEffect } from 'react'
import { X, CheckCircle, AlertCircle, Info } from 'lucide-react'
import { useNotificationStore, type Notification } from '@/stores/notificationStore'
import { useWebSocket } from '@/hooks/useWebSocket'
import type { WSMessage } from '@/types'

interface Toast {
  id: string
  type: 'success' | 'error' | 'info'
  title: string
  message: string
}

// Toast state management
let toastId = 0
const toastListeners: Set<(toasts: Toast[]) => void> = new Set()
let currentToasts: Toast[] = []

export function showToast(type: Toast['type'], title: string, message: string) {
  const id = `toast-${++toastId}`
  currentToasts = [...currentToasts, { id, type, title, message }]
  toastListeners.forEach(listener => listener([...currentToasts]))

  // Auto dismiss after 5 seconds
  setTimeout(() => {
    dismissToast(id)
  }, 5000)
}

export function dismissToast(id: string) {
  currentToasts = currentToasts.filter(t => t.id !== id)
  toastListeners.forEach(listener => listener([...currentToasts]))
}

export function ToastContainer() {
  const [toasts, setToasts] = useState<Toast[]>([])
  const addNotification = useNotificationStore.getState().addNotification

  // Subscribe to toast changes
  useEffect(() => {
    const listener = (newToasts: Toast[]) => {
      setToasts(newToasts)
    }
    toastListeners.add(listener)
    // Initialize with current toasts
    setToasts(currentToasts)
    return () => {
      toastListeners.delete(listener)
    }
  }, [])

  // Handle WebSocket notifications
  useWebSocket({
    channels: ['notifications'],
    onMessage: (message: WSMessage) => {
      if (message.type === 'notification') {
        const payload = message.payload as Notification & { notif_type?: string; created_at: number }
        const notifType = payload.notif_type || payload.type

        // Add to notification store
        addNotification({
          id: payload.id,
          type: notifType,
          title: payload.title,
          message: payload.message,
          data: payload.data,
          read: false,
          created_at: new Date(payload.created_at * 1000).toISOString(),
        })

        // Show toast
        const toastType: Toast['type'] =
          notifType === 'task_complete' ? 'success' :
          notifType === 'task_failed' ? 'error' : 'info'

        showToast(toastType, payload.title, payload.message)
      }
    },
  })

  const getIcon = (type: Toast['type']) => {
    switch (type) {
      case 'success':
        return <CheckCircle className="h-5 w-5 text-green-500" />
      case 'error':
        return <AlertCircle className="h-5 w-5 text-red-500" />
      default:
        return <Info className="h-5 w-5 text-blue-500" />
    }
  }

  if (toasts.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-[100] space-y-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="flex items-start gap-3 p-4 bg-background border rounded-lg shadow-lg max-w-sm animate-in slide-in-from-right-full"
        >
          {getIcon(toast.type)}
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium">{toast.title}</p>
            <p className="text-xs text-muted-foreground">{toast.message}</p>
          </div>
          <button
            onClick={() => dismissToast(toast.id)}
            className="text-muted-foreground hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ))}
    </div>
  )
}