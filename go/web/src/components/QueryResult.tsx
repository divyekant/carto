import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useState } from 'react'

interface QueryResultProps {
  index: number
  source: string
  score: number
  text: string
}

export function QueryResult({ index, source, score, text }: QueryResultProps) {
  const [expanded, setExpanded] = useState(false)
  const preview = text.length > 200 && !expanded ? text.slice(0, 200) + '...' : text

  return (
    <Card className="bg-card cursor-pointer hover:border-primary/30 transition-colors" onClick={() => setExpanded(!expanded)}>
      <CardContent className="pt-4">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground font-mono">{index}.</span>
            <span className="text-sm text-foreground font-mono truncate max-w-md" title={source}>{source}</span>
          </div>
          <Badge variant="secondary" className="text-xs">{score.toFixed(3)}</Badge>
        </div>
        <pre className="text-xs text-muted-foreground whitespace-pre-wrap font-mono leading-relaxed">{preview}</pre>
      </CardContent>
    </Card>
  )
}
