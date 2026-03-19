import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { WidgetConfig, WidgetType, GridLayoutItem, WidgetDefinition } from '@/types'

// API base URL
const API_BASE = import.meta.env.VITE_API_BASE || ''

// Widget registry with definitions
export const WIDGET_REGISTRY: Record<WidgetType, WidgetDefinition> = {
  'system-stats': {
    type: 'system-stats',
    name: 'System Stats',
    icon: 'BarChart3',
    description: 'Display system statistics',
    defaultSize: { w: 12, h: 2 },
    minSize: { w: 6, h: 2 },
  },
  'task-queue': {
    type: 'task-queue',
    name: 'Task Queue',
    icon: 'ListTodo',
    description: 'Display running and pending tasks',
    defaultSize: { w: 6, h: 4 },
    minSize: { w: 4, h: 3 },
  },
  'worker-status': {
    type: 'worker-status',
    name: 'Worker Status',
    icon: 'Users',
    description: 'Display worker status',
    defaultSize: { w: 6, h: 4 },
    minSize: { w: 4, h: 3 },
  },
  'agent-chat': {
    type: 'agent-chat',
    name: 'Agent Chat',
    icon: 'MessageSquare',
    description: 'Chat with AI agent',
    defaultSize: { w: 6, h: 5 },
    minSize: { w: 4, h: 4 },
  },
}

// Default widgets configuration
const DEFAULT_WIDGETS: WidgetConfig[] = [
  {
    id: 'widget-system-stats',
    type: 'system-stats',
    title: 'System Stats',
    layout: { x: 0, y: 0, w: 12, h: 2 },
  },
  {
    id: 'widget-task-queue',
    type: 'task-queue',
    title: 'Task Queue',
    layout: { x: 0, y: 2, w: 6, h: 4 },
  },
  {
    id: 'widget-worker-status',
    type: 'worker-status',
    title: 'Worker Status',
    layout: { x: 6, y: 2, w: 6, h: 4 },
  },
]

interface LayoutState {
  widgets: WidgetConfig[]
  isEditing: boolean
  isLoaded: boolean

  // Actions
  setWidgets: (widgets: WidgetConfig[]) => void
  addWidget: (type: WidgetType) => string
  removeWidget: (id: string) => void
  updateLayout: (id: string, layout: GridLayoutItem) => void
  toggleEditing: () => void
  resetToDefault: () => void

  // Persistence
  loadLayout: () => Promise<void>
  saveLayout: () => Promise<void>
}

export const useLayoutStore = create<LayoutState>()(
  persist(
    (set, get) => ({
      widgets: DEFAULT_WIDGETS,
      isEditing: false,
      isLoaded: false,

      setWidgets: (widgets) => set({ widgets }),

      addWidget: (type) => {
        const definition = WIDGET_REGISTRY[type]
        const id = `widget-${Date.now()}`

        // Find the highest Y position to place new widget
        const widgets = get().widgets
        const maxY = widgets.reduce((max, w) => Math.max(max, w.layout.y + w.layout.h), 0)

        const widget: WidgetConfig = {
          id,
          type,
          title: definition.name,
          layout: {
            x: 0,
            y: maxY,
            ...definition.defaultSize,
            minW: definition.minSize?.w,
            minH: definition.minSize?.h,
          },
        }

        set((state) => ({ widgets: [...state.widgets, widget] }))
        return id
      },

      removeWidget: (id) =>
        set((state) => ({
          widgets: state.widgets.filter((w) => w.id !== id),
        })),

      updateLayout: (id, layout) =>
        set((state) => ({
          widgets: state.widgets.map((w) =>
            w.id === id ? { ...w, layout } : w
          ),
        })),

      toggleEditing: () =>
        set((state) => ({ isEditing: !state.isEditing })),

      resetToDefault: () => set({ widgets: DEFAULT_WIDGETS }),

      loadLayout: async () => {
        try {
          const response = await fetch(`${API_BASE}/api/user/layout`)
          if (!response.ok) {
            throw new Error('Failed to load layout')
          }

          const data = await response.json()
          if (data.success && data.data?.widgets) {
            // Convert widgets array to proper format
            const widgets = Array.isArray(data.data.widgets)
              ? data.data.widgets
              : []

            if (widgets.length > 0) {
              set({ widgets, isLoaded: true })
            } else {
              // Use default widgets if server returns empty
              set({ widgets: DEFAULT_WIDGETS, isLoaded: true })
            }
          }
        } catch (error) {
          console.error('Failed to load layout from server:', error)
          // Keep using localStorage cached layout
          set({ isLoaded: true })
        }
      },

      saveLayout: async () => {
        const { widgets } = get()
        try {
          await fetch(`${API_BASE}/api/user/layout`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ widgets }),
          })
        } catch (error) {
          console.error('Failed to save layout to server:', error)
        }
      },
    }),
    {
      name: 'cowork-layout',
    }
  )
)