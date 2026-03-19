import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { WidgetConfig, WidgetType, GridLayoutItem, WidgetDefinition } from '@/types'

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

  // Actions
  setWidgets: (widgets: WidgetConfig[]) => void
  addWidget: (type: WidgetType) => string
  removeWidget: (id: string) => void
  updateLayout: (id: string, layout: GridLayoutItem) => void
  toggleEditing: () => void
  resetToDefault: () => void
}

export const useLayoutStore = create<LayoutState>()(
  persist(
    (set, get) => ({
      widgets: DEFAULT_WIDGETS,
      isEditing: false,

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
    }),
    {
      name: 'cowork-layout',
    }
  )
)