import { Settings, X } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useLayoutStore, WIDGET_REGISTRY } from '@/stores/layoutStore'
import type { WidgetConfig } from '@/types'

interface WidgetWrapperProps {
  widget: WidgetConfig
  children: React.ReactNode
}

export function WidgetWrapper({ widget, children }: WidgetWrapperProps) {
  const { isEditing, removeWidget } = useLayoutStore()
  const definition = WIDGET_REGISTRY[widget.type]

  return (
    <Card className="h-full flex flex-col overflow-hidden">
      <CardHeader className="flex-row items-center justify-between py-2 px-4 space-y-0 border-b">
        <CardTitle className="text-sm font-medium">
          {widget.title}
        </CardTitle>

        {isEditing && (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 text-muted-foreground hover:text-destructive"
              onClick={() => removeWidget(widget.id)}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        )}
      </CardHeader>

      <CardContent className="flex-1 overflow-auto p-4">
        {children}
      </CardContent>
    </Card>
  )
}