import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
}

const PROVIDER_DEFAULTS: Record<string, { fast: string; deep: string; baseUrl: string; keyPlaceholder: string }> = {
  anthropic: {
    fast: 'claude-haiku-4-5-20251001',
    deep: 'claude-opus-4-6',
    baseUrl: '',
    keyPlaceholder: 'sk-ant-api03-...',
  },
  openai: {
    fast: 'gpt-4o-mini',
    deep: 'gpt-4o',
    baseUrl: 'https://api.openai.com/v1',
    keyPlaceholder: 'sk-...',
  },
  ollama: {
    fast: 'llama3.2',
    deep: 'llama3.2',
    baseUrl: 'http://localhost:11434',
    keyPlaceholder: '(not required for Ollama)',
  },
}

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

  if (!provider) {
    errors.provider = 'Provider is required'
  }

  // API key validation (Anthropic uses anthropic_key, others use llm_api_key)
  // Redacted keys (containing ****) mean the server already has one saved — that's OK.
  // Empty keys mean nothing is configured — flag as error.
  if (provider === 'anthropic') {
    if (!config.anthropic_key) {
      errors.apiKey = 'Anthropic API key is required'
    }
  } else if (provider === 'openai') {
    if (!config.llm_api_key) {
      errors.apiKey = 'API key is required for OpenAI'
    }
  }
  // Ollama doesn't need a key

  // Base URL required for non-Anthropic providers
  if (provider && provider !== 'anthropic' && !config.llm_base_url) {
    errors.baseUrl = 'Base URL is required for ' + provider
  }

  // URL format validation
  if (config.llm_base_url && !config.llm_base_url.match(/^https?:\/\//)) {
    errors.baseUrl = 'Must start with http:// or https://'
  }

  // Model names required
  if (!config.fast_model) {
    errors.fastModel = 'Fast model is required'
  }
  if (!config.deep_model) {
    errors.deepModel = 'Deep model is required'
  }

  // Memories URL validation
  if (!config.memories_url) {
    errors.memoriesUrl = 'Memories URL is required'
  } else if (!config.memories_url.match(/^https?:\/\//)) {
    errors.memoriesUrl = 'Must start with http:// or https://'
  }

  return errors
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
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [connectionStatus, setConnectionStatus] = useState<'idle' | 'testing' | 'connected' | 'unreachable'>('idle')
  const [connectionError, setConnectionError] = useState<string | null>(null)
  const [errors, setErrors] = useState<ValidationErrors>({})
  const [touched, setTouched] = useState<Set<string>>(new Set())

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then((data: Config) => {
        // Normalize the Docker-internal URL for display
        const memoriesUrl = data.memories_url?.replace('host.docker.internal', 'localhost') || data.memories_url
        setConfig({ ...data, memories_url: memoriesUrl })
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  function updateField(key: keyof Config, value: string | number) {
    setConfig(prev => ({ ...prev, [key]: value }))
    setTouched(prev => new Set(prev).add(key))
    setMessage(null)
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
    setMessage(null)
    setErrors({})
  }

  async function save() {
    const validationErrors = validate(config)
    setErrors(validationErrors)
    // Mark all fields as touched on save attempt
    setTouched(new Set(['llm_provider', 'anthropic_key', 'llm_api_key', 'llm_base_url', 'fast_model', 'deep_model', 'memories_url']))

    if (Object.keys(validationErrors).length > 0) {
      setMessage({ type: 'error', text: 'Please fix the errors above.' })
      return
    }

    setSaving(true)
    setMessage(null)
    try {
      // Build a patch with only the fields we want to send
      const patch: Record<string, unknown> = {
        llm_provider: config.llm_provider,
        fast_model: config.fast_model,
        deep_model: config.deep_model,
        memories_url: config.memories_url,
      }

      // Only send API keys if they were actually changed (not redacted)
      if (config.anthropic_key && !config.anthropic_key.includes('****')) {
        patch.anthropic_key = config.anthropic_key
      }
      if (config.llm_api_key && !config.llm_api_key.includes('****')) {
        patch.llm_api_key = config.llm_api_key
      }
      if (config.memories_key && !config.memories_key.includes('****')) {
        patch.memories_key = config.memories_key
      }
      if (config.llm_base_url) {
        patch.llm_base_url = config.llm_base_url
      }

      const res = await fetch('/api/config', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(patch),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setMessage({ type: 'success', text: 'Settings saved successfully.' })
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save' })
    } finally {
      setSaving(false)
    }
  }

  async function testConnection() {
    // Validate URL first
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
      } else {
        setConnectionStatus('unreachable')
        setConnectionError(data.error || 'Connection failed')
      }
    } catch {
      setConnectionStatus('unreachable')
      setConnectionError('Could not reach the server')
    }
  }

  if (loading) {
    return (
      <div>
        <h2 className="text-2xl font-bold mb-6">Settings</h2>
        <p className="text-muted-foreground">Loading...</p>
      </div>
    )
  }

  const provider = config.llm_provider || 'anthropic'
  const defaults = PROVIDER_DEFAULTS[provider] || PROVIDER_DEFAULTS.anthropic
  const showBaseUrl = provider !== 'anthropic'
  const showLlmApiKey = provider !== 'anthropic'

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Settings</h2>

      <div className="space-y-6 max-w-lg">
        <Card className="bg-card border-border">
          <CardHeader>
            <CardTitle className="text-base">LLM Provider</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>Provider</Label>
              <Select value={provider} onValueChange={handleProviderChange}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="anthropic">Anthropic</SelectItem>
                  <SelectItem value="openai">OpenAI-Compatible</SelectItem>
                  <SelectItem value="ollama">Ollama</SelectItem>
                </SelectContent>
              </Select>
              {errors.provider && touched.has('llm_provider') && (
                <p className="text-sm text-red-400">{errors.provider}</p>
              )}
            </div>

            {/* Anthropic uses its own key field */}
            {provider === 'anthropic' && (
              <div className="space-y-2">
                <Label htmlFor="anthropic_key">API Key</Label>
                <Input
                  id="anthropic_key"
                  type="password"
                  placeholder="sk-ant-api03-..."
                  value={config.anthropic_key || ''}
                  onChange={(e) => updateField('anthropic_key', e.target.value)}
                />
                {errors.apiKey && touched.has('anthropic_key') && (
                  <p className="text-sm text-red-400">{errors.apiKey}</p>
                )}
                <p className="text-xs text-muted-foreground">
                  Supports standard keys (sk-ant-api03-) and OAuth tokens (sk-ant-oat01-)
                </p>
              </div>
            )}

            {/* Non-Anthropic providers use llm_api_key */}
            {showLlmApiKey && (
              <div className="space-y-2">
                <Label htmlFor="llm_api_key">API Key</Label>
                <Input
                  id="llm_api_key"
                  type={provider === 'ollama' ? 'text' : 'password'}
                  placeholder={defaults.keyPlaceholder}
                  value={config.llm_api_key || ''}
                  onChange={(e) => updateField('llm_api_key', e.target.value)}
                  disabled={provider === 'ollama'}
                />
                {provider === 'ollama' && (
                  <p className="text-xs text-muted-foreground">Ollama runs locally — no API key needed</p>
                )}
                {errors.apiKey && touched.has('llm_api_key') && (
                  <p className="text-sm text-red-400">{errors.apiKey}</p>
                )}
              </div>
            )}

            {showBaseUrl && (
              <div className="space-y-2">
                <Label htmlFor="llm_base_url">Base URL</Label>
                <Input
                  id="llm_base_url"
                  placeholder={defaults.baseUrl}
                  value={config.llm_base_url || ''}
                  onChange={(e) => updateField('llm_base_url', e.target.value)}
                />
                {errors.baseUrl && touched.has('llm_base_url') && (
                  <p className="text-sm text-red-400">{errors.baseUrl}</p>
                )}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="fast_model">Fast Model</Label>
              <Input
                id="fast_model"
                placeholder={defaults.fast}
                value={config.fast_model || ''}
                onChange={(e) => updateField('fast_model', e.target.value)}
              />
              {errors.fastModel && touched.has('fast_model') && (
                <p className="text-sm text-red-400">{errors.fastModel}</p>
              )}
              <p className="text-xs text-muted-foreground">High-volume, low-cost model for atom analysis</p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="deep_model">Deep Model</Label>
              <Input
                id="deep_model"
                placeholder={defaults.deep}
                value={config.deep_model || ''}
                onChange={(e) => updateField('deep_model', e.target.value)}
              />
              {errors.deepModel && touched.has('deep_model') && (
                <p className="text-sm text-red-400">{errors.deepModel}</p>
              )}
              <p className="text-xs text-muted-foreground">Low-volume, high-cost model for architectural analysis</p>
            </div>
          </CardContent>
        </Card>

        <Card className="bg-card border-border">
          <CardHeader>
            <CardTitle className="text-base">Memories Server</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="memories_url">URL</Label>
              <Input
                id="memories_url"
                placeholder="http://localhost:8900"
                value={config.memories_url || ''}
                onChange={(e) => updateField('memories_url', e.target.value)}
              />
              {errors.memoriesUrl && touched.has('memories_url') && (
                <p className="text-sm text-red-400">{errors.memoriesUrl}</p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="memories_key">API Key</Label>
              <Input
                id="memories_key"
                type="password"
                placeholder="(optional)"
                value={config.memories_key || ''}
                onChange={(e) => updateField('memories_key', e.target.value)}
              />
            </div>
            <div className="flex items-center gap-3">
              <Button variant="secondary" onClick={testConnection} disabled={connectionStatus === 'testing'}>
                {connectionStatus === 'testing' ? 'Testing...' : 'Test Connection'}
              </Button>
              {connectionStatus === 'connected' && (
                <Badge variant="default" className="text-xs">Connected</Badge>
              )}
              {connectionStatus === 'unreachable' && (
                <div className="flex flex-col gap-1">
                  <Badge variant="destructive" className="text-xs">Unreachable</Badge>
                  {connectionError && <p className="text-xs text-red-400">{connectionError}</p>}
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        <div className="flex items-center gap-3">
          <Button onClick={save} disabled={saving}>
            {saving ? 'Saving...' : 'Save Settings'}
          </Button>
          {message && (
            <span className={message.type === 'success' ? 'text-emerald-500 text-sm' : 'text-red-400 text-sm'}>
              {message.text}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
