import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ProjectCard } from '@/components/ProjectCard'

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

export default function Dashboard() {
  const [projects, setProjects] = useState<Project[]>([])
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()

  useEffect(() => {
    Promise.all([
      fetch('/api/projects').then(r => r.json()),
      fetch('/api/health').then(r => r.json()),
    ]).then(([projData, healthData]) => {
      setProjects(projData.projects || [])
      setHealth(healthData)
    }).catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  return (
    <div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold">Dashboard</h2>
          <p className="text-sm text-zinc-400 mt-1">
            {projects.length} indexed project{projects.length !== 1 ? 's' : ''}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {health && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-zinc-500">Memories</span>
              <Badge variant={health.memories_healthy ? 'default' : 'destructive'} className="text-xs">
                {health.memories_healthy ? 'Connected' : 'Offline'}
              </Badge>
            </div>
          )}
          <Button onClick={() => navigate('/index')}>Index Project</Button>
        </div>
      </div>

      {loading ? (
        <p className="text-zinc-400">Loading...</p>
      ) : projects.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-zinc-400 mb-4">No indexed projects yet.</p>
          <Button onClick={() => navigate('/index')}>Index Your First Project</Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map((p) => (
            <ProjectCard key={p.name} name={p.name} path={p.path} indexedAt={p.indexed_at} fileCount={p.file_count} />
          ))}
        </div>
      )}
    </div>
  )
}
