import { useEffect, useState } from 'react'
import { Key, Copy, CopyCheck } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'
import { goApi } from '@/lib/go-api'
import { cn } from '@/lib/utils'
import type { Tenant } from './types'

// A minimal key shape (avoids importing the full ApiKey type + its deps).
interface TenantKey {
  id: string
  name: string
  key: string
  isActive: boolean
  dashboardAccess: boolean
  createdAt?: string
}

interface TenantKeysDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  tenant: Tenant | null
}

export function TenantKeysDialog({
  open,
  onOpenChange,
  tenant,
}: TenantKeysDialogProps) {
  const [keys, setKeys] = useState<TenantKey[]>([])
  const [loading, setLoading] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  useEffect(() => {
    if (!open || !tenant) return
    let cancelled = false
    const load = async () => {
      try {
        setLoading(true)
        const all = await goApi.getKeys()
        if (cancelled) return
        // Filter to only keys pinned to this tenant (slug === tenantId).
        const owned = all.filter((k) => k.tenantId === tenant.slug)
        setKeys(owned)
      } catch {
        if (!cancelled) toast.error('Failed to load tenant keys')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [open, tenant])

  const handleCopy = (k: TenantKey) => {
    navigator.clipboard
      .writeText(k.key)
      .then(() => {
        setCopiedId(k.id)
        toast.success(`Key "${k.name}" copied`)
        setTimeout(() => setCopiedId((prev) => (prev === k.id ? null : prev)), 2000)
      })
      .catch(() => toast.error('Failed to copy'))
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            Keys for {tenant?.name ?? 'Tenant'}
            <code className="ml-2 text-xs text-muted-foreground font-mono">
              {tenant?.slug}
            </code>
          </DialogTitle>
          <DialogDescription>
            All API keys pinned to this tenant. Copy a key to deliver it to the
            tenant out-of-band.
          </DialogDescription>
        </DialogHeader>

        <div className="py-2 min-h-[120px]">
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 2 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3">
                  <Skeleton className="h-5 w-32" />
                  <Skeleton className="h-5 flex-1" />
                  <Skeleton className="size-8" />
                </div>
              ))}
            </div>
          ) : keys.length === 0 ? (
            <div className="text-center py-10 text-muted-foreground">
              <Key className="size-8 mx-auto mb-3 opacity-50" />
              <p className="text-sm">No keys for this tenant yet.</p>
              <p className="text-xs mt-1">
                Use the “Generate Key” action to create one.
              </p>
            </div>
          ) : (
            <div className="divide-y rounded-md border max-h-[50vh] overflow-y-auto">
              {keys.map((k) => (
                <div
                  key={k.id}
                  className="flex items-center gap-3 p-3 hover:bg-muted/30"
                >
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium truncate">{k.name}</span>
                      <Badge
                        variant="outline"
                        className={cn(
                          'text-[10px] px-1.5',
                          k.isActive
                            ? 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20'
                            : 'bg-gray-500/10 text-gray-500 border-gray-500/20',
                        )}
                      >
                        {k.isActive ? 'Active' : 'Inactive'}
                      </Badge>
                      {k.dashboardAccess && (
                        <Badge
                          variant="outline"
                          className="text-[10px] px-1.5 bg-blue-500/10 text-blue-600 border-blue-500/20"
                        >
                          Dashboard
                        </Badge>
                      )}
                    </div>
                    <code className="text-xs text-muted-foreground font-mono break-all">
                      {k.key}
                    </code>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8 shrink-0"
                    onClick={() => handleCopy(k)}
                    title="Copy key"
                  >
                    {copiedId === k.id ? (
                      <CopyCheck className="size-3.5 text-emerald-500" />
                    ) : (
                      <Copy className="size-3.5" />
                    )}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
