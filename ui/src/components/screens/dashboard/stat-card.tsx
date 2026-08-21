import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'

interface StatCardProps {
  label: string
  value: string
  caption?: string
  accent?: string
}

export function StatCard({ label, value, caption, accent }: StatCardProps) {
  return (
    <Card className="border-border/80 bg-card shadow-sm">
      <CardContent className="flex min-h-24 flex-col justify-center gap-1 px-4 py-3.5">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{label}</span>
        <span className={cn('text-2xl font-bold leading-none tracking-tight tabular-nums', accent ?? 'text-foreground')}>{value}</span>
        {caption && <span className="mt-1 text-[10px] text-muted-foreground">{caption}</span>}
      </CardContent>
    </Card>
  )
}
