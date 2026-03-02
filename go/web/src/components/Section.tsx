import { cn } from '@/lib/utils';

interface SectionProps {
  title?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function Section({ title, action, children, className }: SectionProps) {
  return (
    <div className={cn('rounded-lg border border-border/50 bg-card p-5', className)}>
      {(title || action) && (
        <div className="mb-4 flex items-center justify-between">
          {title && <h3 className="text-lg font-semibold">{title}</h3>}
          {action}
        </div>
      )}
      {children}
    </div>
  );
}

interface StatCardProps {
  label: string;
  value: string | number;
  icon?: React.ReactNode;
  status?: 'ok' | 'error' | 'unknown';
  detail?: string;
}

export function StatCard({ label, value, icon, status, detail }: StatCardProps) {
  return (
    <div className="rounded-lg border border-border/50 bg-card p-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        {icon}
        {label}
      </div>
      <div className="mt-1 text-2xl font-bold">{value}</div>
      {detail && <div className="mt-0.5 text-sm text-muted-foreground">{detail}</div>}
      {status && (
        <div className="mt-1 flex items-center gap-1.5 text-sm">
          <span className={cn(
            'inline-block h-2 w-2 rounded-full',
            status === 'ok' && 'bg-emerald-500',
            status === 'error' && 'bg-red-500',
            status === 'unknown' && 'bg-muted-foreground/50',
          )} />
          {status === 'ok' ? 'Connected' : status === 'error' ? 'Disconnected' : 'Unknown'}
        </div>
      )}
    </div>
  );
}
