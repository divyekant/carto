import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

// Matches the Go configResponse JSON shape exactly
interface Config {
  llm_provider: string
  llm_api_key: string
  llm_base_url: string
  anthropic_key: string
  fast_model: string
  deep_model: string
  memories_url: string
  memories_key: string
  max_concurrent: number
  github_token: string
  jira_token: string
  jira_email: string
  jira_base_url: string
  linear_token: string
  notion_token: string
  slack_token: string
}

interface ModelOption {
  value: string
  label: string
  description: string
}

interface ProviderConfig {
  fast: string
  deep: string
  baseUrl: string
  keyPlaceholder: string
  fastModels: ModelOption[]
  deepModels: ModelOption[]
}

const PROVIDER_DEFAULTS: Record<string, ProviderConfig> = {
  anthropic: {
    fast: 'claude-haiku-4-5-20251001',
    deep: 'claude-opus-4-6',
    baseUrl: '',
    keyPlaceholder: 'sk-ant-api03-...',
    fastModels: [
      { value: 'claude-haiku-4-5-20251001', label: 'Claude Haiku 4.5', description: 'Fastest, $1/$5 per MTok' },
      { value: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6', description: 'Fast, $3/$15 per MTok' },
      { value: 'claude-sonnet-4-5-20250929', label: 'Claude Sonnet 4.5', description: 'Previous gen, $3/$15 per MTok' },
      { value: 'claude-3-haiku-20240307', label: 'Claude Haiku 3', description: 'Legacy, $0.25/$1.25 per MTok' },
    ],
    deepModels: [
      { value: 'claude-opus-4-6', label: 'Claude Opus 4.6', description: 'Most intelligent, $5/$25 per MTok' },
      { value: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6', description: 'Near-Opus quality, $3/$15 per MTok' },
      { value: 'claude-opus-4-5-20251101', label: 'Claude Opus 4.5', description: 'Previous gen, $5/$25 per MTok' },
      { value: 'claude-sonnet-4-5-20250929', label: 'Claude Sonnet 4.5', description: 'Previous gen, $3/$15 per MTok' },
    ],
  },
  openai: {
    fast: 'gpt-4.1-mini',
    deep: 'gpt-4.1',
    baseUrl: 'https://api.openai.com/v1',
    keyPlaceholder: 'sk-...',
    fastModels: [
      { value: 'gpt-4.1-mini', label: 'GPT-4.1 Mini', description: 'Fast and affordable' },
      { value: 'gpt-4.1-nano', label: 'GPT-4.1 Nano', description: 'Smallest, cheapest' },
      { value: 'gpt-4o-mini', label: 'GPT-4o Mini', description: 'Previous gen' },
      { value: 'o3-mini', label: 'o3-mini', description: 'Small reasoning model' },
    ],
    deepModels: [
      { value: 'gpt-4.1', label: 'GPT-4.1', description: 'Best coding & instruction following' },
      { value: 'gpt-4o', label: 'GPT-4o', description: 'Previous gen flagship' },
      { value: 'o3', label: 'o3', description: 'Advanced reasoning' },
    ],
  },
  ollama: {
    fast: 'llama3.2',
    deep: 'llama3.2',
    baseUrl: 'http://localhost:11434',
    keyPlaceholder: '(not required for Ollama)',
    fastModels: [
      { value: 'llama3.2', label: 'Llama 3.2', description: '1B/3B, lightweight' },
      { value: 'llama3.3', label: 'Llama 3.3', description: '70B quality' },
      { value: 'qwen3', label: 'Qwen 3', description: 'Dense & MoE variants' },
      { value: 'gemma2', label: 'Gemma 2', description: '2B/9B/27B by Google' },
      { value: 'phi3', label: 'Phi-3', description: '3B/14B by Microsoft' },
    ],
    deepModels: [
      { value: 'llama3.2', label: 'Llama 3.2', description: '1B/3B, lightweight' },
      { value: 'llama3.3', label: 'Llama 3.3', description: '70B quality' },
      { value: 'qwen3', label: 'Qwen 3', description: 'Dense & MoE variants' },
      { value: 'deepseek-r1', label: 'DeepSeek R1', description: 'Strong reasoning' },
      { value: 'mistral', label: 'Mistral 7B', description: 'Versatile 7B model' },
    ],
  },
}

const CUSTOM_MODEL_VALUE = '__custom__'

interface ValidationErrors {
  provider?: string
  apiKey?: string
  baseUrl?: string
  fastModel?: string
  deepModel?: string
  memoriesUrl?: string
}

function validate(config: Config): ValidationErrors {
  const errors: ValidationErrors = {}
  const provider = config.llm_provider

  if (!provider) errors.provider = 'Provider is required'

  if (provider === 'anthropic') {
    if (!config.anthropic_key) errors.apiKey = 'Anthropic API key is required'
  } else if (provider === 'openai') {
    if (!config.llm_api_key) errors.apiKey = 'API key is required for OpenAI'
  }

  if (provider && provider !== 'anthropic' && !config.llm_base_url) {
    errors.baseUrl = 'Base URL is required for ' + provider
  }

  if (config.llm_base_url && !config.llm_base_url.match(/^https?:\/\//)) {
    errors.baseUrl = 'Must start with http:// or https://'
  }

  if (!config.fast_model) errors.fastModel = 'Fast model is required'
  if (!config.deep_model) errors.deepModel = 'Deep model is required'

  if (!config.memories_url) {
    errors.memoriesUrl = 'Memories URL is required'
  } else if (!config.memories_url.match(/^https?:\/\//)) {
    errors.memoriesUrl = 'Must start with http:// or https://'
  }

  return errors
}

function ModelSelect({ label, description, models, value, onChange, error }: {
  label: string
  description: string
  models: ModelOption[]
  value: string
  onChange: (value: string) => void
  error?: string
}) {
  const isCustom = value !== '' && !models.some(m => m.value === value)
  const [showCustomInput, setShowCustomInput] = useState(isCustom)
  const [customValue, setCustomValue] = useState(isCustom ? value : '')

  const prevModelsRef = useRef(models)
  useEffect(() => {
    if (prevModelsRef.current !== models) {
      prevModelsRef.current = models
      const stillCustom = value !== '' && !models.some(m => m.value === value)
      setShowCustomInput(stillCustom)
      setCustomValue(stillCustom ? value : '')
    }
  }, [models, value])

  function handleSelectChange(v: string) {
    if (v === CUSTOM_MODEL_VALUE) {
      setShowCustomInput(true)
      setCustomValue('')
      onChange('')
    } else {
      setShowCustomInput(false)
      setCustomValue('')
      onChange(v)
    }
  }

  return (
    <div className="space-y-1">
      <Label className="text-xs">{label}</Label>
      <Select
        value={showCustomInput ? CUSTOM_MODEL_VALUE : value}
        onValueChange={handleSelectChange}
      >
        <SelectTrigger className="w-full h-8 text-xs">
          <SelectValue placeholder="Select a model" />
        </SelectTrigger>
        <SelectContent>
          {models.map(m => (
            <SelectItem key={m.value} value={m.value}>
              <span>{m.label}</span>
              <span className="ml-2 text-muted-foreground text-xs">{m.description}</span>
            </SelectItem>
          ))}
          <SelectItem value={CUSTOM_MODEL_VALUE}>
            <span>Custom...</span>
          </SelectItem>
        </SelectContent>
      </Select>
      {showCustomInput && (
        <Input
          placeholder="e.g. my-custom-model"
          value={customValue}
          onChange={(e) => {
            setCustomValue(e.target.value)
            onChange(e.target.value)
          }}
          className="h-7 text-xs"
          autoFocus
        />
      )}
      {error && <p className="text-xs text-red-400">{error}</p>}
      <p className="text-xs text-muted-foreground">{description}</p>
    </div>
  )
}

export default function Settings() {
  const [config, setConfig] = useState<Config>({
    llm_provider: '',
    llm_api_key: '',
    llm_base_url: '',
    anthropic_key: '',
    fast_model: '',
    deep_model: '',
    memories_url: '',
    memories_key: '',
    max_concurrent: 10,
    github_token: '',
    jira_token: '',
    jira_email: '',
    jira_base_url: '',
    linear_token: '',
    notion_token: '',
    slack_token: '',
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [isDockerEnv, setIsDockerEnv] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState<'idle' | 'testing' | 'connected' | 'unreachable'>('idle')
  const [connectionError, setConnectionError] = useState<string | null>(null)
  const [errors, setErrors] = useState<ValidationErrors>({})
  const [touched, setTouched] = useState<Set<string>>(new Set())

  useEffect(() => {
    Promise.all([
      fetch('/api/config').then(r => r.json()),
      fetch('/api/health').then(r => r.json()),
    ]).then(([configData, healthData]) => {
      const memoriesUrl = configData.memories_url?.replace('host.docker.internal', 'localhost') || configData.memories_url
      setConfig({ ...configData, memories_url: memoriesUrl })
      setIsDockerEnv(healthData.docker === true)
    }).catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  function updateField(key: keyof Config, value: string | number) {
    setConfig(prev => ({ ...prev, [key]: value }))
    setTouched(prev => new Set(prev).add(key))
  }

  function handleProviderChange(provider: string) {
    const defaults = PROVIDER_DEFAULTS[provider]
    if (!defaults) return

    setConfig(prev => ({
      ...prev,
      llm_provider: provider,
      fast_model: defaults.fast,
      deep_model: defaults.deep,
      llm_base_url: defaults.baseUrl,
    }))
    setTouched(prev => {
      const next = new Set(prev)
      next.add('llm_provider')
      return next
    })
    setErrors({})
  }

  async function save() {
    const validationErrors = validate(config)
    setErrors(validationErrors)
    setTouched(new Set(['llm_provider', 'anthropic_key', 'llm_api_key', 'llm_base_url', 'fast_model', 'deep_model', 'memories_url']))

    if (Object.keys(validationErrors).length > 0) {
      toast.error('Please fix the errors above.')
      return
    }

    setSaving(true)
    try {
      const patch: Record<string, unknown> = {
        llm_provider: config.llm_provider,
        fast_model: config.fast_model,
        deep_model: config.deep_model,
        memories_url: config.memories_url,
      }

      if (config.anthropic_key && !config.anthropic_key.includes('****')) patch.anthropic_key = config.anthropic_key
      if (config.llm_api_key && !config.llm_api_key.includes('****')) patch.llm_api_key = config.llm_api_key
      if (config.memories_key && !config.memories_key.includes('****')) patch.memories_key = config.memories_key
      if (config.llm_base_url) patch.llm_base_url = config.llm_base_url
      if (config.github_token && !config.github_token.includes('****')) patch.github_token = config.github_token
      if (config.jira_token && !config.jira_token.includes('****')) patch.jira_token = config.jira_token
      if (config.jira_email) patch.jira_email = config.jira_email
      if (config.jira_base_url) patch.jira_base_url = config.jira_base_url
      if (config.linear_token && !config.linear_token.includes('****')) patch.linear_token = config.linear_token
      if (config.notion_token && !config.notion_token.includes('****')) patch.notion_token = config.notion_token
      if (config.slack_token && !config.slack_token.includes('****')) patch.slack_token = config.slack_token

      const res = await fetch('/api/config', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(patch),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      toast.success('Settings saved')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function testConnection() {
    if (!config.memories_url || !config.memories_url.match(/^https?:\/\//)) {
      setConnectionStatus('unreachable')
      setConnectionError('Enter a valid URL before testing')
      return
    }

    setConnectionStatus('testing')
    setConnectionError(null)
    try {
      const res = await fetch('/api/test-memories', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: config.memories_url,
          api_key: config.memories_key && !config.memories_key.includes('****') ? config.memories_key : '',
        }),
      })
      const data = await res.json()
      if (data.connected) {
        setConnectionStatus('connected')
        setConnectionError(null)
        toast.success('Memories server connected')
      } else {
        setConnectionStatus('unreachable')
        setConnectionError(data.error || 'Connection failed')
        toast.error(data.error || 'Connection failed')
      }
    } catch {
      setConnectionStatus('unreachable')
      setConnectionError('Could not reach the server')
      toast.error('Could not reach the server')
    }
  }

  if (loading) {
    return (
      <div>
        <h2 className="text-lg font-semibold mb-3">Settings</h2>
        <p className="text-muted-foreground text-sm">Loading...</p>
      </div>
    )
  }

  const provider = config.llm_provider || 'anthropic'
  const defaults = PROVIDER_DEFAULTS[provider] || PROVIDER_DEFAULTS.anthropic
  const showBaseUrl = provider !== 'anthropic'
  const showLlmApiKey = provider !== 'anthropic'

  return (
    <div>
      <h2 className="text-lg font-semibold mb-3">Settings</h2>

      {isDockerEnv && (
        <div className="rounded-md border border-blue-500/30 bg-blue-500/10 p-2 text-xs text-blue-400 mb-3">
          Running in Docker — <code className="text-xs bg-muted px-1 rounded">localhost</code> URLs are automatically routed to your host machine.
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
        {/* Left column: LLM config */}
        <div className="space-y-3">
          <h3 className="text-sm font-medium text-muted-foreground">LLM Provider</h3>

          <div className="grid grid-cols-2 gap-2">
            <div className="space-y-1">
              <Label className="text-xs">Provider</Label>
              <Select value={provider} onValueChange={handleProviderChange}>
                <SelectTrigger className="w-full h-8 text-xs">
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="anthropic">Anthropic</SelectItem>
                  <SelectItem value="openai">OpenAI-Compatible</SelectItem>
                  <SelectItem value="ollama">Ollama</SelectItem>
                </SelectContent>
              </Select>
              {errors.provider && touched.has('llm_provider') && (
                <p className="text-xs text-red-400">{errors.provider}</p>
              )}
            </div>

            {provider === 'anthropic' && (
              <div className="space-y-1">
                <Label className="text-xs">API Key</Label>
                <Input
                  type="password"
                  placeholder="sk-ant-api03-..."
                  value={config.anthropic_key || ''}
                  onChange={(e) => updateField('anthropic_key', e.target.value)}
                  className="h-8 text-xs"
                />
                {errors.apiKey && touched.has('anthropic_key') && (
                  <p className="text-xs text-red-400">{errors.apiKey}</p>
                )}
              </div>
            )}

            {showLlmApiKey && (
              <div className="space-y-1">
                <Label className="text-xs">API Key</Label>
                <Input
                  type={provider === 'ollama' ? 'text' : 'password'}
                  placeholder={defaults.keyPlaceholder}
                  value={config.llm_api_key || ''}
                  onChange={(e) => updateField('llm_api_key', e.target.value)}
                  disabled={provider === 'ollama'}
                  className="h-8 text-xs"
                />
                {errors.apiKey && touched.has('llm_api_key') && (
                  <p className="text-xs text-red-400">{errors.apiKey}</p>
                )}
              </div>
            )}
          </div>

          {showBaseUrl && (
            <div className="space-y-1">
              <Label className="text-xs">Base URL</Label>
              <Input
                placeholder={defaults.baseUrl}
                value={config.llm_base_url || ''}
                onChange={(e) => updateField('llm_base_url', e.target.value)}
                className="h-8 text-xs"
              />
              {errors.baseUrl && touched.has('llm_base_url') && (
                <p className="text-xs text-red-400">{errors.baseUrl}</p>
              )}
            </div>
          )}

          <div className="grid grid-cols-2 gap-2">
            <ModelSelect
              label="Fast Model"
              description="High-volume, low-cost"
              models={defaults.fastModels}
              value={config.fast_model || ''}
              onChange={(v) => updateField('fast_model', v)}
              error={errors.fastModel && touched.has('fast_model') ? errors.fastModel : undefined}
            />
            <ModelSelect
              label="Deep Model"
              description="Low-volume, high-cost"
              models={defaults.deepModels}
              value={config.deep_model || ''}
              onChange={(v) => updateField('deep_model', v)}
              error={errors.deepModel && touched.has('deep_model') ? errors.deepModel : undefined}
            />
          </div>
        </div>

        {/* Right column: Connections */}
        <div className="space-y-3">
          <h3 className="text-sm font-medium text-muted-foreground">Connections</h3>

          {/* Memories */}
          <div className="space-y-2">
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <Label className="text-xs">Memories URL</Label>
                <Input
                  placeholder="http://localhost:8900"
                  value={config.memories_url || ''}
                  onChange={(e) => updateField('memories_url', e.target.value)}
                  className="h-8 text-xs"
                />
                {errors.memoriesUrl && touched.has('memories_url') && (
                  <p className="text-xs text-red-400">{errors.memoriesUrl}</p>
                )}
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Memories Key</Label>
                <Input
                  type="password"
                  placeholder="(optional)"
                  value={config.memories_key || ''}
                  onChange={(e) => updateField('memories_key', e.target.value)}
                  className="h-8 text-xs"
                />
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="secondary" size="sm" onClick={testConnection} disabled={connectionStatus === 'testing'}>
                {connectionStatus === 'testing' ? 'Testing...' : 'Test'}
              </Button>
              {connectionStatus === 'connected' && <Badge variant="default" className="text-xs">Connected</Badge>}
              {connectionStatus === 'unreachable' && (
                <>
                  <Badge variant="destructive" className="text-xs">Unreachable</Badge>
                  {connectionError && <span className="text-xs text-red-400">{connectionError}</span>}
                </>
              )}
            </div>
          </div>

          <div className="border-t border-border pt-2 space-y-2">
            {/* GitHub */}
            <div className="space-y-1">
              <Label className="text-xs">GitHub Token</Label>
              <Input
                type="password"
                placeholder="ghp_... (optional)"
                value={config.github_token || ''}
                onChange={(e) => updateField('github_token', e.target.value)}
                className="h-7 text-xs"
              />
            </div>

            {/* Jira */}
            <div className="space-y-1">
              <Label className="text-xs">Jira Base URL</Label>
              <Input
                placeholder="https://your-org.atlassian.net"
                value={config.jira_base_url || ''}
                onChange={(e) => updateField('jira_base_url', e.target.value)}
                className="h-7 text-xs"
              />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <Label className="text-xs">Jira Email</Label>
                <Input
                  placeholder="user@company.com"
                  value={config.jira_email || ''}
                  onChange={(e) => updateField('jira_email', e.target.value)}
                  className="h-7 text-xs"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Jira Token</Label>
                <Input
                  type="password"
                  placeholder="(optional)"
                  value={config.jira_token || ''}
                  onChange={(e) => updateField('jira_token', e.target.value)}
                  className="h-7 text-xs"
                />
              </div>
            </div>

            {/* Linear */}
            <div className="space-y-1">
              <Label className="text-xs">Linear API Key</Label>
              <Input
                type="password"
                placeholder="lin_api_..."
                value={config.linear_token || ''}
                onChange={(e) => updateField('linear_token', e.target.value)}
                className="h-7 text-xs"
              />
            </div>

            {/* Notion */}
            <div className="space-y-1">
              <Label className="text-xs">Notion Token</Label>
              <Input
                type="password"
                placeholder="ntn_..."
                value={config.notion_token || ''}
                onChange={(e) => updateField('notion_token', e.target.value)}
                className="h-7 text-xs"
              />
            </div>

            {/* Slack */}
            <div className="space-y-1">
              <Label className="text-xs">Slack Bot Token</Label>
              <Input
                type="password"
                placeholder="xoxb-..."
                value={config.slack_token || ''}
                onChange={(e) => updateField('slack_token', e.target.value)}
                className="h-7 text-xs"
              />
            </div>
          </div>
        </div>
      </div>

      <div className="mt-3">
        <Button size="sm" onClick={save} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>
    </div>
  )
}
