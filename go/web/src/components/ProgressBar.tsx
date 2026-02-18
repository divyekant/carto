import { Progress } from '@/components/ui/progress'

interface ProgressBarProps {
  phase: string
  done: number
  total: number
}

export function ProgressBar({ phase, done, total }: ProgressBarProps) {
  const percent = total > 0 ? Math.round((done / total) * 100) : 0
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="text-zinc-300 capitalize">{phase}</span>
        <span className="text-zinc-400">{done}/{total}</span>
      </div>
      <Progress value={percent} className="h-2" />
    </div>
  )
}
