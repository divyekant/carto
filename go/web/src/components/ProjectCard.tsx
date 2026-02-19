import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface ProjectCardProps {
  name: string
  path: string
  indexedAt: string
  fileCount: number
  runStatus?: {
    status: string
    result?: { modules: number; files: number; atoms: number; errors: number }
    error?: string
  }
  onReindex?: () => void
}

export function ProjectCard({ name, path, indexedAt, fileCount, runStatus, onReindex }: ProjectCardProps) {
  const timeAgo = getTimeAgo(indexedAt)

  const statusBadge = runStatus ? (
    runStatus.status === 'running' ? (
      <Badge variant="secondary" className="text-xs">&#10227; Running</Badge>
    ) : runStatus.status === 'error' ? (
      <Badge variant="destructive" className="text-xs">&#10007; Error</Badge>
    ) : (
      <Badge variant="default" className="text-xs">&#10003; Indexed</Badge>
    )
  ) : null

  return (
    <Card className="bg-card hover:border-primary/30 transition-colors">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-semibold">{name}</CardTitle>
          <div className="flex items-center gap-2">
            {statusBadge}
            <Badge variant="secondary" className="text-xs">{fileCount} files</Badge>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-xs text-muted-foreground truncate mb-1" title={path}>{path}</p>
        <p className="text-xs text-muted-foreground mb-2">Indexed {timeAgo}</p>
        {runStatus?.status === 'error' && runStatus.error && (
          <p className="text-xs text-red-400 mb-2 truncate" title={runStatus.error}>
            {runStatus.error}
          </p>
        )}
        {onReindex && (
          <button
            onClick={onReindex}
            className="text-xs text-primary hover:text-primary/80 transition-colors"
          >
            Re-index &rarr;
          </button>
        )}
      </CardContent>
    </Card>
  )
}

function getTimeAgo(dateStr: string): string {
  const date = new Date(dateStr)
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
