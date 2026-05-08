import { AlertTriangle, ChevronRight } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { formatRelativeTime } from './helpers'

// ─── Errors Hint ─────────────────────────────────────────────────────────────

export interface RecentError {
  id: string
  createdAt: string
  errorMessage: string | null
  responseStatus: number
  provider: string | null
}

interface ErrorsHintProps {
  errors: RecentError[]
  onViewAll: () => void
}

export function ErrorsHint({ errors, onViewAll }: ErrorsHintProps) {
  return (
    <Card className="border-red-500/20 bg-red-500/5">
      <CardContent className="px-4 py-3">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-semibold text-red-400 flex items-center gap-1.5">
            <AlertTriangle className="size-3.5" />
            Recent Errors
            <Badge variant="destructive" className="text-[9px] px-1.5 py-0 ml-0.5">
              {errors.length}
            </Badge>
          </span>
          <button
            onClick={onViewAll}
            className="text-[10px] text-muted-foreground hover:text-foreground flex items-center gap-0.5 transition-colors"
          >
            View all <ChevronRight className="size-3" />
          </button>
        </div>
        <div className="space-y-1.5">
          {errors.slice(0, 3).map(err => (
            <div key={err.id} className="flex items-start gap-2 text-xs">
              <Badge variant="destructive" className="text-[9px] px-1.5 py-0 shrink-0 mt-0.5">
                {err.responseStatus || 'ERR'}
              </Badge>
              <span className="text-red-300/90 truncate flex-1">
                {err.errorMessage || 'Unknown error'}
              </span>
              <span className="text-muted-foreground shrink-0 whitespace-nowrap">
                {formatRelativeTime(err.createdAt)}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
