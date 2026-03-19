import { useState } from 'react'
import { Search, BarChart3, ListTodo, Users } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useLayoutStore, WIDGET_REGISTRY } from '@/stores/layoutStore'
import type { WidgetType } from '@/types'

// Icon mapping
const ICON_MAP: Record<string, React.ElementType> = {
  BarChart3,
  ListTodo,
  Users,
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
          <DialogTitle>Add Widget</DialogTitle>
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

        <div className="grid gap-3 max-h-80 overflow-auto">
          {widgets.map((widget) => {
            const Icon = ICON_MAP[widget.icon] || BarChart3
            return (
              <div
                key={widget.type}
                className="p-4 border rounded-lg hover:bg-accent cursor-pointer transition-colors"
                onClick={() => handleAdd(widget.type)}
              >
                <div className="flex items-center gap-3">
                  <div className="p-2 bg-muted rounded-lg">
                    <Icon className="h-5 w-5" />
                  </div>
                  <div>
                    <div className="font-medium">{widget.name}</div>
                    <p className="text-sm text-muted-foreground">
                      {widget.description}
                    </p>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </DialogContent>
    </Dialog>
  )
}