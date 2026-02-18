import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface ProjectCardProps {
  name: string
  path: string
  indexedAt: string
  fileCount: number
}

export function ProjectCard({ name, path, indexedAt, fileCount }: ProjectCardProps) {
  const timeAgo = getTimeAgo(indexedAt)
  return (
    <Card className="bg-zinc-900 border-zinc-800 hover:border-zinc-700 transition-colors">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-semibold">{name}</CardTitle>
          <Badge variant="secondary" className="text-xs">{fileCount} files</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-xs text-zinc-500 truncate mb-1" title={path}>{path}</p>
        <p className="text-xs text-zinc-400">Indexed {timeAgo}</p>
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
