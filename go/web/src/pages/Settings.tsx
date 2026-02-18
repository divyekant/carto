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

interface Config {
  provider?: string
  api_key?: string
  base_url?: string
  fast_model?: string
  deep_model?: string
  memories_url?: string
  memories_api_key?: string
  [key: string]: unknown
}

export default function Settings() {
  const [config, setConfig] = useState<Config>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [connectionStatus, setConnectionStatus] = useState<'idle' | 'testing' | 'connected' | 'unreachable'>('idle')

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then(data => setConfig(data))
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  function updateField(key: string, value: string) {
    setConfig(prev => ({ ...prev, [key]: value }))
  }

  async function save() {
    setSaving(true)
    setMessage(null)
    try {
      const res = await fetch('/api/config', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
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
    setConnectionStatus('testing')
    try {
      const res = await fetch('/api/health')
      const data = await res.json()
      setConnectionStatus(data.memories_healthy ? 'connected' : 'unreachable')
    } catch {
      setConnectionStatus('unreachable')
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

  const showBaseUrl = config.provider !== 'anthropic'

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
              <Select value={config.provider || ''} onValueChange={(v) => updateField('provider', v)}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="anthropic">Anthropic</SelectItem>
                  <SelectItem value="openai">OpenAI</SelectItem>
                  <SelectItem value="ollama">Ollama</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="api_key">API Key</Label>
              <Input
                id="api_key"
                type="password"
                placeholder="sk-..."
                value={config.api_key || ''}
                onChange={(e) => updateField('api_key', e.target.value)}
              />
            </div>
            {showBaseUrl && (
              <div className="space-y-2">
                <Label htmlFor="base_url">Base URL</Label>
                <Input
                  id="base_url"
                  placeholder="https://api.example.com"
                  value={config.base_url || ''}
                  onChange={(e) => updateField('base_url', e.target.value)}
                />
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="fast_model">Fast Model</Label>
              <Input
                id="fast_model"
                placeholder="claude-haiku-4-5-20251001"
                value={config.fast_model || ''}
                onChange={(e) => updateField('fast_model', e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="deep_model">Deep Model</Label>
              <Input
                id="deep_model"
                placeholder="claude-opus-4-6"
                value={config.deep_model || ''}
                onChange={(e) => updateField('deep_model', e.target.value)}
              />
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
            </div>
            <div className="space-y-2">
              <Label htmlFor="memories_api_key">API Key</Label>
              <Input
                id="memories_api_key"
                type="password"
                placeholder="memories key"
                value={config.memories_api_key || ''}
                onChange={(e) => updateField('memories_api_key', e.target.value)}
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
                <Badge variant="destructive" className="text-xs">Unreachable</Badge>
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
