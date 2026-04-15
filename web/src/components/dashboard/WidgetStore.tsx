import { useState } from 'react'
import { Search, BarChart3, ListTodo, Users, MessageSquare, Sparkles, Shield, Server, GitBranch } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useLayoutStore, WIDGET_REGISTRY } from '@/stores/layoutStore'
import type { WidgetType } from '@/types'
import { cn } from '@/lib/utils'

// Icon mapping
const ICON_MAP: Record<string, React.ElementType> = {
  BarChart3,
  ListTodo,
  Users,
  MessageSquare,
  Sparkles,
  Shield,
  Server,
  GitBranch,
}

interface WidgetStoreProps {
  open: boolean
  onClose: () => void
}

export function WidgetStore({ open, onClose }: WidgetStoreProps) {
  const [search, setSearch] = useState('')
  const { addWidget } = useLayoutStore()

  const widgets = Object.values(WIDGET_REGISTRY).filter(
    (w) =>
      w.name.toLowerCase().includes(search.toLowerCase()) ||
      w.description.toLowerCase().includes(search.toLowerCase())
  )

  const handleAdd = (type: WidgetType) => {
    addWidget(type)
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-lg">Add Widget</DialogTitle>
        </DialogHeader>

        <div className="relative mb-4">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search widgets..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>

        <div className="grid gap-2 max-h-80 overflow-auto">
          {widgets.map((widget) => {
            const Icon = ICON_MAP[widget.icon] || BarChart3
            return (
              <button
                key={widget.type}
                onClick={() => handleAdd(widget.type)}
                className={cn(
                  'w-full p-3 rounded-xl border text-left transition-all',
                  'hover:border-primary/50 hover:bg-accent/50 hover:shadow-sm',
                  'active:scale-[0.98]'
                )}
              >
                <div className="flex items-center gap-3">
                  <div className="p-2.5 bg-gradient-to-br from-primary/10 to-primary/5 rounded-lg">
                    <Icon className="h-5 w-5 text-primary" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-sm">{widget.name}</div>
                    <p className="text-xs text-muted-foreground truncate">
                      {widget.description}
                    </p>
                  </div>
                </div>
              </button>
            )
          })}
        </div>
      </DialogContent>
    </Dialog>
  )
}