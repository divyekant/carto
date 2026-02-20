import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'

interface SourceDef {
  key: string
  label: string
  credentialKeys: string[]
  fields: { key: string; label: string; placeholder: string }[]
}

const SOURCE_DEFS: SourceDef[] = [
  {
    key: 'github',
    label: 'GitHub',
    credentialKeys: ['github_token'],
    fields: [
      { key: 'owner', label: 'Owner', placeholder: 'e.g. divyekant' },
      { key: 'repo', label: 'Repository', placeholder: 'e.g. carto' },
    ],
  },
  {
    key: 'jira',
    label: 'Jira',
    credentialKeys: ['jira_token', 'jira_email'],
    fields: [
      { key: 'url', label: 'Base URL', placeholder: 'https://your-org.atlassian.net' },
      { key: 'project', label: 'Project Key', placeholder: 'e.g. PROJ' },
    ],
  },
  {
    key: 'linear',
    label: 'Linear',
    credentialKeys: ['linear_token'],
    fields: [
      { key: 'team', label: 'Team Key', placeholder: 'e.g. ENG' },
    ],
  },
  {
    key: 'notion',
    label: 'Notion',
    credentialKeys: ['notion_token'],
    fields: [
      { key: 'database', label: 'Database ID', placeholder: 'e.g. abc123-def456' },
    ],
  },
  {
    key: 'slack',
    label: 'Slack',
    credentialKeys: ['slack_token'],
    fields: [
      { key: 'channels', label: 'Channel ID', placeholder: 'e.g. C01234ABC' },
    ],
  },
  {
    key: 'web',
    label: 'Web Pages',
    credentialKeys: [],
    fields: [
      { key: 'urls', label: 'URLs', placeholder: 'https://docs.example.com (comma-separated)' },
    ],
  },
]

interface SourcesEditorProps {
  projectName: string
}

export function SourcesEditor({ projectName }: SourcesEditorProps) {
  const [sources, setSources] = useState<Record<string, Record<string, string>>>({})
  const [credentials, setCredentials] = useState<Record<string, boolean>>({})
  const [enabled, setEnabled] = useState<Record<string, boolean>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetch(`/api/projects/${encodeURIComponent(projectName)}/sources`)
      .then(r => r.json())
      .then(data => {
        setSources(data.sources || {})
        setCredentials(data.credentials || {})
        const en: Record<string, boolean> = {}
        for (const key of Object.keys(data.sources || {})) {
          en[key] = true
        }
        setEnabled(en)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [projectName])

  function toggleSource(key: string) {
    setEnabled(prev => {
      const next = { ...prev, [key]: !prev[key] }
      if (!next[key]) {
        setSources(prev => {
          const copy = { ...prev }
          delete copy[key]
          return copy
        })
      }
      return next
    })
  }

  function updateField(sourceKey: string, fieldKey: string, value: string) {
    setSources(prev => ({
      ...prev,
      [sourceKey]: { ...(prev[sourceKey] || {}), [fieldKey]: value },
    }))
  }

  async function save() {
    setSaving(true)
    try {
      const payload: Record<string, Record<string, string>> = {}
      for (const [key, settings] of Object.entries(sources)) {
        if (enabled[key]) {
          payload[key] = settings
        }
      }

      const res = await fetch(`/api/projects/${encodeURIComponent(projectName)}/sources`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sources: payload }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      toast.success('Sources saved')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <p className="text-sm text-muted-foreground">Loading sources...</p>
  }

  function credStatus(def: SourceDef): 'ok' | 'missing' | 'na' {
    if (def.credentialKeys.length === 0) return 'na'
    return def.credentialKeys.every(k => credentials[k]) ? 'ok' : 'missing'
  }

  return (
    <div className="space-y-4">
      {SOURCE_DEFS.map(def => {
        const isEnabled = enabled[def.key] || false
        const cred = credStatus(def)
        const settings = sources[def.key] || {}

        return (
          <div key={def.key} className="border border-border rounded-lg p-4">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <button
                  onClick={() => toggleSource(def.key)}
                  className={`w-9 h-5 rounded-full transition-colors relative ${isEnabled ? 'bg-primary' : 'bg-muted'}`}
                >
                  <span className={`block w-3.5 h-3.5 rounded-full bg-white absolute top-[3px] transition-transform ${isEnabled ? 'translate-x-[18px]' : 'translate-x-[2px]'}`} />
                </button>
                <span className="font-medium text-sm">{def.label}</span>
              </div>
              {cred === 'ok' && <Badge variant="default" className="text-xs">Token configured</Badge>}
              {cred === 'missing' && (
                <a href="/settings" className="text-xs text-amber-500 hover:underline">
                  Set up in Settings &rarr;
                </a>
              )}
            </div>

            {isEnabled && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
                {def.fields.map(field => (
                  <div key={field.key} className="space-y-1">
                    <Label className="text-xs">{field.label}</Label>
                    <Input
                      placeholder={field.placeholder}
                      value={settings[field.key] || ''}
                      onChange={e => updateField(def.key, field.key, e.target.value)}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        )
      })}

      <Button onClick={save} disabled={saving}>
        {saving ? 'Saving...' : 'Save Sources'}
      </Button>
    </div>
  )
}
