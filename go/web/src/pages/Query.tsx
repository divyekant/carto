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

interface Project {
  name: string
}

interface Result {
  id: string
  text: string
  score: number
  source: string
}

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
        body: JSON.stringify({ text: text.trim(), project, tier, k }),
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

      <div className="space-y-4 max-w-2xl mb-6">
        <div className="space-y-2">
          <Label htmlFor="query">Search Query</Label>
          <Input
            id="query"
            placeholder="Describe what you're looking for..."
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={handleKeyDown}
          />
        </div>

        <div className="flex items-end gap-4 flex-wrap">
          <div className="space-y-2">
            <Label>Project</Label>
            <Select value={project} onValueChange={setProject}>
              <SelectTrigger className="w-48">
                <SelectValue placeholder="Select project" />
              </SelectTrigger>
              <SelectContent>
                {projects.map((p) => (
                  <SelectItem key={p.name} value={p.name}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>Tier</Label>
            <div className="flex rounded-md overflow-hidden border border-border">
              {tiers.map((t) => (
                <button
                  key={t}
                  onClick={() => setTier(t)}
                  className={cn(
                    'px-3 py-1.5 text-sm capitalize transition-colors',
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

          <div className="space-y-2">
            <Label htmlFor="count">Count</Label>
            <Input
              id="count"
              type="number"
              min={1}
              max={50}
              value={k}
              onChange={(e) => setK(Math.max(1, Math.min(50, Number(e.target.value))))}
              className="w-20"
            />
          </div>

          <Button onClick={search} disabled={searching || !text.trim() || !project}>
            {searching ? 'Searching...' : 'Search'}
          </Button>
        </div>
      </div>

      <div className="space-y-3">
        {results.slice(0, visibleCount).map((r, i) => (
          <QueryResult key={r.id || i} index={i + 1} source={r.source} score={r.score} text={r.text} />
        ))}
        {results.length > visibleCount && (
          <div className="text-center py-4">
            <Button
              variant="secondary"
              onClick={() => setVisibleCount(prev => prev + PAGE_SIZE)}
            >
              Show more ({results.length - visibleCount} remaining)
            </Button>
          </div>
        )}
        {searched && results.length === 0 && (
          <p className="text-muted-foreground text-sm py-8 text-center">No results found.</p>
        )}
      </div>
    </div>
  )
}
