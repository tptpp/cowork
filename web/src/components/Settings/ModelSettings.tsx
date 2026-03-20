import { Key, Globe, Cpu, Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { useSettingsStore } from '@/stores/settingsStore'

// 常用模型建议
const SUGGESTED_MODELS = [
  { name: 'gpt-4', label: 'GPT-4' },
  { name: 'gpt-4o', label: 'GPT-4o' },
  { name: 'gpt-3.5-turbo', label: 'GPT-3.5 Turbo' },
  { name: 'claude-3-opus', label: 'Claude 3 Opus' },
  { name: 'claude-3-sonnet', label: 'Claude 3 Sonnet' },
  { name: 'glm-4', label: 'GLM-4' },
  { name: 'glm-4-flash', label: 'GLM-4 Flash' },
]

// 常用 Base URL 建议
const SUGGESTED_BASE_URLS = [
  { url: 'https://api.openai.com/v1', label: 'OpenAI' },
  { url: 'https://api.anthropic.com/v1', label: 'Anthropic' },
  { url: 'https://open.bigmodel.cn/api/paas/v4', label: 'GLM (智谱)' },
]

export function ModelSettings() {
  const { modelConfig, updateModelConfig } = useSettingsStore()
  const [showApiKey, setShowApiKey] = useState(false)

  return (
    <div className="space-y-6">
      {/* Model Configuration */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Cpu className="w-4 h-4" />
            Model Configuration
          </CardTitle>
          <CardDescription>
            Configure your AI model endpoint and settings
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Base URL */}
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Globe className="w-3 h-3" />
              Base URL
            </label>
            <Input
              type="text"
              value={modelConfig.baseUrl}
              onChange={(e) => updateModelConfig({ baseUrl: e.target.value })}
              placeholder="https://api.openai.com/v1"
            />
            {/* Quick select suggestions */}
            <div className="flex flex-wrap gap-1.5 mt-1">
              {SUGGESTED_BASE_URLS.map((item) => (
                <button
                  key={item.url}
                  type="button"
                  onClick={() => updateModelConfig({ baseUrl: item.url })}
                  className="px-2 py-0.5 text-xs rounded-md bg-muted hover:bg-muted/80 transition-colors"
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>

          {/* Model Name */}
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Cpu className="w-3 h-3" />
              Model Name
            </label>
            <Input
              type="text"
              value={modelConfig.modelName}
              onChange={(e) => updateModelConfig({ modelName: e.target.value })}
              placeholder="gpt-4, claude-3-opus, glm-4..."
            />
            {/* Quick select suggestions */}
            <div className="flex flex-wrap gap-1.5 mt-1">
              {SUGGESTED_MODELS.map((item) => (
                <button
                  key={item.name}
                  type="button"
                  onClick={() => updateModelConfig({ modelName: item.name })}
                  className="px-2 py-0.5 text-xs rounded-md bg-muted hover:bg-muted/80 transition-colors"
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>

          {/* API Key (Optional) */}
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Key className="w-3 h-3" />
              API Key
              <span className="text-xs text-muted-foreground">(optional)</span>
            </label>
            <div className="flex gap-2">
              <Input
                type={showApiKey ? 'text' : 'password'}
                value={modelConfig.apiKey || ''}
                onChange={(e) => updateModelConfig({ apiKey: e.target.value })}
                placeholder="sk-..."
                className="flex-1"
              />
              <Button
                variant="outline"
                size="icon"
                onClick={() => setShowApiKey(!showApiKey)}
                type="button"
              >
                {showApiKey ? (
                  <EyeOff className="w-4 h-4" />
                ) : (
                  <Eye className="w-4 h-4" />
                )}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              API Key can also be configured on the server via environment variables
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Current Config Preview */}
      {(modelConfig.baseUrl || modelConfig.modelName) && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm text-muted-foreground">Current Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="bg-muted/50 rounded-lg p-3 font-mono text-sm space-y-1">
              <div className="flex gap-2">
                <span className="text-muted-foreground">Base URL:</span>
                <span>{modelConfig.baseUrl || <span className="text-muted-foreground italic">not set</span>}</span>
              </div>
              <div className="flex gap-2">
                <span className="text-muted-foreground">Model:</span>
                <span>{modelConfig.modelName || <span className="text-muted-foreground italic">not set</span>}</span>
              </div>
              <div className="flex gap-2">
                <span className="text-muted-foreground">API Key:</span>
                <span>{modelConfig.apiKey ? '••••••••' : <span className="text-muted-foreground italic">not set</span>}</span>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}