import { ArrowLeft, Bot, Palette, RotateCcw } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { ModelSettings } from './ModelSettings'
import { AppearanceSettings } from './AppearanceSettings'
import { useSettingsStore } from '@/stores/settingsStore'
import { cn } from '@/lib/utils'

type SettingsTab = 'model' | 'appearance'

interface SettingsPageProps {
  onBack: () => void
}

const tabs: { id: SettingsTab; label: string; icon: typeof Bot }[] = [
  { id: 'model', label: 'Model', icon: Bot },
  { id: 'appearance', label: 'Appearance', icon: Palette },
]

export function SettingsPage({ onBack }: SettingsPageProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('model')
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
        <div className="container flex h-14 items-center gap-4 px-4">
          <Button variant="ghost" size="sm" onClick={onBack} className="gap-1">
            <ArrowLeft className="w-4 h-4" />
            Back
          </Button>
          <h1 className="text-lg font-semibold">Settings</h1>
          <div className="flex-1" />
          <Button variant="outline" size="sm" onClick={handleReset} className="gap-1">
            <RotateCcw className="w-4 h-4" />
            Reset
          </Button>
        </div>
      </header>

      {/* Content */}
      <div className="container px-4 py-6">
        <div className="flex gap-6">
          {/* Sidebar */}
          <nav className="w-48 flex-shrink-0">
            <div className="space-y-1">
              {tabs.map((tab) => {
                const Icon = tab.icon
                return (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id)}
                    className={cn(
                      'w-full flex items-center gap-3 px-3 py-2 text-sm rounded-lg transition-colors',
                      activeTab === tab.id
                        ? 'bg-primary text-primary-foreground'
                        : 'hover:bg-muted text-foreground'
                    )}
                  >
                    <Icon className="w-4 h-4" />
                    {tab.label}
                  </button>
                )
              })}
            </div>
          </nav>

          {/* Main Content */}
          <main className="flex-1 max-w-2xl">
            {activeTab === 'model' && <ModelSettings />}
            {activeTab === 'appearance' && <AppearanceSettings />}
          </main>
        </div>
      </div>
    </div>
  )
}