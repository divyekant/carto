import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { QueryResult } from '@/components/QueryResult'
import { Section } from '@/components/Section'

interface Project {
  name: string
}

interface Result {
  id: string
  text: string
  score: number
  source: string
  match_type?: string
  confidence?: number
  graph_support?: number
  metadata?: Record<string, string>
}

const QUICK_QUERIES = [
  'authentication flow', 'error handling', 'API endpoints',
  'database schema', 'configuration', 'middleware',
  'testing patterns', 'data models',
]

const tiers = ['mini', 'standard', 'full'] as const
type Tier = (typeof tiers)[number]
const PAGE_SIZE = 20

export default function Query() {
  const [projects, setProjects] = useState<Project[]>([])
  const [project, setProject] = useState('')
  const [text, setText] = useState('')
  const [tier, setTier] = useState<Tier>('standard')
  const [k, setK] = useState(10)
  const [results, setResults] = useState<Result[]>([])
  const [searching, setSearching] = useState(false)
  const [searched, setSearched] = useState(false)
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)

  // Advanced filter state
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [graphWeight, setGraphWeight] = useState(0.1)
  const [confidenceWeight, setConfidenceWeight] = useState(0)
  const [feedbackWeight, setFeedbackWeight] = useState(0.1)
  const [since, setSince] = useState('')
  const [until, setUntil] = useState('')

  useEffect(() => {
    fetch('/api/projects')
      .then(r => r.json())
      .then(data => {
        const projs = (Array.isArray(data) ? data : data.projects || []) as Project[]
        setProjects(projs)
        if (projs.length > 0) setProject(projs[0].name)
      })
      .catch(console.error)
  }, [])

  async function search() {
    if (!text.trim() || !project) return
    setSearching(true)
    setResults([])
    setVisibleCount(PAGE_SIZE)
    setSearched(false)

    try {
      const res = await fetch('/api/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          text: text.trim(), project, tier, k,
          ...(showAdvanced && {
            graph_weight: graphWeight,
            confidence_weight: confidenceWeight,
            feedback_weight: feedbackWeight,
            ...(since && { since }),
            ...(until && { until }),
          }),
        }),
      })
      const data = await res.json()
      setResults(data.results || [])
    } catch (err) {
      console.error(err)
    } finally {
      setSearching(false)
      setSearched(true)
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') search()
  }

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Query</h2>

      <Section title="Search">
        {/* Single-row search bar with all filters inline */}
        <div className="flex items-end gap-3 flex-wrap">
          <div className="flex-1 min-w-[200px]">
            <Label htmlFor="query" className="text-sm font-medium mb-1 block">Search Query</Label>
            <Input
              id="query"
              placeholder="Describe what you're looking for..."
              value={text}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={handleKeyDown}
            />
          </div>

          <div className="w-40">
            <Label className="text-sm font-medium mb-1 block">Project</Label>
            <Select value={project} onValueChange={setProject}>
              <SelectTrigger>
                <SelectValue placeholder="Select project" />
              </SelectTrigger>
              <SelectContent>
                {projects.map((p) => (
                  <SelectItem key={p.name} value={p.name}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div>
            <Label className="text-sm font-medium mb-1 block">Tier</Label>
            <div className="flex rounded-md overflow-hidden border border-border">
              {tiers.map((t) => (
                <button
                  key={t}
                  onClick={() => setTier(t)}
                  className={cn(
                    'px-2.5 py-1.5 text-sm capitalize transition-colors',
                    tier === t
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-secondary text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t}
                </button>
              ))}
            </div>
          </div>

          <div className="w-16">
            <Label htmlFor="count" className="text-sm font-medium mb-1 block">Count</Label>
            <Input
              id="count"
              type="number"
              min={1}
              max={50}
              value={k}
              onChange={(e) => setK(Math.max(1, Math.min(50, Number(e.target.value))))}
              className="w-16"
            />
          </div>

          <Button size="sm" onClick={search} disabled={searching || !text.trim() || !project}>
            {searching ? 'Searching...' : 'Search'}
          </Button>
        </div>

        {/* Advanced Filters toggle */}
        <div className="mt-3">
          <button
            onClick={() => setShowAdvanced(v => !v)}
            className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
          >
            <span className={cn('transition-transform', showAdvanced ? 'rotate-90' : '')}>▶</span>
            Advanced Filters
          </button>

          {showAdvanced && (
            <div className="mt-3 p-3 rounded-md border border-border bg-muted/30 flex flex-wrap gap-4">
              <div className="flex flex-col gap-1 min-w-[120px]">
                <Label className="text-xs font-medium">Graph Weight</Label>
                <div className="flex items-center gap-2">
                  <input
                    type="range"
                    min={0}
                    max={1}
                    step={0.1}
                    value={graphWeight}
                    onChange={(e) => setGraphWeight(Number(e.target.value))}
                    className="w-24"
                  />
                  <span className="text-xs text-muted-foreground w-6">{graphWeight.toFixed(1)}</span>
                </div>
              </div>

              <div className="flex flex-col gap-1 min-w-[120px]">
                <Label className="text-xs font-medium">Confidence Weight</Label>
                <div className="flex items-center gap-2">
                  <input
                    type="range"
                    min={0}
                    max={1}
                    step={0.1}
                    value={confidenceWeight}
                    onChange={(e) => setConfidenceWeight(Number(e.target.value))}
                    className="w-24"
                  />
                  <span className="text-xs text-muted-foreground w-6">{confidenceWeight.toFixed(1)}</span>
                </div>
              </div>

              <div className="flex flex-col gap-1 min-w-[120px]">
                <Label className="text-xs font-medium">Feedback Weight</Label>
                <div className="flex items-center gap-2">
                  <input
                    type="range"
                    min={0}
                    max={1}
                    step={0.1}
                    value={feedbackWeight}
                    onChange={(e) => setFeedbackWeight(Number(e.target.value))}
                    className="w-24"
                  />
                  <span className="text-xs text-muted-foreground w-6">{feedbackWeight.toFixed(1)}</span>
                </div>
              </div>

              <div className="flex flex-col gap-1">
                <Label htmlFor="since" className="text-xs font-medium">Since</Label>
                <Input
                  id="since"
                  type="date"
                  value={since}
                  onChange={(e) => setSince(e.target.value)}
                  className="h-8 text-xs w-36"
                />
              </div>

              <div className="flex flex-col gap-1">
                <Label htmlFor="until" className="text-xs font-medium">Until</Label>
                <Input
                  id="until"
                  type="date"
                  value={until}
                  onChange={(e) => setUntil(e.target.value)}
                  className="h-8 text-xs w-36"
                />
              </div>
            </div>
          )}
        </div>
      </Section>

      {!searched && (
        <Section title="Quick Queries" className="mt-6">
          <p className="mb-3 text-sm text-muted-foreground">Try one of these to get started:</p>
          <div className="flex flex-wrap gap-2">
            {QUICK_QUERIES.map(q => (
              <button
                key={q}
                onClick={() => setText(q)}
                className="rounded-full border border-border/50 px-3 py-1.5 text-sm hover:bg-muted transition-colors"
              >
                {q}
              </button>
            ))}
          </div>
        </Section>
      )}

      {searched && results.length > 0 && (
        <Section title={`Results (${results.length} matches)`} className="mt-6">
          {results.slice(0, visibleCount).map((r, i) => (
            <QueryResult
              key={r.id || i}
              index={i + 1}
              source={r.source}
              score={r.score}
              text={r.text}
              matchType={r.match_type}
              confidence={r.confidence}
              metadata={r.metadata}
            />
          ))}
          {results.length > visibleCount && (
            <div className="text-center py-3">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setVisibleCount(prev => prev + PAGE_SIZE)}
              >
                Show more ({results.length - visibleCount} remaining)
              </Button>
            </div>
          )}
        </Section>
      )}

      {searched && results.length === 0 && (
        <Section className="mt-6">
          <p className="text-sm text-muted-foreground text-center py-8">No results found. Try a different query or project.</p>
        </Section>
      )}
    </div>
  )
}
