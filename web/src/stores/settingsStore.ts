import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemeMode = 'light' | 'dark' | 'system'
export type DefaultModel = 'default' | 'openai' | 'anthropic' | 'glm'

export interface ModelConfig {
  openaiApiKey: string
  openaiBaseUrl: string
  anthropicApiKey: string
  anthropicBaseUrl: string
  glmApiKey: string
  glmBaseUrl: string
  defaultModel: DefaultModel
}

interface SettingsState {
  // Theme
  theme: ThemeMode

  // Model config
  modelConfig: ModelConfig

  // Actions
  setTheme: (theme: ThemeMode) => void
  updateModelConfig: (config: Partial<ModelConfig>) => void
  resetSettings: () => void
}

const defaultModelConfig: ModelConfig = {
  openaiApiKey: '',
  openaiBaseUrl: '',
  anthropicApiKey: '',
  anthropicBaseUrl: '',
  glmApiKey: '',
  glmBaseUrl: '',
  defaultModel: 'default',
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