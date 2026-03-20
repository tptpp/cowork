import { X } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useLayoutStore, WIDGET_REGISTRY } from '@/stores/layoutStore'
import type { WidgetConfig } from '@/types'
import { cn } from '@/lib/utils'

interface WidgetWrapperProps {
  widget: WidgetConfig
  children: React.ReactNode
}

export function WidgetWrapper({ widget, children }: WidgetWrapperProps) {
  const { isEditing, removeWidget } = useLayoutStore()
  const definition = WIDGET_REGISTRY[widget.type]

  return (
    <Card
      className={cn(
        'h-full flex flex-col overflow-hidden transition-all duration-200',
        isEditing && 'ring-2 ring-dashed ring-primary/30 hover:ring-primary/50'
      )}
    >
      <CardHeader className="flex-row items-center justify-between py-3 px-4 space-y-0 border-b bg-muted/30">
        <CardTitle className="text-sm font-semibold">
          {widget.title}
        </CardTitle>

        {isEditing && (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
              onClick={() => removeWidget(widget.id)}
              title="Remove widget"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        )}
      </CardHeader>

      <CardContent className="flex-1 overflow-auto p-0">
        {children}
      </CardContent>
    </Card>
  )
}