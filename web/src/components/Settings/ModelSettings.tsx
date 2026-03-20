import { Key, Globe, Cpu, Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { useSettingsStore, type DefaultModel } from '@/stores/settingsStore'

const MODEL_OPTIONS = [
  { value: 'default', label: 'Default (Auto)' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'glm', label: 'GLM (智谱)' },
]

export function ModelSettings() {
  const { modelConfig, updateModelConfig } = useSettingsStore()
  const [showKeys, setShowKeys] = useState({
    openai: false,
    anthropic: false,
    glm: false,
  })

  const toggleKeyVisibility = (provider: keyof typeof showKeys) => {
    setShowKeys((prev) => ({ ...prev, [provider]: !prev[provider] }))
  }

  return (
    <div className="space-y-6">
      {/* Default Model Selection */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Cpu className="w-4 h-4" />
            Default Model
          </CardTitle>
          <CardDescription>
            Choose which AI model to use by default for new conversations
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Select
            value={modelConfig.defaultModel}
            onChange={(e) => updateModelConfig({ defaultModel: e.target.value as DefaultModel })}
            options={MODEL_OPTIONS}
            className="w-full max-w-xs"
          />
        </CardContent>
      </Card>

      {/* OpenAI Configuration */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <span className="w-4 h-4 flex items-center justify-center text-sm font-bold">O</span>
            OpenAI Configuration
          </CardTitle>
          <CardDescription>
            Configure your OpenAI API credentials
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Key className="w-3 h-3" />
              API Key
            </label>
            <div className="flex gap-2">
              <Input
                type={showKeys.openai ? 'text' : 'password'}
                value={modelConfig.openaiApiKey}
                onChange={(e) => updateModelConfig({ openaiApiKey: e.target.value })}
                placeholder="sk-..."
                className="flex-1"
              />
              <Button
                variant="outline"
                size="icon"
                onClick={() => toggleKeyVisibility('openai')}
                type="button"
              >
                {showKeys.openai ? (
                  <EyeOff className="w-4 h-4" />
                ) : (
                  <Eye className="w-4 h-4" />
                )}
              </Button>
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Globe className="w-3 h-3" />
              Base URL (Optional)
            </label>
            <Input
              type="text"
              value={modelConfig.openaiBaseUrl}
              onChange={(e) => updateModelConfig({ openaiBaseUrl: e.target.value })}
              placeholder="https://api.openai.com/v1"
            />
          </div>
        </CardContent>
      </Card>

      {/* Anthropic Configuration */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <span className="w-4 h-4 flex items-center justify-center text-sm font-bold">A</span>
            Anthropic Configuration
          </CardTitle>
          <CardDescription>
            Configure your Anthropic API credentials
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Key className="w-3 h-3" />
              API Key
            </label>
            <div className="flex gap-2">
              <Input
                type={showKeys.anthropic ? 'text' : 'password'}
                value={modelConfig.anthropicApiKey}
                onChange={(e) => updateModelConfig({ anthropicApiKey: e.target.value })}
                placeholder="sk-ant-..."
                className="flex-1"
              />
              <Button
                variant="outline"
                size="icon"
                onClick={() => toggleKeyVisibility('anthropic')}
                type="button"
              >
                {showKeys.anthropic ? (
                  <EyeOff className="w-4 h-4" />
                ) : (
                  <Eye className="w-4 h-4" />
                )}
              </Button>
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Globe className="w-3 h-3" />
              Base URL (Optional)
            </label>
            <Input
              type="text"
              value={modelConfig.anthropicBaseUrl}
              onChange={(e) => updateModelConfig({ anthropicBaseUrl: e.target.value })}
              placeholder="https://api.anthropic.com"
            />
          </div>
        </CardContent>
      </Card>

      {/* GLM Configuration */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <span className="w-4 h-4 flex items-center justify-center text-sm font-bold">G</span>
            GLM (智谱) Configuration
          </CardTitle>
          <CardDescription>
            Configure your GLM API credentials
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Key className="w-3 h-3" />
              API Key
            </label>
            <div className="flex gap-2">
              <Input
                type={showKeys.glm ? 'text' : 'password'}
                value={modelConfig.glmApiKey}
                onChange={(e) => updateModelConfig({ glmApiKey: e.target.value })}
                placeholder="Your GLM API key"
                className="flex-1"
              />
              <Button
                variant="outline"
                size="icon"
                onClick={() => toggleKeyVisibility('glm')}
                type="button"
              >
                {showKeys.glm ? (
                  <EyeOff className="w-4 h-4" />
                ) : (
                  <Eye className="w-4 h-4" />
                )}
              </Button>
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium flex items-center gap-2">
              <Globe className="w-3 h-3" />
              Base URL (Optional)
            </label>
            <Input
              type="text"
              value={modelConfig.glmBaseUrl}
              onChange={(e) => updateModelConfig({ glmBaseUrl: e.target.value })}
              placeholder="https://open.bigmodel.cn/api/paas/v4"
            />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}