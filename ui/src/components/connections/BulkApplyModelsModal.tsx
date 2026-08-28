import { useState } from 'react'
import { Layers, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../api'
import { ProviderLogo } from './ProviderLogo'
import { stripModelForConnection } from './BulkModelsModal'
import { getProviderLabel } from '@/lib/provider-registry'
import type { Connection } from '@/types/connections'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface BulkApplyModelsModalProps {
  /** Connection whose model list is being copied. */
  source: Connection
  /** Candidate connections (already excludes the source). */
  connections: Connection[]
  /** Model list in "prefix/model" form — prefixes are stripped per target. */
  models: string[]
  onApplied: () => void
  onClose: () => void
}

/**
 * Applies the source connection's current model list to other connections.
 * Each target keeps its own provider prefix handling (same semantics as
 * BulkModelsModal / EditModelsModal on save).
 */
export function BulkApplyModelsModal({ source, connections, models, onApplied, onClose }: BulkApplyModelsModalProps) {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [applying, setApplying] = useState(false)
  const [progress, setProgress] = useState('')

  const toggle = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const selectAll = () => setSelectedIds(new Set(connections.map((c) => c.id)))
  const clearAll = () => setSelectedIds(new Set())

  const handleApply = async () => {
    const targets = connections.filter((c) => selectedIds.has(c.id))
    if (targets.length === 0) return
    setApplying(true)
    let done = 0
    const errors: string[] = []
    for (const target of targets) {
      setProgress(`${done + 1}/${targets.length}`)
      try {
        await api.updateConnection(target.id, {
          supportedModels: models.map((m) => stripModelForConnection(m, target)),
          setModels: true,
        })
        done++
      } catch (e: any) {
        errors.push(`${target.name}: ${e.message || 'failed'}`)
      }
    }
    setApplying(false)
    if (errors.length > 0) {
      toast.error(
        `Bulk update: ${done}/${targets.length} done. First error — ${errors[0]}${errors.length > 1 ? ` (+${errors.length - 1} more)` : ''}`,
      )
    } else {
      toast.success(`Bulk update: applied to ${done} connection${done === 1 ? '' : 's'}`)
    }
    onApplied()
    onClose()
  }

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
      <DialogContent className="max-w-lg max-h-[90vh] flex flex-col">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 border border-primary/20">
              <Layers className="h-5 w-5 text-primary" />
            </div>
            <div>
              <DialogTitle className="text-lg">Apply to connections</DialogTitle>
              <DialogDescription className="text-xs mt-1">
                Copy the {models.length} model{models.length === 1 ? '' : 's'} currently selected for{' '}
                <span className="font-medium text-foreground">{source.name}</span> to other connections.
                Prefixes are stripped per connection on apply.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-4 py-2">
          {/* Model list preview */}
          <div className="space-y-1.5">
            <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">Models to apply</span>
            <div className="flex max-h-28 flex-wrap gap-1.5 overflow-y-auto rounded-lg border border-border bg-muted/20 p-2.5">
              {models.length === 0 ? (
                <span className="text-xs text-muted-foreground">
                  Empty list — targets will allow all models.
                </span>
              ) : (
                models.map((m) => (
                  <code
                    key={m}
                    className="rounded-md border border-border bg-background px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground"
                  >
                    {m}
                  </code>
                ))
              )}
            </div>
          </div>

          {/* Connection multi-select */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                Target connections
              </span>
              <div className="flex items-center gap-3 text-xs font-medium">
                <button
                  onClick={selectAll}
                  disabled={applying}
                  className="text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
                >
                  Select all
                </button>
                <button
                  onClick={clearAll}
                  disabled={applying}
                  className="text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
                >
                  Clear
                </button>
              </div>
            </div>
            <div className="max-h-52 space-y-0.5 overflow-y-auto rounded-lg border border-border bg-background p-1">
              {connections.map((target) => (
                <label
                  key={target.id}
                  className={`flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-2 text-sm transition-colors ${
                    selectedIds.has(target.id) ? 'bg-accent' : 'hover:bg-muted/60'
                  }`}
                >
                  <input
                    type="checkbox"
                    checked={selectedIds.has(target.id)}
                    onChange={() => toggle(target.id)}
                    disabled={applying}
                    className="accent-[hsl(var(--primary))]"
                  />
                  <ProviderLogo provider={target.provider} size={16} />
                  <span className="min-w-0 flex-1 truncate font-medium">{target.name}</span>
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {getProviderLabel(target.provider)} · {target.supportedModels?.length || 0} models
                  </span>
                </label>
              ))}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={applying}>
            Cancel
          </Button>
          <Button onClick={handleApply} disabled={applying || selectedIds.size === 0} className="gap-2">
            {applying && <Loader2 size={14} className="animate-spin" />}
            {applying ? `Applying ${progress}…` : `Apply to ${selectedIds.size} connection${selectedIds.size === 1 ? '' : 's'}`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
