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
    <div
      className="flex items-start gap-3 py-2 border-b border-border cursor-pointer hover:bg-muted/30"
      onClick={() => setExpanded(!expanded)}
    >
      <span className="text-xs font-mono text-muted-foreground shrink-0 w-6">{index}.</span>
      <span className="text-xs font-mono truncate max-w-[200px] shrink-0" title={source}>{source}</span>
      <div className="w-12 shrink-0 pt-1.5">
        <div className="h-1.5 bg-muted rounded-full overflow-hidden">
          <div className="h-full bg-primary rounded-full" style={{ width: `${Math.min(score * 100, 100)}%` }} />
        </div>
      </div>
      <pre className="text-xs text-muted-foreground flex-1 whitespace-pre-wrap font-mono leading-relaxed">{preview}</pre>
    </div>
  )
}
