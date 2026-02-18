import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ProgressBar } from '@/components/ProgressBar'

type PageState = 'idle' | 'starting' | 'running' | 'complete' | 'error'

interface ProgressData {
  phase: string
  done: number
  total: number
}

interface CompleteData {
  modules: number
  files: number
  atoms: number
  errors: number
  elapsed: string
}

export default function IndexRun() {
  const [state, setState] = useState<PageState>('idle')
  const [path, setPath] = useState('')
  const [module, setModule] = useState('')
  const [incremental, setIncremental] = useState(false)
  const [progress, setProgress] = useState<ProgressData>({ phase: '', done: 0, total: 0 })
  const [result, setResult] = useState<CompleteData | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const eventSourceRef = useRef<EventSource | null>(null)

  useEffect(() => {
    return () => {
      eventSourceRef.current?.close()
    }
  }, [])

  function reset() {
    eventSourceRef.current?.close()
    eventSourceRef.current = null
    setState('idle')
    setProgress({ phase: '', done: 0, total: 0 })
    setResult(null)
    setErrorMsg('')
  }

  async function startIndexing() {
    if (!path.trim()) return
    setState('starting')
    setErrorMsg('')
    setResult(null)

    try {
      const body: Record<string, unknown> = { path: path.trim(), incremental }
      if (module.trim()) body.module = module.trim()

      const res = await fetch('/api/projects/index', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(data.error || `HTTP ${res.status}`)
      }

      const data = await res.json()
      const projectName = data.project

      setState('running')
      connectSSE(projectName)
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
      setState('error')
    }
  }

  function connectSSE(projectName: string) {
    const es = new EventSource(`/api/projects/${encodeURIComponent(projectName)}/progress`)
    eventSourceRef.current = es

    es.addEventListener('progress', (e) => {
      const data: ProgressData = JSON.parse(e.data)
      setProgress(data)
    })

    es.addEventListener('complete', (e) => {
      const data: CompleteData = JSON.parse(e.data)
      setResult(data)
      setState('complete')
      es.close()
    })

    es.addEventListener('error', (e) => {
      if (e instanceof MessageEvent && e.data) {
        const data = JSON.parse(e.data)
        setErrorMsg(data.message || 'Unknown error')
      } else {
        setErrorMsg('Connection to progress stream lost')
      }
      setState('error')
      es.close()
    })
  }

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Index Project</h2>

      {state === 'idle' && (
        <Card className="bg-card border-border max-w-lg">
          <CardHeader>
            <CardTitle className="text-base">Start Indexing</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="path">Project Path</Label>
              <Input
                id="path"
                placeholder="/home/user/project"
                value={path}
                onChange={(e) => setPath(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="module">Module Filter (optional)</Label>
              <Input
                id="module"
                placeholder="e.g. go, symbols"
                value={module}
                onChange={(e) => setModule(e.target.value)}
              />
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="incremental"
                checked={incremental}
                onChange={(e) => setIncremental(e.target.checked)}
                className="rounded border-border"
              />
              <Label htmlFor="incremental">Incremental</Label>
            </div>
            <Button onClick={startIndexing} disabled={!path.trim()}>
              Start Indexing
            </Button>
          </CardContent>
        </Card>
      )}

      {state === 'starting' && (
        <Card className="bg-card border-border max-w-lg">
          <CardContent className="pt-6">
            <p className="text-muted-foreground">Starting indexing...</p>
          </CardContent>
        </Card>
      )}

      {state === 'running' && (
        <Card className="bg-card border-border max-w-lg">
          <CardHeader>
            <CardTitle className="text-base">Indexing in Progress</CardTitle>
          </CardHeader>
          <CardContent>
            <ProgressBar phase={progress.phase} done={progress.done} total={progress.total} />
          </CardContent>
        </Card>
      )}

      {state === 'complete' && result && (
        <Card className="bg-card border-border max-w-lg">
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base">Indexing Complete</CardTitle>
              <Badge variant="default" className="text-xs">Done</Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <span className="text-muted-foreground">Modules</span>
                <p className="text-foreground font-medium">{result.modules}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Files</span>
                <p className="text-foreground font-medium">{result.files}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Atoms</span>
                <p className="text-foreground font-medium">{result.atoms}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Errors</span>
                <p className={result.errors > 0 ? 'text-red-400 font-medium' : 'text-foreground font-medium'}>
                  {result.errors}
                </p>
              </div>
            </div>
            <p className="text-xs text-muted-foreground">Elapsed: {result.elapsed}</p>
            <Button variant="secondary" onClick={reset}>Index Another</Button>
          </CardContent>
        </Card>
      )}

      {state === 'error' && (
        <Card className="bg-card border-destructive/30 max-w-lg">
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base">Error</CardTitle>
              <Badge variant="destructive" className="text-xs">Failed</Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm text-red-400">{errorMsg}</p>
            <Button variant="secondary" onClick={reset}>Try Again</Button>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
