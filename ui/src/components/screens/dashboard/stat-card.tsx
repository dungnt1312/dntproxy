import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import { KPI_RANGE_LABELS, type KpiRange } from './helpers'

// ─── StatCard ────────────────────────────────────────────────────────────────

interface StatCardProps {
  label: string
  value: string
  accent?: string
  range: KpiRange
}

export function StatCard({ label, value, accent, range }: StatCardProps) {
  return (
    <Card>
      <CardContent className="px-4 py-3 flex flex-col gap-1">
        <span className="text-[10px] text-muted-foreground uppercase font-semibold tracking-wider">
          {label}
        </span>
        <span className={cn('text-2xl font-bold tabular-nums leading-none', accent ?? 'text-foreground')}>
          {value}
        </span>
        <span className="text-[9px] text-muted-foreground/60">
          {KPI_RANGE_LABELS[range]}
        </span>
      </CardContent>
    </Card>
  )
}
