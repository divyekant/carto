import { useState } from 'react'
import { cn } from '@/lib/utils'

interface QueryResultProps {
  index: number
  source: string
  score: number
  text: string
  matchType?: string
  confidence?: number
  metadata?: Record<string, string>
}

export function QueryResult({ index, source, score, text, matchType, confidence, metadata }: QueryResultProps) {
  const [expanded, setExpanded] = useState(false)
  const preview = text.length > 200 && !expanded ? text.slice(0, 200) + '...' : text

  const metadataKeys = ['module', 'language', 'kind']
  const metaTags = metadata
    ? metadataKeys.filter(k => metadata[k]).map(k => ({ key: k, value: metadata[k] }))
    : []

  return (
    <div
      className="flex items-start gap-3 py-2 border-b border-border cursor-pointer hover:bg-muted/30"
      onClick={() => setExpanded(!expanded)}
    >
      <span className="text-xs font-mono text-muted-foreground shrink-0 w-6">{index}.</span>
      <span className="text-xs font-mono truncate max-w-[200px] shrink-0" title={source}>{source}</span>
      <div className="flex items-center gap-1.5 shrink-0">
        <span className={cn(
          'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium',
          score > 0.8 ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400' :
          score > 0.5 ? 'bg-yellow-500/15 text-yellow-700 dark:text-yellow-400' :
          'bg-muted text-muted-foreground'
        )}>
          {score.toFixed(2)}
        </span>
        {confidence !== undefined && (
          <span className="inline-flex items-center rounded-md px-1.5 py-0.5 text-xs font-medium bg-sky-500/10 text-sky-700 dark:text-sky-400">
            c:{confidence.toFixed(2)}
          </span>
        )}
        {matchType === 'graph' && (
          <span className="inline-flex items-center rounded-md px-1.5 py-0.5 text-xs font-medium bg-violet-500/15 text-violet-700 dark:text-violet-400">
            graph
          </span>
        )}
      </div>
      <div className="flex-1 min-w-0">
        <pre className="text-xs text-muted-foreground whitespace-pre-wrap font-mono leading-relaxed">{preview}</pre>
        {metaTags.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-1.5">
            {metaTags.map(({ key, value }) => (
              <span
                key={key}
                className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-muted text-muted-foreground"
              >
                {key}:{value}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
