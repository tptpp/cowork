import { useState, useEffect, useRef } from 'react'
import { ResponsiveGridLayout } from 'react-grid-layout'
import type { Layout, ResponsiveLayouts } from 'react-grid-layout'
import { Plus, LayoutDashboard, List, Settings, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { TaskList } from '@/components/tasks/TaskList'
import { TaskDetail } from '@/components/tasks/TaskDetail'
import { TaskForm } from '@/components/tasks/TaskForm'
import { WidgetWrapper } from '@/components/dashboard/WidgetWrapper'
import { WidgetStore } from '@/components/dashboard/WidgetStore'
import {
  SystemStatsWidget,
  TaskQueueWidget,
  WorkerStatusWidget,
} from '@/components/widgets'
import { AgentChatWidget } from '@/components/widgets/AgentChatWidget'
import { useLayoutStore } from '@/stores/layoutStore'
import { useTaskStore } from '@/stores/taskStore'
import { useSystemStore } from '@/stores/systemStore'
import { useWorkerStore } from '@/stores/workerStore'
import { useWebSocket } from '@/hooks/useWebSocket'
import type { Task, WSMessage, Worker, WidgetConfig } from '@/types'

import 'react-grid-layout/css/styles.css'
import 'react-resizable/css/styles.css'

type ViewMode = 'dashboard' | 'tasks'

// Widget renderer component
function WidgetRenderer({ widget, onSelectTask }: { widget: WidgetConfig; onSelectTask: (task: Task) => void }) {
  switch (widget.type) {
    case 'system-stats':
      return <SystemStatsWidget />
    case 'task-queue':
      return <TaskQueueWidget onSelectTask={onSelectTask} />
    case 'worker-status':
      return <WorkerStatusWidget />
    case 'agent-chat':
      return <AgentChatWidget />
    default:
      return (
        <div className="flex items-center justify-center h-full text-muted-foreground">
          Unknown widget type: {widget.type}
        </div>
      )
  }
}

export function Dashboard() {
  const [viewMode, setViewMode] = useState<ViewMode>('dashboard')
  const [showTaskForm, setShowTaskForm] = useState(false)
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null)
  const [showTaskDetail, setShowTaskDetail] = useState(false)
  const [showWidgetStore, setShowWidgetStore] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const [containerWidth, setContainerWidth] = useState(1200)

  const { widgets, isEditing, updateLayout, toggleEditing, resetToDefault, loadLayout, saveLayout } = useLayoutStore()
  const { fetchStats } = useSystemStore()
  const { fetchTasks, updateTask } = useTaskStore()
  const { fetchWorkers, updateWorker } = useWorkerStore()

  // WebSocket for real-time updates
  useWebSocket({
    onMessage: (message: WSMessage) => {
      switch (message.type) {
        case 'task_update':
          updateTask(message.payload as Task)
          break
        case 'worker_update':
          updateWorker(message.payload as Worker)
          break
      }
    },
  })

  // Load layout from server on mount
  useEffect(() => {
    loadLayout()
  }, [loadLayout])

  useEffect(() => {
    fetchStats()
    fetchTasks()
    fetchWorkers()
  }, [fetchStats, fetchTasks, fetchWorkers])

  // Auto-refresh stats every 10 seconds
  useEffect(() => {
    const interval = setInterval(fetchStats, 10000)
    return () => clearInterval(interval)
  }, [fetchStats])

  // Update container width on resize
  useEffect(() => {
    const updateWidth = () => {
      if (containerRef.current) {
        setContainerWidth(containerRef.current.offsetWidth)
      }
    }
    updateWidth()
    window.addEventListener('resize', updateWidth)
    return () => window.removeEventListener('resize', updateWidth)
  }, [])

  const handleSelectTask = (task: Task) => {
    setSelectedTaskId(task.id)
    setShowTaskDetail(true)
  }

  const handleTaskCreated = (task: Task) => {
    setShowTaskForm(false)
    setSelectedTaskId(task.id)
    setShowTaskDetail(true)
  }

  // Convert widgets to grid layout items
  const layout: Layout = widgets.map((w) => ({
    i: w.id,
    x: w.layout.x,
    y: w.layout.y,
    w: w.layout.w,
    h: w.layout.h,
    minW: w.layout.minW,
    minH: w.layout.minH,
  }))

  // Responsive layouts
  const layouts: ResponsiveLayouts = {
    lg: layout,
  }

  const handleLayoutChange = (currentLayout: Layout, allLayouts: ResponsiveLayouts) => {
    const newLayout = allLayouts.lg || currentLayout
    newLayout.forEach((l) => {
      updateLayout(l.i, { x: l.x, y: l.y, w: l.w, h: l.h })
    })
    // Save to backend
    saveLayout()
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container flex h-14 items-center justify-between px-4">
          <div className="flex items-center gap-4">
            <h1 className="text-xl font-bold">Cowork</h1>
            <Badge variant="secondary" className="text-xs">
              Distributed Task Processing
            </Badge>
          </div>

          <div className="flex items-center gap-2">
            {/* View Mode Toggle */}
            <div className="flex items-center border rounded-lg p-1">
              <Button
                variant={viewMode === 'dashboard' ? 'default' : 'ghost'}
                size="sm"
                onClick={() => setViewMode('dashboard')}
                className="gap-1"
              >
                <LayoutDashboard className="w-4 h-4" />
                Dashboard
              </Button>
              <Button
                variant={viewMode === 'tasks' ? 'default' : 'ghost'}
                size="sm"
                onClick={() => setViewMode('tasks')}
                className="gap-1"
              >
                <List className="w-4 h-4" />
                Tasks
              </Button>
            </div>

            {/* Edit Mode Toggle (only in dashboard view) */}
            {viewMode === 'dashboard' && (
              <>
                <Button
                  variant={isEditing ? 'default' : 'outline'}
                  size="sm"
                  onClick={toggleEditing}
                  className="gap-1"
                >
                  <Settings className="w-4 h-4" />
                  {isEditing ? 'Done' : 'Edit'}
                </Button>

                {isEditing && (
                  <>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setShowWidgetStore(true)}
                      className="gap-1"
                    >
                      <Plus className="w-4 h-4" />
                      Add Widget
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={resetToDefault}
                      className="gap-1"
                    >
                      <RotateCcw className="w-4 h-4" />
                      Reset
                    </Button>
                  </>
                )}
              </>
            )}

            {/* Quick Create Task Button */}
            <Button onClick={() => setShowTaskForm(true)} className="gap-1">
              <Plus className="w-4 h-4" />
              New Task
            </Button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container px-4 py-6">
        {viewMode === 'dashboard' ? (
          <div ref={containerRef} className="space-y-4">
            {/* Grid Layout */}
            <ResponsiveGridLayout
              className="layout"
              layouts={layouts}
              width={containerWidth - 32}
              breakpoints={{ lg: 1200, md: 996, sm: 768, xs: 480, xxs: 0 }}
              cols={{ lg: 12, md: 10, sm: 6, xs: 4, xxs: 2 }}
              rowHeight={80}
              margin={[16, 16]}
              dragConfig={{ enabled: isEditing }}
              resizeConfig={{ enabled: isEditing, handles: ['se'] }}
              onLayoutChange={handleLayoutChange}
            >
              {widgets.map((widget) => (
                <div key={widget.id} className="overflow-hidden">
                  <WidgetWrapper widget={widget}>
                    <WidgetRenderer widget={widget} onSelectTask={handleSelectTask} />
                  </WidgetWrapper>
                </div>
              ))}
            </ResponsiveGridLayout>

            {widgets.length === 0 && (
              <div className="flex items-center justify-center h-64 text-muted-foreground">
                <div className="text-center">
                  <p className="mb-2">No widgets configured</p>
                  <Button onClick={() => setShowWidgetStore(true)}>
                    <Plus className="w-4 h-4 mr-2" />
                    Add Widget
                  </Button>
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-6">
            <TaskList onSelectTask={handleSelectTask} />
          </div>
        )}
      </main>

      {/* Widget Store Dialog */}
      <WidgetStore open={showWidgetStore} onClose={() => setShowWidgetStore(false)} />

      {/* Task Form Dialog */}
      <Dialog open={showTaskForm} onOpenChange={setShowTaskForm}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Create New Task</DialogTitle>
          </DialogHeader>
          <TaskForm
            onSuccess={handleTaskCreated}
            onCancel={() => setShowTaskForm(false)}
            showCard={false}
          />
        </DialogContent>
      </Dialog>

      {/* Task Detail Dialog */}
      <Dialog open={showTaskDetail} onOpenChange={setShowTaskDetail}>
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Task Details</DialogTitle>
          </DialogHeader>
          {selectedTaskId && <TaskDetail taskId={selectedTaskId} />}
        </DialogContent>
      </Dialog>
    </div>
  )
}