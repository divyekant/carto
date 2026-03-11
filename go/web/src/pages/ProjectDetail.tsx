import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { SourcesEditor } from '@/components/SourcesEditor'
import { ProgressBar } from '@/components/ProgressBar'
import { apiFetch } from '@/lib/api'
import { connectSSE, type SSEConnection } from '@/lib/sse'

interface Project {
  name: string
  path: string
  indexed_at: string
  file_count: number
}

type IndexState = 'idle' | 'starting' | 'running' | 'complete' | 'error' | 'stopped'

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

  const [indexState, setIndexState] = useState<IndexState>('idle')
  const [incremental, setIncremental] = useState(false)
  const [moduleFilter, setModuleFilter] = useState('')
  const [stopping, setStopping] = useState(false)
  const [progress, setProgress] = useState<ProgressData>({ phase: '', done: 0, total: 0 })
  const [result, setResult] = useState<CompleteData | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [errorsExpanded, setErrorsExpanded] = useState(false)
  const streamRef = useRef<SSEConnection | null>(null)
  const terminalEventRef = useRef(false)
  const stateRef = useRef<IndexState>('idle')
  const logEndRef = useRef<HTMLDivElement>(null)

  function setPageState(s: IndexState) {
    stateRef.current = s
    setIndexState(s)
  }

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  useEffect(() => {
    return () => { streamRef.current?.close() }
  }, [])

  useEffect(() => {
    apiFetch<Project[]>('/projects')
      .then((projects) => {
        const found = projects.find((project) => project.name === name)
        setProject(found || null)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [name])

  useEffect(() => {
    if (!name) return
    apiFetch<Array<{ project: string; status: string; result?: CompleteData; error?: string }>>('/projects/runs')
      .then((runs) => {
        const myRun = runs.find(r => r.project === name)
        if (!myRun) return
        if (myRun.status === 'running') {
          setPageState('running')
          setLogs([{ level: 'info', message: 'Reconnecting to active run...', timestamp: Date.now() }])
          openProgressStream(myRun.project)
        } else if (myRun.status === 'complete' && myRun.result) {
          setResult(myRun.result)
          setPageState('complete')
        } else if (myRun.status === 'error' && myRun.error) {
          setErrorMsg(myRun.error)
          setPageState('error')
        } else if (myRun.status === 'stopped') {
          setPageState('stopped')
        }
      })
      .catch(() => {})
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [name])

  function openProgressStream(projectName: string) {
    streamRef.current?.close()
    terminalEventRef.current = false

    const stream = connectSSE(`/projects/${encodeURIComponent(projectName)}/progress`, {
      progress: (payload) => {
        const data: ProgressData = JSON.parse(payload)
        setProgress(data)
      },
      log: (payload) => {
        const data = JSON.parse(payload) as { level: string; message: string }
        setLogs(prev => [...prev, { level: data.level, message: data.message, timestamp: Date.now() }])
      },
      complete: (payload) => {
        terminalEventRef.current = true
        const data: CompleteData = JSON.parse(payload)
        setResult(data)
        setLogs(prev => [...prev, { level: 'info', message: 'Indexing complete!', timestamp: Date.now() }])
        setPageState('complete')
        stream.close()
      },
      pipeline_error: (payload) => {
        terminalEventRef.current = true
        const data = JSON.parse(payload) as { message?: string }
        const msg = data.message || 'Unknown pipeline error'
        setErrorMsg(msg)
        toast.error(msg)
        setLogs(prev => [...prev, { level: 'error', message: msg, timestamp: Date.now() }])
        setPageState('error')
        stream.close()
      },
      stopped: () => {
        terminalEventRef.current = true
        setLogs(prev => [...prev, { level: 'warn', message: 'Indexing stopped by user', timestamp: Date.now() }])
        setPageState('stopped')
        setStopping(false)
        toast('Indexing stopped')
        stream.close()
      },
    }, {
      onError: () => {
        if (stateRef.current === 'running') {
          terminalEventRef.current = true
          setErrorMsg('Connection to progress stream lost')
          toast.error('Connection to progress stream lost')
          setPageState('error')
        }
      },
    })

    stream.done.finally(() => {
      if (!terminalEventRef.current && stateRef.current === 'running') {
        setErrorMsg('Connection to progress stream lost')
        toast.error('Connection to progress stream lost')
        setPageState('error')
      }
    })

    streamRef.current = stream
  }

  async function stopIndex() {
    if (!name) return
    setStopping(true)
    try {
      await apiFetch(`/projects/${encodeURIComponent(name)}/stop`, { method: 'POST' })
    } catch {
      setStopping(false)
      toast.error('Failed to stop indexing')
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
      const trimmedModule = moduleFilter.trim()
      if (trimmedModule) body.module = trimmedModule

      const data = await apiFetch<{ project: string }>('/projects/index', {
        method: 'POST',
        body: JSON.stringify(body),
      })
      setPageState('running')
      toast.success('Indexing started')
      openProgressStream(data.project)
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
      setPageState('error')
    }
  }

  function resetIndex() {
    streamRef.current?.close()
    streamRef.current = null
    terminalEventRef.current = false
    setPageState('idle')
    setProgress({ phase: '', done: 0, total: 0 })
    setResult(null)
    setErrorMsg('')
    setLogs([])
    setStopping(false)
  }

  if (loading) {
    return (
      <div>
        <h2 className="text-lg font-semibold mb-3">Project</h2>
        <p className="text-muted-foreground text-sm">Loading...</p>
      </div>
    )
  }

  if (!project) {
    return (
      <div>
        <h2 className="text-lg font-semibold mb-3">Project Not Found</h2>
        <p className="text-muted-foreground mb-3 text-sm">No indexed project named &quot;{name}&quot;.</p>
        <Button variant="secondary" size="sm" onClick={() => navigate('/')}>Back to Dashboard</Button>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-3">
        <button onClick={() => navigate('/')} className="text-muted-foreground hover:text-foreground">
          &larr;
        </button>
        <h2 className="text-lg font-semibold">{project.name}</h2>
        <Badge variant="secondary" className="text-xs">{project.file_count} files</Badge>
      </div>
      <p className="text-xs text-muted-foreground mb-3 truncate" title={project.path}>{project.path}</p>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
        {/* Left column: Sources */}
        <div>
          <h3 className="text-sm font-medium mb-2 text-muted-foreground">Sources</h3>
          <SourcesEditor projectName={project.name} />
        </div>

        {/* Right column: Index controls */}
        <div>
          <h3 className="text-sm font-medium mb-2 text-muted-foreground">Index</h3>

          {indexState === 'idle' && (
            <div className="space-y-2">
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-2">
                  <Switch checked={incremental} onCheckedChange={setIncremental} id="proj-incremental" />
                  <Label htmlFor="proj-incremental" className="text-xs">Incremental</Label>
                </div>
                <div className="flex-1">
                  <Input
                    placeholder="Module filter (optional)"
                    value={moduleFilter}
                    onChange={e => setModuleFilter(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>
              </div>
              <Button size="sm" onClick={startIndex}>Index Now</Button>
            </div>
          )}

          {indexState === 'starting' && (
            <p className="text-muted-foreground text-xs">Starting indexing...</p>
          )}

          {indexState === 'running' && (
            <div className="space-y-2">
              <ProgressBar phase={progress.phase} done={progress.done} total={progress.total} />
              <Button
                variant="destructive"
                size="sm"
                onClick={stopIndex}
                disabled={stopping}
              >
                {stopping ? 'Stopping...' : 'Stop'}
              </Button>
            </div>
          )}

          {indexState === 'stopped' && (
            <div className="space-y-2">
              <Badge variant="secondary" className="text-xs">Stopped</Badge>
              <p className="text-xs text-muted-foreground">Indexing was stopped by user</p>
              <Button variant="secondary" size="sm" onClick={resetIndex}>Index Again</Button>
            </div>
          )}

          {indexState === 'complete' && result && (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Badge variant="default" className="text-xs">Done</Badge>
                <span className="text-xs text-muted-foreground">Elapsed: {result.elapsed}</span>
              </div>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
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
                  <p className={result.errors > 0 ? 'text-red-400 font-medium' : 'font-medium'}>{result.errors}</p>
                </div>
              </div>
              {result.errors > 0 && result.error_messages && result.error_messages.length > 0 && (
                <div className="border-t border-border pt-2">
                  <button
                    onClick={() => setErrorsExpanded(!errorsExpanded)}
                    className="flex items-center gap-2 text-xs text-red-400 hover:text-red-300 transition-colors w-full text-left"
                  >
                    <span className={`transition-transform ${errorsExpanded ? 'rotate-90' : ''}`}>&#9654;</span>
                    <span>{result.error_messages.length} error{result.error_messages.length !== 1 ? 's' : ''}</span>
                  </button>
                  {errorsExpanded && (
                    <div className="mt-1 bg-muted/50 rounded-md p-2 max-h-40 overflow-y-auto font-mono text-xs space-y-1">
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
            <div className="space-y-2">
              <Badge variant="destructive" className="text-xs">Failed</Badge>
              <p className="text-xs text-red-400">{errorMsg}</p>
              <Button variant="secondary" size="sm" onClick={resetIndex}>Try Again</Button>
            </div>
          )}

          {/* Pipeline Log */}
          {logs.length > 0 && (
            <div className="mt-3 border-t border-border pt-2">
              <p className="text-xs font-medium text-muted-foreground mb-1">Pipeline Log</p>
              <div className="bg-muted/50 rounded-md p-2 max-h-40 overflow-y-auto font-mono text-xs space-y-1">
                {logs.map((entry, i) => (
                  <div key={i} className="flex gap-2">
                    <span className={
                      entry.level === 'error' ? 'text-red-400 shrink-0' :
                      entry.level === 'warn' ? 'text-yellow-400 shrink-0' :
                      'text-muted-foreground shrink-0'
                    }>
                      {entry.level === 'error' ? '\u2717' : entry.level === 'warn' ? '\u26A0' : '\u25B8'}
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
        </div>
      </div>
    </div>
  )
}
