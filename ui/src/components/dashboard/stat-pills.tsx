import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Activity, Clock, Coins } from 'lucide-react'

interface StatPillsProps {
  totalRequests: number
  avgLatencyMs: number
  costTotal: number
  successRate: number
  loading: boolean
}

export function StatPills({ totalRequests, avgLatencyMs, costTotal, successRate, loading }: StatPillsProps) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        {[1, 2, 3, 4].map(i => (
          <Card key={i}><CardContent className="p-3"><Skeleton className="h-10 w-full" /></CardContent></Card>
        ))}
      </div>
    )
  }

  const items = [
    {
      icon: <Activity className="size-3.5" />,
      value: totalRequests.toLocaleString(),
      label: 'requests',
    },
    {
      icon: <CheckCircle2 className="size-3.5" />,
      value: `${successRate.toFixed(0)}%`,
      label: 'success',
      valueColor: successRate >= 95 ? 'text-emerald-600 dark:text-emerald-400' : successRate >= 80 ? 'text-amber-600 dark:text-amber-400' : 'text-red-600 dark:text-red-400',
    },
    {
      icon: <Clock className="size-3.5" />,
      value: avgLatencyMs < 1000 ? `${Math.round(avgLatencyMs)}ms` : `${(avgLatencyMs / 1000).toFixed(1)}s`,
      label: 'avg latency',
      valueColor: avgLatencyMs < 500 ? 'text-emerald-600 dark:text-emerald-400' : avgLatencyMs < 1500 ? 'text-amber-600 dark:text-amber-400' : 'text-red-600 dark:text-red-400',
    },
    {
      icon: <Coins className="size-3.5" />,
      value: costTotal >= 1 ? `$${costTotal.toFixed(2)}` : costTotal > 0 ? `$${costTotal.toFixed(4)}` : '$0',
      label: 'cost (24h)',
    },
  ]

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
      {items.map(item => (
        <Card key={item.label}>
          <CardContent className="p-3 flex items-center gap-2.5">
            <div className="size-8 rounded-lg bg-muted flex items-center justify-center shrink-0 text-muted-foreground">
              {item.icon}
            </div>
            <div className="min-w-0">
              <p className={`text-base font-bold tabular-nums leading-none ${item.valueColor || ''}`}>{item.value}</p>
              <p className="text-[10px] text-muted-foreground mt-0.5">{item.label}</p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function CheckCircle2(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}>
      <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><path d="m9 11 3 3L22 4" />
    </svg>
  )
}
