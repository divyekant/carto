import { useEffect, useState } from 'react'
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
import { Section } from '@/components/Section'
import { apiFetch } from '@/lib/api'
import { cn } from '@/lib/utils'

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
  fast_max_tokens: number
  deep_max_tokens: number
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
const SECRET_FIELDS = new Set<keyof Config>([
  'anthropic_key',
  'llm_api_key',
  'memories_key',
  'github_token',
  'jira_token',
  'linear_token',
  'notion_token',
  'slack_token',
])

// ─── Validation ──────────────────────────────────────────────────────────────

interface ValidationErrors {
  // LLM Provider
  provider?: string
  apiKey?: string
  baseUrl?: string
  fastModel?: string
  deepModel?: string
  // Memories
  memoriesUrl?: string
  // Performance
  maxConcurrent?: string
  fastMaxTokens?: string
  deepMaxTokens?: string
  // Integrations
  githubToken?: string
  jiraBaseUrl?: string
  jiraEmail?: string
  linearToken?: string
  notionToken?: string
  slackToken?: string
}

function validate(config: Config): ValidationErrors {
  const errors: ValidationErrors = {}
  const provider = config.llm_provider

  // ── Provider & API key ──
  if (!provider) {
    errors.provider = 'Provider is required'
  }

  if (provider === 'anthropic') {
    if (!config.anthropic_key) {
      errors.apiKey = 'Anthropic API key is required'
    } else if (
      !config.anthropic_key.includes('****') &&
      !config.anthropic_key.startsWith('sk-ant-')
    ) {
      errors.apiKey = 'Expected format: sk-ant-api03-…'
    }
  } else if (provider === 'openai') {
    if (!config.llm_api_key) {
      errors.apiKey = 'API key is required for OpenAI'
    } else if (
      !config.llm_api_key.includes('****') &&
      !config.llm_api_key.startsWith('sk-')
    ) {
      errors.apiKey = 'Expected format: sk-…'
    }
  }

  // ── Base URL ──
  if (provider && provider !== 'anthropic' && !config.llm_base_url) {
    errors.baseUrl = 'Base URL is required for ' + provider
  }
  if (config.llm_base_url && !config.llm_base_url.match(/^https?:\/\//)) {
    errors.baseUrl = 'Must start with http:// or https://'
  }

  // ── Models ──
  if (!config.fast_model) errors.fastModel = 'Fast model is required'
  if (!config.deep_model) errors.deepModel = 'Deep model is required'

  // ── Memories URL ──
  if (!config.memories_url) {
    errors.memoriesUrl = 'Memories URL is required'
  } else if (!config.memories_url.match(/^https?:\/\//)) {
    errors.memoriesUrl = 'Must start with http:// or https://'
  }

  // ── Performance ──
  const maxConcurrent = Number(config.max_concurrent)
  if (!Number.isInteger(maxConcurrent) || maxConcurrent < 1 || maxConcurrent > 100) {
    errors.maxConcurrent = 'Must be an integer between 1 and 100'
  }

  const fastMaxTokens = Number(config.fast_max_tokens)
  if (!Number.isInteger(fastMaxTokens) || fastMaxTokens < 256 || fastMaxTokens > 65536) {
    errors.fastMaxTokens = 'Must be between 256 and 65,536'
  }

  const deepMaxTokens = Number(config.deep_max_tokens)
  if (!Number.isInteger(deepMaxTokens) || deepMaxTokens < 256 || deepMaxTokens > 65536) {
    errors.deepMaxTokens = 'Must be between 256 and 65,536'
  }

  // ── Integration format hints (only validate if a non-masked value is present) ──
  if (
    config.github_token &&
    !config.github_token.includes('****') &&
    !config.github_token.startsWith('ghp_') &&
    !config.github_token.startsWith('github_pat_') &&
    !config.github_token.startsWith('gho_') &&
    !config.github_token.startsWith('ghs_')
  ) {
    errors.githubToken = 'Expected format: ghp_… or github_pat_…'
  }

  if (config.jira_base_url && !config.jira_base_url.match(/^https?:\/\//)) {
    errors.jiraBaseUrl = 'Must start with http:// or https://'
  }

  if (config.jira_email && !config.jira_email.match(/^[^\s@]+@[^\s@]+\.[^\s@]+$/)) {
    errors.jiraEmail = 'Must be a valid email address'
  }

  if (
    config.linear_token &&
    !config.linear_token.includes('****') &&
    !config.linear_token.startsWith('lin_api_')
  ) {
    errors.linearToken = 'Expected format: lin_api_…'
  }

  if (
    config.notion_token &&
    !config.notion_token.includes('****') &&
    !config.notion_token.startsWith('ntn_') &&
    !config.notion_token.startsWith('secret_')
  ) {
    errors.notionToken = 'Expected format: ntn_… or secret_…'
  }

  if (
    config.slack_token &&
    !config.slack_token.includes('****') &&
    !config.slack_token.startsWith('xoxb-') &&
    !config.slack_token.startsWith('xoxp-') &&
    !config.slack_token.startsWith('xoxa-') &&
    !config.slack_token.startsWith('xoxs-')
  ) {
    errors.slackToken = 'Expected format: xoxb-… or xoxp-…'
  }

  return errors
}

// All fields that exist in the form — used to mark everything touched on save.
const ALL_TOUCHED_FIELDS = [
  'llm_provider', 'anthropic_key', 'llm_api_key', 'llm_base_url',
  'fast_model', 'deep_model',
  'memories_url',
  'max_concurrent', 'fast_max_tokens', 'deep_max_tokens',
  'github_token', 'jira_base_url', 'jira_email', 'jira_token',
  'linear_token', 'notion_token', 'slack_token',
]

// ─── ModelSelect ─────────────────────────────────────────────────────────────

function ModelSelect({ label, description, models, value, onChange, error }: {
  label: string
  description: string
  models: ModelOption[]
  value: string
  onChange: (value: string) => void
  error?: string
}) {
  const isKnownModel = models.some(m => m.value === value)
  const [customMode, setCustomMode] = useState(value !== '' && !isKnownModel)
  const showCustomInput = customMode || (value !== '' && !isKnownModel)

  function handleSelectChange(v: string) {
    if (v === CUSTOM_MODEL_VALUE) {
      setCustomMode(true)
      onChange('')
    } else {
      setCustomMode(false)
      onChange(v)
    }
  }

  return (
    <div className="space-y-1">
      <Label className="text-sm font-medium">{label}</Label>
      <Select
        value={showCustomInput ? CUSTOM_MODEL_VALUE : value}
        onValueChange={handleSelectChange}
      >
        <SelectTrigger className={cn('w-full h-8 text-xs', error && 'border-red-500 focus-visible:ring-red-500')}>
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
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className={cn('h-7 text-xs', error && 'border-red-500 focus-visible:ring-red-500')}
          autoFocus
        />
      )}
      {error && <p className="text-xs text-red-400">{error}</p>}
      <p className="text-xs text-muted-foreground">{description}</p>
    </div>
  )
}

// ─── Settings page ────────────────────────────────────────────────────────────

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
    fast_max_tokens: 4096,
    deep_max_tokens: 8192,
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
      apiFetch<Config>('/config'),
      apiFetch<{ docker?: boolean }>('/health'),
    ]).then(([configData, healthData]) => {
      const memoriesUrl = configData.memories_url?.replace('host.docker.internal', 'localhost') || configData.memories_url
      setConfig({ ...configData, memories_url: memoriesUrl })
      setIsDockerEnv(healthData.docker === true)
    }).catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  function updateField(key: keyof Config, value: string | number) {
    setConfig(prev => {
      const next = { ...prev, [key]: value }
      // Re-run validation on every change so errors update live
      setErrors(validate(next))
      return next
    })
    setTouched(prev => new Set(prev).add(key))
  }

  function getPatchValue(key: keyof Config): string | number | undefined {
    const value = config[key]
    if (typeof value !== 'string') {
      return value
    }
    if (SECRET_FIELDS.has(key)) {
      return value.includes('****') ? undefined : value
    }
    return value
  }

  function handleProviderChange(provider: string) {
    const defaults = PROVIDER_DEFAULTS[provider]
    if (!defaults) return

    setConfig(prev => {
      const next = {
        ...prev,
        llm_provider: provider,
        fast_model: defaults.fast,
        deep_model: defaults.deep,
        llm_base_url: defaults.baseUrl,
      }
      setErrors(validate(next))
      return next
    })
    setTouched(prev => {
      const next = new Set(prev)
      next.add('llm_provider')
      return next
    })
  }

  async function save() {
    const validationErrors = validate(config)
    setErrors(validationErrors)
    setTouched(new Set(ALL_TOUCHED_FIELDS))

    if (Object.keys(validationErrors).length > 0) {
      toast.error('Please fix the errors highlighted below.')
      return
    }

    setSaving(true)
    try {
      const patch: Record<string, unknown> = {
        llm_provider: getPatchValue('llm_provider'),
        fast_model: getPatchValue('fast_model'),
        deep_model: getPatchValue('deep_model'),
        memories_url: getPatchValue('memories_url'),
        max_concurrent: getPatchValue('max_concurrent'),
        fast_max_tokens: getPatchValue('fast_max_tokens'),
        deep_max_tokens: getPatchValue('deep_max_tokens'),
      }

      for (const key of [
        'anthropic_key',
        'llm_api_key',
        'memories_key',
        'llm_base_url',
        'github_token',
        'jira_token',
        'jira_email',
        'jira_base_url',
        'linear_token',
        'notion_token',
        'slack_token',
      ] as const) {
        const value = getPatchValue(key)
        if (value !== undefined) {
          patch[key] = value
        }
      }

      await apiFetch('/config', {
        method: 'PATCH',
        body: JSON.stringify(patch),
      })
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
      const data = await apiFetch<{ connected?: boolean; error?: string }>('/test-memories', {
        method: 'POST',
        body: JSON.stringify({
          url: config.memories_url,
          api_key: config.memories_key.includes('****') ? '' : config.memories_key,
        }),
      })
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

  // Helper: return error message only when the field has been touched
  function fieldError(touchKey: string, err?: string) {
    return touched.has(touchKey) && err ? err : undefined
  }

  if (loading) {
    return (
      <div>
        <h2 className="text-2xl font-bold mb-3">Settings</h2>
        <p className="text-muted-foreground text-sm">Loading...</p>
      </div>
    )
  }

  const provider = config.llm_provider || 'anthropic'
  const defaults = PROVIDER_DEFAULTS[provider] || PROVIDER_DEFAULTS.anthropic
  const showBaseUrl = provider !== 'anthropic'
  const showLlmApiKey = provider !== 'anthropic'

  return (
    <div className="space-y-6">
      {/* Header with save */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">Settings</h2>
        <Button onClick={save} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>

      {isDockerEnv && (
        <div className="rounded-md border border-blue-500/30 bg-blue-500/10 p-2 text-xs text-blue-400">
          Running in Docker — <code className="text-xs bg-muted px-1 rounded">localhost</code> URLs are automatically routed to your host machine.
        </div>
      )}

      {/* ── Section 1: LLM Provider ── */}
      <Section title="LLM Provider">
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-2">
            {/* Provider selector */}
            <div className="space-y-1">
              <Label className="text-sm font-medium">Provider</Label>
              <Select value={provider} onValueChange={handleProviderChange}>
                <SelectTrigger className={cn('w-full h-8 text-xs', fieldError('llm_provider', errors.provider) && 'border-red-500 focus-visible:ring-red-500')}>
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="anthropic">Anthropic</SelectItem>
                  <SelectItem value="openai">OpenAI-Compatible</SelectItem>
                  <SelectItem value="ollama">Ollama</SelectItem>
                </SelectContent>
              </Select>
              {fieldError('llm_provider', errors.provider) && (
                <p className="text-xs text-red-400">{errors.provider}</p>
              )}
            </div>

            {/* Anthropic API key */}
            {provider === 'anthropic' && (
              <div className="space-y-1">
                <Label className="text-sm font-medium">API Key</Label>
                <Input
                  type="password"
                  placeholder="sk-ant-api03-..."
                  value={config.anthropic_key || ''}
                  onChange={(e) => updateField('anthropic_key', e.target.value)}
                  className={cn('h-8 text-xs', fieldError('anthropic_key', errors.apiKey) && 'border-red-500 focus-visible:ring-red-500')}
                />
                <p className="text-xs text-muted-foreground">Format: sk-ant-api03-…</p>
                {fieldError('anthropic_key', errors.apiKey) && (
                  <p className="text-xs text-red-400">{errors.apiKey}</p>
                )}
              </div>
            )}

            {/* OpenAI-compatible API key */}
            {showLlmApiKey && (
              <div className="space-y-1">
                <Label className="text-sm font-medium">API Key</Label>
                <Input
                  type={provider === 'ollama' ? 'text' : 'password'}
                  placeholder={defaults.keyPlaceholder}
                  value={config.llm_api_key || ''}
                  onChange={(e) => updateField('llm_api_key', e.target.value)}
                  disabled={provider === 'ollama'}
                  className={cn('h-8 text-xs', fieldError('llm_api_key', errors.apiKey) && 'border-red-500 focus-visible:ring-red-500')}
                />
                {provider !== 'ollama' && (
                  <p className="text-xs text-muted-foreground">Format: {defaults.keyPlaceholder}</p>
                )}
                {fieldError('llm_api_key', errors.apiKey) && (
                  <p className="text-xs text-red-400">{errors.apiKey}</p>
                )}
              </div>
            )}
          </div>

          {/* Base URL */}
          {showBaseUrl && (
            <div className="space-y-1">
              <Label className="text-sm font-medium">Base URL</Label>
              <Input
                placeholder={defaults.baseUrl}
                value={config.llm_base_url || ''}
                onChange={(e) => updateField('llm_base_url', e.target.value)}
                className={cn('h-8 text-xs', fieldError('llm_base_url', errors.baseUrl) && 'border-red-500 focus-visible:ring-red-500')}
              />
              <p className="text-xs text-muted-foreground">Must start with http:// or https://</p>
              {fieldError('llm_base_url', errors.baseUrl) && (
                <p className="text-xs text-red-400">{errors.baseUrl}</p>
              )}
            </div>
          )}

          {/* Model selectors */}
          <div className="grid grid-cols-2 gap-2">
            <ModelSelect
              key={`fast-${provider}`}
              label="Fast Model"
              description="High-volume, low-cost"
              models={defaults.fastModels}
              value={config.fast_model || ''}
              onChange={(v) => updateField('fast_model', v)}
              error={fieldError('fast_model', errors.fastModel)}
            />
            <ModelSelect
              key={`deep-${provider}`}
              label="Deep Model"
              description="Low-volume, high-cost"
              models={defaults.deepModels}
              value={config.deep_model || ''}
              onChange={(v) => updateField('deep_model', v)}
              error={fieldError('deep_model', errors.deepModel)}
            />
          </div>
        </div>
      </Section>

      {/* ── Section 2: Performance ── */}
      <Section title="Performance">
        <div className="grid grid-cols-3 gap-2">
          {/* Max Concurrent */}
          <div className="space-y-1">
            <Label className="text-sm font-medium">Max Concurrent</Label>
            <Input
              type="number"
              min={1}
              max={100}
              placeholder="10"
              value={config.max_concurrent || 10}
              onChange={(e) => updateField('max_concurrent', parseInt(e.target.value, 10) || 10)}
              className={cn('h-8 text-xs', fieldError('max_concurrent', errors.maxConcurrent) && 'border-red-500 focus-visible:ring-red-500')}
            />
            <p className="text-xs text-muted-foreground">Parallel LLM calls (1–100)</p>
            {fieldError('max_concurrent', errors.maxConcurrent) && (
              <p className="text-xs text-red-400">{errors.maxConcurrent}</p>
            )}
          </div>

          {/* Fast Max Tokens */}
          <div className="space-y-1">
            <Label className="text-sm font-medium">Fast Max Tokens</Label>
            <Input
              type="number"
              min={256}
              max={65536}
              placeholder="4096"
              value={config.fast_max_tokens || 4096}
              onChange={(e) => updateField('fast_max_tokens', parseInt(e.target.value, 10) || 4096)}
              className={cn('h-8 text-xs', fieldError('fast_max_tokens', errors.fastMaxTokens) && 'border-red-500 focus-visible:ring-red-500')}
            />
            <p className="text-xs text-muted-foreground">Fast model output cap (256–65,536)</p>
            {fieldError('fast_max_tokens', errors.fastMaxTokens) && (
              <p className="text-xs text-red-400">{errors.fastMaxTokens}</p>
            )}
          </div>

          {/* Deep Max Tokens */}
          <div className="space-y-1">
            <Label className="text-sm font-medium">Deep Max Tokens</Label>
            <Input
              type="number"
              min={256}
              max={65536}
              placeholder="8192"
              value={config.deep_max_tokens || 8192}
              onChange={(e) => updateField('deep_max_tokens', parseInt(e.target.value, 10) || 8192)}
              className={cn('h-8 text-xs', fieldError('deep_max_tokens', errors.deepMaxTokens) && 'border-red-500 focus-visible:ring-red-500')}
            />
            <p className="text-xs text-muted-foreground">Deep model output cap (256–65,536)</p>
            {fieldError('deep_max_tokens', errors.deepMaxTokens) && (
              <p className="text-xs text-red-400">{errors.deepMaxTokens}</p>
            )}
          </div>
        </div>
      </Section>

      {/* ── Section 3: Memories Server ── */}
      <Section title="Memories Server">
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-2">
            <div className="space-y-1">
              <Label className="text-sm font-medium">Memories URL</Label>
              <Input
                placeholder="http://localhost:8900"
                value={config.memories_url || ''}
                onChange={(e) => updateField('memories_url', e.target.value)}
                className={cn('h-8 text-xs', fieldError('memories_url', errors.memoriesUrl) && 'border-red-500 focus-visible:ring-red-500')}
              />
              <p className="text-xs text-muted-foreground">Must start with http:// or https://</p>
              {fieldError('memories_url', errors.memoriesUrl) && (
                <p className="text-xs text-red-400">{errors.memoriesUrl}</p>
              )}
            </div>
            <div className="space-y-1">
              <Label className="text-sm font-medium">Memories Key</Label>
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
      </Section>

      {/* ── Section 4: Integrations ── */}
      <Section title="Integrations">
        <div className="space-y-3">
          {/* GitHub */}
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <span className={cn('h-2 w-2 shrink-0 rounded-full',
                config.github_token ? 'bg-emerald-500' : 'bg-muted-foreground/30'
              )} />
              <Label className="w-24 shrink-0 text-sm font-medium">GitHub</Label>
              <Input
                type="password"
                placeholder="ghp_... (optional)"
                value={config.github_token || ''}
                onChange={(e) => updateField('github_token', e.target.value)}
                className={cn('flex-1 h-8 text-xs', fieldError('github_token', errors.githubToken) && 'border-red-500 focus-visible:ring-red-500')}
              />
            </div>
            {fieldError('github_token', errors.githubToken) && (
              <p className="text-xs text-red-400 ml-[calc(0.5rem+8px+0.75rem+6rem)]">{errors.githubToken}</p>
            )}
          </div>

          {/* Jira */}
          <div className="space-y-2">
            <div className="flex items-center gap-3">
              <span className={cn('h-2 w-2 shrink-0 rounded-full',
                config.jira_token ? 'bg-emerald-500' : 'bg-muted-foreground/30'
              )} />
              <Label className="w-24 shrink-0 text-sm font-medium">Jira</Label>
              <Input
                placeholder="https://your-org.atlassian.net"
                value={config.jira_base_url || ''}
                onChange={(e) => updateField('jira_base_url', e.target.value)}
                className={cn('flex-1 h-8 text-xs', fieldError('jira_base_url', errors.jiraBaseUrl) && 'border-red-500 focus-visible:ring-red-500')}
              />
            </div>
            {fieldError('jira_base_url', errors.jiraBaseUrl) && (
              <p className="text-xs text-red-400 ml-[calc(0.5rem+8px+0.75rem+6rem)]">{errors.jiraBaseUrl}</p>
            )}
            <div className="ml-[calc(0.5rem+8px+0.75rem+6rem)] grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <Input
                  placeholder="user@company.com"
                  value={config.jira_email || ''}
                  onChange={(e) => updateField('jira_email', e.target.value)}
                  className={cn('h-8 text-xs', fieldError('jira_email', errors.jiraEmail) && 'border-red-500 focus-visible:ring-red-500')}
                />
                {fieldError('jira_email', errors.jiraEmail) && (
                  <p className="text-xs text-red-400">{errors.jiraEmail}</p>
                )}
              </div>
              <Input
                type="password"
                placeholder="API Token (optional)"
                value={config.jira_token || ''}
                onChange={(e) => updateField('jira_token', e.target.value)}
                className="h-8 text-xs"
              />
            </div>
          </div>

          {/* Linear */}
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <span className={cn('h-2 w-2 shrink-0 rounded-full',
                config.linear_token ? 'bg-emerald-500' : 'bg-muted-foreground/30'
              )} />
              <Label className="w-24 shrink-0 text-sm font-medium">Linear</Label>
              <Input
                type="password"
                placeholder="lin_api_... (optional)"
                value={config.linear_token || ''}
                onChange={(e) => updateField('linear_token', e.target.value)}
                className={cn('flex-1 h-8 text-xs', fieldError('linear_token', errors.linearToken) && 'border-red-500 focus-visible:ring-red-500')}
              />
            </div>
            {fieldError('linear_token', errors.linearToken) && (
              <p className="text-xs text-red-400 ml-[calc(0.5rem+8px+0.75rem+6rem)]">{errors.linearToken}</p>
            )}
          </div>

          {/* Notion */}
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <span className={cn('h-2 w-2 shrink-0 rounded-full',
                config.notion_token ? 'bg-emerald-500' : 'bg-muted-foreground/30'
              )} />
              <Label className="w-24 shrink-0 text-sm font-medium">Notion</Label>
              <Input
                type="password"
                placeholder="ntn_... (optional)"
                value={config.notion_token || ''}
                onChange={(e) => updateField('notion_token', e.target.value)}
                className={cn('flex-1 h-8 text-xs', fieldError('notion_token', errors.notionToken) && 'border-red-500 focus-visible:ring-red-500')}
              />
            </div>
            {fieldError('notion_token', errors.notionToken) && (
              <p className="text-xs text-red-400 ml-[calc(0.5rem+8px+0.75rem+6rem)]">{errors.notionToken}</p>
            )}
          </div>

          {/* Slack */}
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <span className={cn('h-2 w-2 shrink-0 rounded-full',
                config.slack_token ? 'bg-emerald-500' : 'bg-muted-foreground/30'
              )} />
              <Label className="w-24 shrink-0 text-sm font-medium">Slack</Label>
              <Input
                type="password"
                placeholder="xoxb-... (optional)"
                value={config.slack_token || ''}
                onChange={(e) => updateField('slack_token', e.target.value)}
                className={cn('flex-1 h-8 text-xs', fieldError('slack_token', errors.slackToken) && 'border-red-500 focus-visible:ring-red-500')}
              />
            </div>
            {fieldError('slack_token', errors.slackToken) && (
              <p className="text-xs text-red-400 ml-[calc(0.5rem+8px+0.75rem+6rem)]">{errors.slackToken}</p>
            )}
          </div>
        </div>
      </Section>

      {/* ── Bottom Save button ── */}
      <div className="flex justify-end pb-4">
        <Button onClick={save} disabled={saving} size="lg">
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>
    </div>
  )
}
