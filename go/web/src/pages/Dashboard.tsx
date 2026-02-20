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
      fetch('/api/projects').then(r => r.json()),
      fetch('/api/health').then(r => r.json()),
      fetch('/api/projects/runs').then(r => r.json()).catch(() => []),
    ]).then(([projData, healthData, runsData]) => {
      setProjects(Array.isArray(projData) ? projData : projData.projects || [])
      setHealth(healthData)
      const runMap: Record<string, RunStatus> = {}
      for (const run of (runsData as RunStatus[])) {
        runMap[run.project] = run
      }
      setRunStatuses(runMap)
    }).catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <div>
          <h2 className="text-lg font-semibold">Dashboard</h2>
          <p className="text-xs text-muted-foreground">
            {projects.length} project{projects.length !== 1 ? 's' : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {health && (
            <Badge variant={health.memories_healthy ? 'default' : 'destructive'} className="text-xs">
              {health.memories_healthy ? 'Memories \u2713' : 'Memories \u2717'}
            </Badge>
          )}
          <Button size="sm" onClick={() => navigate('/index')}>Index New</Button>
        </div>
      </div>

      {loading ? (
        <p className="text-muted-foreground text-sm">Loading...</p>
      ) : projects.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-muted-foreground mb-4 text-sm">No indexed projects yet.</p>
          <Button size="sm" onClick={() => navigate('/index')}>Index Your First Project</Button>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Name</TableHead>
                <TableHead className="text-xs hidden sm:table-cell">Path</TableHead>
                <TableHead className="text-xs w-16">Files</TableHead>
                <TableHead className="text-xs w-24">Indexed</TableHead>
                <TableHead className="text-xs w-20 hidden sm:table-cell">Status</TableHead>
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
                    <TableCell className="text-xs text-muted-foreground truncate max-w-[200px] hidden sm:table-cell" title={p.path}>{p.path}</TableCell>
                    <TableCell className="text-xs">{p.file_count}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{getTimeAgo(p.indexed_at)}</TableCell>
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
      )}
    </div>
  )
}
