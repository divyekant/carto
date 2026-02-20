import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { FolderPicker } from '@/components/FolderPicker'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
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
  const [inputMode, setInputMode] = useState<'local' | 'git'>('local')
  const [path, setPath] = useState('')
  const [gitUrl, setGitUrl] = useState('')
  const [branch, setBranch] = useState('')
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

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  useEffect(() => {
    return () => { eventSourceRef.current?.close() }
  }, [])

  useEffect(() => {
    const urlPath = searchParams.get('path')
    if (urlPath) setPath(urlPath)
  }, [searchParams])

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
      .catch(() => {})
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
    if (inputMode === 'local' && !path.trim()) return
    if (inputMode === 'git' && !gitUrl.trim()) return
    setPageState('starting')
    setErrorMsg('')
    setResult(null)
    setLogs([])

    try {
      const body: Record<string, unknown> = { incremental }
      if (inputMode === 'local') {
        body.path = path.trim()
      } else {
        body.url = gitUrl.trim()
        if (branch.trim()) body.branch = branch.trim()
      }
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

  return (
    <div>
      <h2 className="text-lg font-semibold mb-3">Index Project</h2>

      {state === 'idle' && (
        <div className="space-y-3">
          {/* Tab toggle */}
          <div className="flex gap-1 p-1 bg-muted rounded-lg w-fit">
            <button
              className={`px-3 py-1 text-xs rounded-md transition-colors ${
                inputMode === 'local' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              }`}
              onClick={() => setInputMode('local')}
            >
              Local Path
            </button>
            <button
              className={`px-3 py-1 text-xs rounded-md transition-colors ${
                inputMode === 'git' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              }`}
              onClick={() => setInputMode('git')}
            >
              Git URL
            </button>
          </div>

          {/* Single-row form */}
          <div className="flex items-end gap-3 flex-wrap">
            {inputMode === 'local' && (
              <div className="flex-1 min-w-[200px]">
                <Label className="text-xs mb-1 block">Project Path</Label>
                <FolderPicker value={path} onChange={setPath} />
              </div>
            )}

            {inputMode === 'git' && (
              <>
                <div className="flex-1 min-w-[200px]">
                  <Label className="text-xs mb-1 block">Repository URL</Label>
                  <Input
                    placeholder="https://github.com/user/repo"
                    value={gitUrl}
                    onChange={(e) => setGitUrl(e.target.value)}
                  />
                </div>
                <div className="w-32">
                  <Label className="text-xs mb-1 block">Branch</Label>
                  <Input
                    placeholder="main"
                    value={branch}
                    onChange={(e) => setBranch(e.target.value)}
                  />
                </div>
              </>
            )}

            <div className="w-36">
              <Label className="text-xs mb-1 block">Module (opt.)</Label>
              <Input
                placeholder="e.g. go"
                value={module}
                onChange={(e) => setModule(e.target.value)}
              />
            </div>

            <div className="flex items-center gap-2 pb-1">
              <Switch checked={incremental} onCheckedChange={setIncremental} id="incremental" />
              <Label htmlFor="incremental" className="text-xs">Incremental</Label>
            </div>

            <Button size="sm" onClick={startIndexing} disabled={inputMode === 'local' ? !path.trim() : !gitUrl.trim()}>
              Start
            </Button>
          </div>
        </div>
      )}

      {state === 'starting' && (
        <p className="text-muted-foreground text-sm">Starting indexing...</p>
      )}

      {(state === 'running' || state === 'complete' || state === 'error') && (
        <div className="space-y-3">
          <div className="flex gap-3">
            {/* Left: progress / result */}
            <div className="flex-1 min-w-0">
              {state === 'running' && (
                <ProgressBar phase={progress.phase} done={progress.done} total={progress.total} />
              )}

              {state === 'complete' && result && (
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Badge variant="default" className="text-xs">Done</Badge>
                    <span className="text-xs text-muted-foreground">Elapsed: {result.elapsed}</span>
                  </div>
                  <div className="grid grid-cols-4 gap-2 text-xs">
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
                  <Button variant="secondary" size="sm" onClick={reset}>Index Another</Button>
                </div>
              )}

              {state === 'error' && (
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Badge variant="destructive" className="text-xs">Failed</Badge>
                  </div>
                  <p className="text-xs text-red-400">{errorMsg}</p>
                  <Button variant="secondary" size="sm" onClick={reset}>Try Again</Button>
                </div>
              )}
            </div>

            {/* Right: log */}
            {logs.length > 0 && (
              <div className="flex-1 min-w-0 bg-muted/50 rounded-md p-2 max-h-80 overflow-y-auto font-mono text-xs space-y-1">
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
            )}
          </div>
        </div>
      )}
    </div>
  )
}
