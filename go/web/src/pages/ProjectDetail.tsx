import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { SourcesEditor } from '@/components/SourcesEditor'
import { ProgressBar } from '@/components/ProgressBar'

interface Project {
  name: string
  path: string
  indexed_at: string
  file_count: number
}

type IndexState = 'idle' | 'starting' | 'running' | 'complete' | 'error'

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

export default function ProjectDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const [project, setProject] = useState<Project | null>(null)
  const [loading, setLoading] = useState(true)

  // Index card state
  const [indexState, setIndexState] = useState<IndexState>('idle')
  const [incremental, setIncremental] = useState(false)
  const [moduleFilter, setModuleFilter] = useState('')
  const [progress, setProgress] = useState<ProgressData>({ phase: '', done: 0, total: 0 })
  const [result, setResult] = useState<CompleteData | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [errorsExpanded, setErrorsExpanded] = useState(false)
  const eventSourceRef = useRef<EventSource | null>(null)
  const stateRef = useRef<IndexState>('idle')
  const logEndRef = useRef<HTMLDivElement>(null)

  function setPageState(s: IndexState) {
    stateRef.current = s
    setIndexState(s)
  }

  // Auto-scroll logs
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  // Cleanup SSE on unmount
  useEffect(() => {
    return () => { eventSourceRef.current?.close() }
  }, [])

  // Load project data
  useEffect(() => {
    fetch('/api/projects')
      .then(r => r.json())
      .then((data: Project[]) => {
        const projects = Array.isArray(data) ? data : (data as any).projects || []
        const found = projects.find((p: Project) => p.name === name)
        setProject(found || null)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [name])

  // Check for active runs on mount
  useEffect(() => {
    if (!name) return
    fetch('/api/projects/runs')
      .then(r => r.json())
      .then((runs: Array<{ project: string; status: string; result?: CompleteData; error?: string }>) => {
        const myRun = runs.find(r => r.project === name)
        if (!myRun) return
        if (myRun.status === 'running') {
          setPageState('running')
          setLogs([{ level: 'info', message: 'Reconnecting to active run...', timestamp: Date.now() }])
          connectSSE(myRun.project)
        } else if (myRun.status === 'complete' && myRun.result) {
          setResult(myRun.result)
          setPageState('complete')
        } else if (myRun.status === 'error' && myRun.error) {
          setErrorMsg(myRun.error)
          setPageState('error')
        }
      })
      .catch(() => {})
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [name])

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

    es.onerror = () => {
      if (stateRef.current === 'running') {
        setErrorMsg('Connection to progress stream lost')
        toast.error('Connection to progress stream lost')
        setPageState('error')
      }
      es.close()
    }
  }

  async function startIndex() {
    if (!project) return
    setPageState('starting')
    setErrorMsg('')
    setResult(null)
    setLogs([])

    try {
      const body: Record<string, unknown> = {
        path: project.path,
        project: project.name,
        incremental,
      }
      if (moduleFilter.trim()) body.module = moduleFilter.trim()

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
      setPageState('running')
      toast.success('Indexing started')
      connectSSE(data.project)
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
      setPageState('error')
    }
  }

  function resetIndex() {
    eventSourceRef.current?.close()
    eventSourceRef.current = null
    setPageState('idle')
    setProgress({ phase: '', done: 0, total: 0 })
    setResult(null)
    setErrorMsg('')
    setLogs([])
  }

  if (loading) {
    return (
      <div>
        <h2 className="text-2xl font-bold mb-6">Project</h2>
        <p className="text-muted-foreground">Loading...</p>
      </div>
    )
  }

  if (!project) {
    return (
      <div>
        <h2 className="text-2xl font-bold mb-6">Project Not Found</h2>
        <p className="text-muted-foreground mb-4">No indexed project named &quot;{name}&quot;.</p>
        <Button variant="secondary" onClick={() => navigate('/')}>Back to Dashboard</Button>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <button onClick={() => navigate('/')} className="text-muted-foreground hover:text-foreground">
          &larr;
        </button>
        <h2 className="text-2xl font-bold">{project.name}</h2>
        <Badge variant="secondary" className="text-xs">{project.file_count} files</Badge>
      </div>
      <p className="text-sm text-muted-foreground mb-6 truncate" title={project.path}>{project.path}</p>

      <div className="space-y-6 max-w-2xl">
        {/* Sources Card */}
        <Card className="bg-card border-border">
          <CardHeader>
            <CardTitle className="text-base">Sources</CardTitle>
          </CardHeader>
          <CardContent>
            <SourcesEditor projectName={project.name} />
          </CardContent>
        </Card>

        {/* Index Card */}
        <Card className="bg-card border-border">
          <CardHeader>
            <CardTitle className="text-base">Index</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {indexState === 'idle' && (
              <>
                <div className="flex items-center gap-4">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="incremental"
                      checked={incremental}
                      onChange={e => setIncremental(e.target.checked)}
                      className="rounded border-border"
                    />
                    <Label htmlFor="incremental" className="text-sm">Incremental</Label>
                  </div>
                  <div className="flex-1 max-w-xs">
                    <Input
                      placeholder="Module filter (optional)"
                      value={moduleFilter}
                      onChange={e => setModuleFilter(e.target.value)}
                    />
                  </div>
                </div>
                <Button onClick={startIndex}>Index Now</Button>
              </>
            )}

            {indexState === 'starting' && (
              <p className="text-muted-foreground text-sm">Starting indexing...</p>
            )}

            {indexState === 'running' && (
              <ProgressBar phase={progress.phase} done={progress.done} total={progress.total} />
            )}

            {indexState === 'complete' && result && (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <Badge variant="default" className="text-xs">Done</Badge>
                  <span className="text-xs text-muted-foreground">Elapsed: {result.elapsed}</span>
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
                  <div>
                    <span className="text-muted-foreground">Modules</span>
                    <p className="font-medium">{result.modules}</p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Files</span>
                    <p className="font-medium">{result.files}</p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Atoms</span>
                    <p className="font-medium">{result.atoms}</p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Errors</span>
                    <p className={result.errors > 0 ? 'text-red-400 font-medium' : 'font-medium'}>
                      {result.errors}
                    </p>
                  </div>
                </div>
                {result.errors > 0 && result.error_messages && result.error_messages.length > 0 && (
                  <div className="border-t border-border pt-3">
                    <button
                      onClick={() => setErrorsExpanded(!errorsExpanded)}
                      className="flex items-center gap-2 text-sm text-red-400 hover:text-red-300 transition-colors w-full text-left"
                    >
                      <span className={`transition-transform ${errorsExpanded ? 'rotate-90' : ''}`}>&#9654;</span>
                      <span>{result.error_messages.length} error{result.error_messages.length !== 1 ? 's' : ''}</span>
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
                <Button variant="secondary" size="sm" onClick={resetIndex}>Index Again</Button>
              </div>
            )}

            {indexState === 'error' && (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <Badge variant="destructive" className="text-xs">Failed</Badge>
                </div>
                <p className="text-sm text-red-400">{errorMsg}</p>
                <Button variant="secondary" size="sm" onClick={resetIndex}>Try Again</Button>
              </div>
            )}

            {/* Pipeline Log */}
            {logs.length > 0 && (
              <div className="border-t border-border pt-3">
                <p className="text-xs font-medium text-muted-foreground mb-2">Pipeline Log</p>
                <div className="bg-muted/50 rounded-md p-3 max-h-48 overflow-y-auto font-mono text-xs space-y-1">
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
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
