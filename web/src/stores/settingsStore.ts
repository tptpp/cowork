import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemeMode = 'light' | 'dark' | 'system'

export interface ModelConfig {
  baseUrl: string    // API 端点地址，如 https://api.openai.com/v1
  modelName: string  // 模型名称，如 gpt-4、claude-3-opus、glm-4
  apiKey?: string    // API Key（可选）
}

interface SettingsState {
  // Theme
  theme: ThemeMode

  // Model config (simplified unified config)
  modelConfig: ModelConfig

  // Actions
  setTheme: (theme: ThemeMode) => void
  updateModelConfig: (config: Partial<ModelConfig>) => void
  resetSettings: () => void
}

const defaultModelConfig: ModelConfig = {
  baseUrl: '',
  modelName: '',
  apiKey: '',
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      // Initial state
      theme: 'system',
      modelConfig: defaultModelConfig,

      // Actions
      setTheme: (theme) => {
        set({ theme })
        applyTheme(theme)
      },

      updateModelConfig: (config) =>
        set((state) => ({
          modelConfig: { ...state.modelConfig, ...config },
        })),

      resetSettings: () => {
        set({
          theme: 'system',
          modelConfig: defaultModelConfig,
        })
        applyTheme('system')
      },
    }),
    {
      name: 'cowork-settings',
      onRehydrateStorage: () => (state) => {
        if (state) {
          applyTheme(state.theme)
        }
      },
    }
  )
)

// Apply theme to document
function applyTheme(theme: ThemeMode) {
  const root = document.documentElement
  root.classList.remove('light', 'dark')

  if (theme === 'system') {
    const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    root.classList.add(systemDark ? 'dark' : 'light')
  } else {
    root.classList.add(theme)
  }
}

// Initialize theme on load
if (typeof window !== 'undefined') {
  const stored = localStorage.getItem('cowork-settings')
  if (stored) {
    try {
      const parsed = JSON.parse(stored)
      if (parsed.state?.theme) {
        applyTheme(parsed.state.theme)
      }
    } catch {
      applyTheme('system')
    }
  }
}