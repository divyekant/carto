import { useState } from 'react'
import { cn } from '@/lib/utils'

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
      <span className={cn(
        'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium shrink-0',
        score > 0.8 ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400' :
        score > 0.5 ? 'bg-yellow-500/15 text-yellow-700 dark:text-yellow-400' :
        'bg-muted text-muted-foreground'
      )}>
        {score.toFixed(2)}
      </span>
      <pre className="text-xs text-muted-foreground flex-1 whitespace-pre-wrap font-mono leading-relaxed">{preview}</pre>
    </div>
  )
}
