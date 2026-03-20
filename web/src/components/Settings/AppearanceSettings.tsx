import { Sun, Moon, Monitor } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useSettingsStore, type ThemeMode } from '@/stores/settingsStore'
import { cn } from '@/lib/utils'

const themeOptions: { value: ThemeMode; label: string; icon: typeof Sun; description: string }[] = [
  { value: 'light', label: 'Light', icon: Sun, description: 'Light theme' },
  { value: 'dark', label: 'Dark', icon: Moon, description: 'Dark theme' },
  { value: 'system', label: 'System', icon: Monitor, description: 'Follow system preference' },
]

export function AppearanceSettings() {
  const { theme, setTheme } = useSettingsStore()

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Theme</CardTitle>
        <CardDescription>
          Choose your preferred color theme for the interface
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex gap-3">
          {themeOptions.map((option) => {
            const Icon = option.icon
            const isActive = theme === option.value
            return (
              <button
                key={option.value}
                onClick={() => setTheme(option.value)}
                className={cn(
                  'flex flex-col items-center gap-2 p-4 rounded-lg border-2 transition-all',
                  'hover:border-primary/50 hover:bg-accent/50',
                  isActive
                    ? 'border-primary bg-primary/10'
                    : 'border-border bg-background'
                )}
              >
                <div
                  className={cn(
                    'w-10 h-10 rounded-full flex items-center justify-center',
                    isActive ? 'bg-primary text-primary-foreground' : 'bg-muted'
                  )}
                >
                  <Icon className="w-5 h-5" />
                </div>
                <span className="text-sm font-medium">{option.label}</span>
              </button>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}