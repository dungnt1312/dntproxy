import { useMemo, useState } from 'react'
import { GitBranch, Link, Plus, Trash2, TerminalSquare, Pencil, Search, ChevronsUpDown, Loader2, Layers } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { ComboData, ConnectionOption, UiModel } from './types'
import { RoutingCard } from './routing-card'
import { ComboStepBuilder, ComboStep } from './combo-step-builder'

function inferProvidersFromModels(models: string[]): string[] {
  const map: Record<string, string> = {
    'kr': 'KIRO',
    'oai': 'OPENAI',
    'glm': 'GLM',
    'minimax': 'MINIMAX',
    'qwen': 'QWEN',
    'anthropic': 'ANTHROPIC',
    'gemini': 'GEMINI',
  }
  const providers = new Set<string>()
  models.forEach(m => {
    const prefix = m.split('/')[0]
    if (prefix && map[prefix]) {
      providers.add(map[prefix])
    }
  })
  return Array.from(providers)
}

export interface CombosTabProps {
  combos: ComboData[]
  connections: ConnectionOption[]
  models: UiModel[]
  loading: boolean
  onRefresh: () => Promise<void>
  onOpenLogModal: (comboName: string, allowedProviders: string[]) => void
}

export default function CombosTab({ combos, connections, models, loading, onRefresh, onOpenLogModal }: CombosTabProps) {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ComboData | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [selectedCombo, setSelectedCombo] = useState<ComboData | null>(null)

  const [formName, setFormName] = useState('')
  const [formSteps, setFormSteps] = useState<ComboStep[]>([])

  const [searchQuery, setSearchQuery] = useState('')

  const connectionNameMap = useMemo(
    () => new Map(connections.map((connection) => [connection.id, connection.name])),
    [connections]
  )

  const modelNameMap = useMemo(
    () => new Map(models.map((model) => [model.id, model.name || model.id])),
    [models]
  )

  // Parse model string "provider/model@connectionId" to ComboStep
  function parseModelString(modelStr: string, order: number): ComboStep {
    const [modelPart, connId] = modelStr.split('@')
    const parts = modelPart.split('/')
    
    // Handle "provider/model" or "provider/subprovider/model"
    const provider = parts[0]
    const model = parts.slice(1).join('/')
    
    return {
      id: `step-${order}`,
      provider,
      model,
      accountMode: connId && connId !== 'auto' ? 'pinned' : 'auto',
      accountId: connId && connId !== 'auto' ? connId : undefined,
      order,
    }
  }

  // Serialize ComboStep to model string "provider/model@connectionId"
  function serializeStep(step: ComboStep): string {
    const base = `${step.provider}/${step.model}`
    
    if (step.accountMode === 'pinned' && step.accountId) {
      return `${base}@${step.accountId}`
    }
    return base
  }

  function openCreateDialog() {
    setSelectedCombo(null)
    setFormName('')
    setFormSteps([])
    setDialogOpen(true)
  }

  function openEditDialog(combo: ComboData) {
    setSelectedCombo(combo)
    setFormName(combo.name)
    // Parse existing models to steps
    const steps = combo.models.map((m, idx) => parseModelString(m, idx))
    setFormSteps(steps)
    setDialogOpen(true)
  }

  async function handleSave() {
    const name = formName.trim()

    if (!name) {
      toast.error('Combo name is required')
      return
    }

    if (formSteps.length === 0) {
      toast.error('At least one step is required')
      return
    }

    setSaving(true)
    try {
      // Serialize steps to model strings
      const models = formSteps
        .sort((a, b) => a.order - b.order)
        .map(serializeStep)

      const payload = {
        name,
        models,
        setModels: true,
      }

      if (selectedCombo) {
        await goApi.updateCombo(selectedCombo.id, payload)
        toast.success('Combo updated')
      } else {
        await goApi.createCombo(payload)
        toast.success('Combo created')
      }

      setDialogOpen(false)
      await onRefresh()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to save combo')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return

    setDeleting(true)
    try {
      await goApi.deleteCombo(deleteTarget.id)
      setDeleteTarget(null)
      toast.success('Combo deleted')
      await onRefresh()
    } catch {
      toast.error('Failed to delete combo')
    } finally {
      setDeleting(false)
    }
  }

  const filteredCombos = useMemo(() => {
    const q = searchQuery.toLowerCase().trim()
    if (!q) return combos
    return combos.filter(c => 
       c.name.toLowerCase().includes(q) || 
       c.models.some(m => m.toLowerCase().includes(q) || (modelNameMap.get(m)?.toLowerCase() || '').includes(q))
    )
  }, [combos, searchQuery, modelNameMap])

  return (
    <div className="space-y-4 pt-0">
      {/* Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
         <div className="relative flex-1 min-w-0 sm:max-w-md">
             <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
             <Input 
                type="search" 
                value={searchQuery} 
                onChange={e => setSearchQuery(e.target.value)} 
                placeholder="Search combos by name or model..." 
                className="pl-9 h-9 text-sm"
             />
         </div>
         <Button onClick={openCreateDialog} className="gap-2 self-start sm:self-auto h-9 bg-emerald-600 hover:bg-emerald-700 text-white">
           <Plus className="h-4 w-4" />
           Create Combo
         </Button>
      </div>

      {/* Content */}
      {loading ? (
        <div className="flex items-center justify-center rounded-lg border bg-card p-16 text-sm text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin mr-2" />
          Loading combos...
        </div>
      ) : combos.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed py-20 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-violet-500/10 mb-4">
            <GitBranch className="h-6 w-6 text-violet-500" />
          </div>
          <h3 className="mb-1 text-lg font-semibold">No combos yet</h3>
          <p className="mb-4 max-w-md text-sm text-muted-foreground">
            Create a named list of model targets. The backend will resolve the combo according to the global combo strategy.
          </p>
          <Button onClick={openCreateDialog} variant="outline" className="gap-2">
            <Plus className="h-4 w-4" />
            Create Combo
          </Button>
        </div>
      ) : filteredCombos.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border py-12 text-center">
           <p className="text-sm text-muted-foreground">No combos matching "{searchQuery.trim()}"</p>
           <Button variant="link" onClick={() => setSearchQuery('')} className="mt-2 text-sm">Clear filter</Button>
        </div>
      ) : (
        <div className="rounded-xl border bg-card shadow-sm overflow-hidden">
          <div className="divide-y divide-border/50">
            {filteredCombos.map((combo) => (
              <RoutingCard
                key={combo.id}
                title={combo.name}
                type="combo"
                targets={combo.models.map(m => modelNameMap.get(m) || m)}
                badges={
                  combo.connectionIds && combo.connectionIds.length > 0 && (
                    <Badge variant="outline" className="h-5 px-2 text-[11px] font-normal text-muted-foreground bg-muted/30 whitespace-nowrap gap-1">
                      <Link className="h-3 w-3 opacity-70" />
                      {combo.connectionIds.length} conn{combo.connectionIds.length !== 1 && 's'}
                    </Badge>
                  )
                }
                actions={
                  <>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-muted"
                      onClick={() => onOpenLogModal(combo.name, inferProvidersFromModels(combo.models))}
                      title={`View logs for combo: ${combo.name}`}
                    >
                      <TerminalSquare className="h-4 w-4" />
                    </Button>
                    <Button 
                      variant="ghost" 
                      size="icon" 
                      className="h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-muted" 
                      onClick={() => openEditDialog(combo)}
                      title="Edit Combo"
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-destructive/70 hover:text-destructive hover:bg-destructive/10"
                      onClick={() => setDeleteTarget(combo)}
                      title="Delete Combo"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </>
                }
              />
            ))}
          </div>
        </div>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-violet-500/10">
                <GitBranch className="h-4 w-4 text-violet-500" />
              </div>
              {selectedCombo ? 'Edit Combo' : 'Create Combo'}
            </DialogTitle>
            <DialogDescription>
              Build each combo step in sequence: provider, model, then account. This allows repeating the same provider and model with different accounts.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-2">
            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor="combo-name" className="text-xs font-medium">Combo Name</Label>
              <Input
                id="combo-name"
                value={formName}
                onChange={(event) => setFormName(event.target.value)}
                placeholder="e.g. primary-backup-chain"
                className="h-9"
              />
            </div>

            {/* Step Builder */}
            <ComboStepBuilder
              steps={formSteps}
              connections={connections}
              models={models}
              onChange={setFormSteps}
            />
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSave} disabled={saving || formSteps.length === 0} className="gap-1.5">
              {saving ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  Saving…
                </>
              ) : selectedCombo ? 'Update Combo' : 'Create Combo'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Combo</AlertDialogTitle>
            <AlertDialogDescription>
              Delete &quot;{deleteTarget?.name}&quot;? This removes the combo name and its model list.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault()
                handleDelete()
              }}
              disabled={deleting}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deleting ? 'Deleting…' : 'Delete'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

    </div>
  )
}
