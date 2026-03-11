import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Section, StatCard } from '@/components/Section'
import { apiFetch } from '@/lib/api'
import { cn } from '@/lib/utils'

interface Project {
  name: string
  path: string
  indexed_at: string
  file_count: number
}

interface HealthStatus {
  status: string
  memories_healthy: boolean
}

interface RunStatus {
  project: string
  status: string
  result?: {
    modules: number
    files: number
    atoms: number
    errors: number
  }
  error?: string
}

function getTimeAgo(dateStr: string): string {
  if (!dateStr) return 'never'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return 'unknown'
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}

export default function Dashboard() {
  const [projects, setProjects] = useState<Project[]>([])
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [runStatuses, setRunStatuses] = useState<Record<string, RunStatus>>({})
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()

  useEffect(() => {
    Promise.all([
      apiFetch<Project[]>('/projects'),
      apiFetch<HealthStatus>('/health'),
      apiFetch<RunStatus[]>('/projects/runs').catch(() => []),
    ]).then(([projData, healthData, runsData]) => {
      setProjects(projData)
      setHealth(healthData)
      const runMap: Record<string, RunStatus> = {}
      for (const run of runsData) {
        runMap[run.project] = run
      }
      setRunStatuses(runMap)
    }).catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  const runs = Object.values(runStatuses)
  const filesIndexed = projects.reduce((sum, project) => sum + project.file_count, 0)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold">Dashboard</h2>
          <p className="text-sm text-muted-foreground">
            {projects.length} project{projects.length !== 1 ? 's' : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={() => navigate('/index')}>Index New</Button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4 mb-6">
        <StatCard label="Projects" value={projects.length} />
        <StatCard label="Files" value={filesIndexed} />
        <StatCard label="Atoms" value={'\u2014'} />
        <StatCard
          label="Memories"
          value={health?.memories_healthy ? 'Healthy' : '\u2014'}
          status={health === null ? 'unknown' : health.memories_healthy ? 'ok' : 'error'}
        />
      </div>

      {loading ? (
        <p className="text-muted-foreground text-sm">Loading...</p>
      ) : projects.length === 0 ? (
        <Section>
          <div className="flex flex-col items-center gap-4 py-12 text-center">
            <div className="rounded-full bg-primary/10 p-4">
              <span className="text-3xl">{'\u25EB'}</span>
            </div>
            <div>
              <h3 className="text-lg font-semibold">No indexed projects yet</h3>
              <p className="mt-1 text-sm text-muted-foreground">Get started by indexing your first codebase</p>
            </div>
            <div className="flex flex-col gap-1 text-sm text-muted-foreground">
              <span>1. Configure your LLM provider in Settings</span>
              <span>2. Index your first project</span>
            </div>
            <Button onClick={() => navigate('/index')}>Index Your First Project</Button>
          </div>
        </Section>
      ) : (
        <>
          <Section title="Projects">
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="text-sm font-medium">Name</TableHead>
                    <TableHead className="text-sm font-medium hidden sm:table-cell">Path</TableHead>
                    <TableHead className="text-sm font-medium w-16">Files</TableHead>
                    <TableHead className="text-sm font-medium w-24">Indexed</TableHead>
                    <TableHead className="text-sm font-medium w-20 hidden sm:table-cell">Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {projects.map((p) => {
                    const run = runStatuses[p.name]
                    return (
                      <TableRow
                        key={p.name}
                        className="cursor-pointer hover:bg-muted/50"
                        onClick={() => navigate(`/projects/${encodeURIComponent(p.name)}`)}
                      >
                        <TableCell className="text-sm font-medium">{p.name}</TableCell>
                        <TableCell className="text-sm text-muted-foreground truncate max-w-[200px] hidden sm:table-cell" title={p.path}>{p.path}</TableCell>
                        <TableCell className="text-sm">{p.file_count}</TableCell>
                        <TableCell className="text-sm text-muted-foreground">{getTimeAgo(p.indexed_at)}</TableCell>
                        <TableCell className="hidden sm:table-cell">
                          {run?.status === 'running' && <Badge variant="secondary" className="text-xs">Running</Badge>}
                          {run?.status === 'error' && <Badge variant="destructive" className="text-xs">Error</Badge>}
                          {(!run || run.status === 'complete') && <Badge variant="default" className="text-xs">Indexed</Badge>}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          </Section>

          {runs.length > 0 && (
            <Section title="Recent Activity" className="mt-6">
              <div className="space-y-2">
                {runs.map((run, i) => (
                  <div key={i} className="flex items-center justify-between rounded-md border border-border/30 px-4 py-2.5">
                    <div className="flex items-center gap-3">
                      <span className={cn(
                        'h-2 w-2 rounded-full',
                        run.status === 'complete' ? 'bg-emerald-500' :
                        run.status === 'error' ? 'bg-red-500' : 'bg-yellow-500'
                      )} />
                      <span className="font-medium">{run.project}</span>
                      <span className="text-sm text-muted-foreground">
                        {run.status === 'complete' ? `${run.result?.atoms ?? 0} atoms` : run.error ?? run.status}
                      </span>
                    </div>
                    <span className="text-sm text-muted-foreground">
                      {run.status === 'running' ? 'In progress' : 'Latest known state'}
                    </span>
                  </div>
                ))}
              </div>
            </Section>
          )}
        </>
      )}
    </div>
  )
}
