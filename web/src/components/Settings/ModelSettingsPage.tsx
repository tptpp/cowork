import { RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ModelSettings } from '@/components/Settings/ModelSettings'
import { useSettingsStore } from '@/stores/settingsStore'

export function ModelSettingsPage() {
  const { resetSettings } = useSettingsStore()

  const handleReset = () => {
    if (confirm('Are you sure you want to reset all settings to default?')) {
      resetSettings()
    }
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container flex h-14 items-center justify-between px-6">
          <h1 className="text-lg font-semibold">Model Settings</h1>
          <Button variant="outline" size="sm" onClick={handleReset} className="gap-1">
            <RotateCcw className="w-4 h-4" />
            Reset
          </Button>
        </div>
      </header>

      {/* Content */}
      <div className="container px-6 py-6">
        <div className="max-w-2xl">
          <ModelSettings />
        </div>
      </div>
    </div>
  )
}