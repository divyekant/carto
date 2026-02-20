import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { SourcesEditor } from '@/components/SourcesEditor'

interface Project {
  name: string
  path: string
  indexed_at: string
  file_count: number
}

export default function ProjectDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const [project, setProject] = useState<Project | null>(null)
  const [loading, setLoading] = useState(true)

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
        <Card className="bg-card border-border">
          <CardHeader>
            <CardTitle className="text-base">Sources</CardTitle>
          </CardHeader>
          <CardContent>
            <SourcesEditor projectName={project.name} />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
