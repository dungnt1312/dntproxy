import { Edit3, Eye, EyeOff, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { CopyButton } from './copy-button'
import { PermissionSummary } from './permission-summary'
import type { ApiKey } from './types'

interface KeysMobileProps {
  keys: ApiKey[]
  revealedKeys: Set<string>
  onToggleReveal: (id: string) => void
  onDelete: (key: ApiKey) => void
  onEdit: (key: ApiKey) => void
  formatDate: (dateStr: string) => string
  maskKey: (key: string) => string
}

export function KeysMobile({
  keys,
  revealedKeys,
  onToggleReveal,
  onDelete,
  onEdit,
  formatDate,
  maskKey,
}: KeysMobileProps) {
  return (
    <div className="space-y-3 p-4 md:hidden">
      {keys.map((apiKey) => {
        const isRevealed = revealedKeys.has(apiKey.id)
        return (
          <div key={apiKey.id} className="rounded-lg border bg-card p-4 space-y-3">
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-2 min-w-0">
                <div className={cn(
                  'size-2 rounded-full shrink-0 mt-1.5',
                  apiKey.isActive ? 'bg-emerald-500' : 'bg-gray-400'
                )} />
                <div className="min-w-0">
                  <p className="font-medium truncate">{apiKey.name}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{formatDate(apiKey.createdAt)}</p>
                </div>
              </div>
              <Badge
                variant="outline"
                className={cn(
                  'shrink-0 font-medium',
                  apiKey.isActive
                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                    : 'bg-gray-500/10 text-gray-500 dark:text-gray-400 border-gray-500/20'
                )}
              >
                {apiKey.isActive ? 'Active' : 'Inactive'}
              </Badge>
            </div>

            <div className="flex items-center gap-1.5">
              <code className="min-w-0 flex-1 truncate rounded bg-muted px-2 py-1.5 font-mono text-xs select-all">
                {isRevealed ? apiKey.key : maskKey(apiKey.key)}
              </code>
              <Button
                variant="ghost"
                size="icon"
                className="size-7 shrink-0"
                onClick={() => onToggleReveal(apiKey.id)}
              >
                {isRevealed ? <EyeOff className="size-3" /> : <Eye className="size-3" />}
              </Button>
              <CopyButton text={apiKey.key} label="Key" />
            </div>

            <div className="flex items-center justify-between gap-2 text-xs">
              <span className="text-muted-foreground">Access</span>
              <PermissionSummary apiKey={apiKey} />
            </div>

            <div className="flex items-center justify-end gap-1 pt-1 border-t">
              <Button
                variant="ghost"
                size="sm"
                className="h-7 text-xs"
                onClick={() => onEdit(apiKey)}
              >
                <Edit3 className="size-3" />
                Edit
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 text-xs text-destructive hover:text-destructive hover:bg-destructive/10"
                onClick={() => onDelete(apiKey)}
              >
                <Trash2 className="size-3" />
                Delete
              </Button>
            </div>
          </div>
        )
      })}
    </div>
  )
}
