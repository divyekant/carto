import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
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
  error_messages?: string[]
}

interface LogEntry {
  level: string
  message: string
  timestamp: number
}

export default function IndexRun() {
  const [searchParams] = useSearchParams()
  const [state, setState] = useState<PageState>('idle')
  const [path, setPath] = useState('')
  const [errorsExpanded, setErrorsExpanded] = useState(false)
  const [module, setModule] = useState('')
  const [incremental, setIncremental] = useState(false)
  const [progress, setProgress] = useState<ProgressData>({ phase: '', done: 0, total: 0 })
  const [result, setResult] = useState<CompleteData | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const [logs, setLogs] = useState<LogEntry[]>([])
  const eventSourceRef = useRef<EventSource | null>(null)
  const stateRef = useRef<PageState>('idle')
  const logEndRef = useRef<HTMLDivElement>(null)

  function setPageState(s: PageState) {
    stateRef.current = s
    setState(s)
  }

  // Auto-scroll logs to bottom
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  useEffect(() => {
    return () => {
      eventSourceRef.current?.close()
    }
  }, [])

  useEffect(() => {
    const urlPath = searchParams.get('path')
    if (urlPath) setPath(urlPath)
  }, [searchParams])

  // Check for active/completed runs on mount so navigating away doesn't lose state
  useEffect(() => {
    fetch('/api/projects/runs')
      .then(r => r.json())
      .then((runs: Array<{ project: string; status: string; result?: CompleteData; error?: string }>) => {
        if (runs.length > 0) {
          const lastRun = runs[0]
          if (lastRun.status === 'running') {
            setPageState('running')
            setLogs([{ level: 'info', message: 'Reconnecting to active run...', timestamp: Date.now() }])
            connectSSE(lastRun.project)
          } else if (lastRun.status === 'complete' && lastRun.result) {
            setResult(lastRun.result)
            setPageState('complete')
          } else if (lastRun.status === 'error' && lastRun.error) {
            setErrorMsg(lastRun.error)
            setPageState('error')
          }
        }
      })
      .catch(() => {}) // Silently ignore if endpoint not available
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function reset() {
    eventSourceRef.current?.close()
    eventSourceRef.current = null
    setPageState('idle')
    setProgress({ phase: '', done: 0, total: 0 })
    setResult(null)
    setErrorMsg('')
    setLogs([])
  }

  async function startIndexing() {
    if (!path.trim()) return
    setPageState('starting')
    setErrorMsg('')
    setResult(null)
    setLogs([])

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

      setPageState('running')
      toast.success('Indexing started')
      connectSSE(projectName)
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
      setPageState('error')
    }
  }

  function connectSSE(projectName: string) {
    const es = new EventSource(`/api/projects/${encodeURIComponent(projectName)}/progress`)
    eventSourceRef.current = es

    es.addEventListener('progress', (e) => {
      const data: ProgressData = JSON.parse(e.data)
      setProgress(data)
    })

    es.addEventListener('log', (e) => {
      if (e instanceof MessageEvent && e.data) {
        const data = JSON.parse(e.data)
        setLogs(prev => [...prev, { level: data.level, message: data.message, timestamp: Date.now() }])
      }
    })

    es.addEventListener('complete', (e) => {
      const data: CompleteData = JSON.parse(e.data)
      setResult(data)
      setLogs(prev => [...prev, { level: 'info', message: 'Indexing complete!', timestamp: Date.now() }])
      setPageState('complete')
      es.close()
    })

    // Pipeline errors from the backend (named "pipeline_error" to avoid
    // collision with SSE's built-in "error" event).
    es.addEventListener('pipeline_error', (e) => {
      if (e instanceof MessageEvent && e.data) {
        const data = JSON.parse(e.data)
        const msg = data.message || 'Unknown pipeline error'
        setErrorMsg(msg)
        toast.error(msg)
        setLogs(prev => [...prev, { level: 'error', message: msg, timestamp: Date.now() }])
      }
      setPageState('error')
      es.close()
    })

    // SSE connection-level errors (network failures, stream dropped).
    es.onerror = () => {
      if (stateRef.current === 'running') {
        setErrorMsg('Connection to progress stream lost')
        toast.error('Connection to progress stream lost')
        setPageState('error')
      }
      es.close()
    }
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
                placeholder="/projects/my-project"
                value={path}
                onChange={(e) => setPath(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Path inside the container. Projects are mounted at <code className="text-xs bg-muted px-1 rounded">/projects/</code>
              </p>
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

      {(state === 'running' || state === 'complete' || state === 'error') && (
        <div className="space-y-4 max-w-2xl">
          {state === 'running' && (
            <Card className="bg-card border-border">
              <CardHeader>
                <CardTitle className="text-base">Indexing in Progress</CardTitle>
              </CardHeader>
              <CardContent>
                <ProgressBar phase={progress.phase} done={progress.done} total={progress.total} />
              </CardContent>
            </Card>
          )}

          {state === 'complete' && result && (
            <Card className="bg-card border-border">
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
                {result.errors > 0 && result.error_messages && result.error_messages.length > 0 && (
                  <div className="border-t border-border pt-3">
                    <button
                      onClick={() => setErrorsExpanded(!errorsExpanded)}
                      className="flex items-center gap-2 text-sm text-red-400 hover:text-red-300 transition-colors w-full text-left"
                    >
                      <span className={`transition-transform ${errorsExpanded ? 'rotate-90' : ''}`}>&#9654;</span>
                      <span>{result.error_messages.length} error{result.error_messages.length !== 1 ? 's' : ''} — click to {errorsExpanded ? 'collapse' : 'expand'}</span>
                    </button>
                    {errorsExpanded && (
                      <div className="mt-2 bg-muted/50 rounded-md p-3 max-h-48 overflow-y-auto font-mono text-xs space-y-1">
                        {result.error_messages.map((msg, i) => (
                          <div key={i} className="flex gap-2">
                            <span className="text-red-400 shrink-0">&#10007;</span>
                            <span className="text-red-400">{msg}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
                <Button variant="secondary" onClick={reset}>Index Another</Button>
              </CardContent>
            </Card>
          )}

          {state === 'error' && (
            <Card className="bg-card border-destructive/30">
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

          {/* Pipeline Log */}
          {logs.length > 0 && (
            <Card className="bg-card border-border">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">Pipeline Log</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="bg-muted/50 rounded-md p-3 max-h-64 overflow-y-auto font-mono text-xs space-y-1">
                  {logs.map((entry, i) => (
                    <div key={i} className="flex gap-2">
                      <span className={
                        entry.level === 'error' ? 'text-red-400 shrink-0' :
                        entry.level === 'warn' ? 'text-yellow-400 shrink-0' :
                        'text-muted-foreground shrink-0'
                      }>
                        {entry.level === 'error' ? '✗' : entry.level === 'warn' ? '⚠' : '▸'}
                      </span>
                      <span className={
                        entry.level === 'error' ? 'text-red-400' :
                        entry.level === 'warn' ? 'text-yellow-400' :
                        'text-foreground'
                      }>
                        {entry.message}
                      </span>
                    </div>
                  ))}
                  <div ref={logEndRef} />
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      )}
    </div>
  )
}
