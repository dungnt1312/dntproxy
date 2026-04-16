import { useMemo, useState } from 'react'
import { GitBranch, Link, Plus, Trash2, TerminalSquare, Pencil, Search, ChevronsUpDown, Loader2, Layers } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
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
import { Checkbox } from '@/components/ui/checkbox'
import { ComboData, ConnectionOption, UiModel } from './types'
import { RoutingCard } from './routing-card'

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
  const [formSelectedModels, setFormSelectedModels] = useState<string[]>([])
  const [formConnectionIds, setFormConnectionIds] = useState<string[]>([])

  const [searchQuery, setSearchQuery] = useState('')
  const [modelComboboxOpen, setModelComboboxOpen] = useState(false)

  const connectionNameMap = useMemo(
    () => new Map(connections.map((connection) => [connection.id, connection.name])),
    [connections]
  )

  const modelNameMap = useMemo(
    () => new Map(models.map((model) => [model.id, model.name || model.id])),
    [models]
  )

  function openCreateDialog() {
    setSelectedCombo(null)
    setFormName('')
    setFormSelectedModels([])
    setFormConnectionIds([])
    setDialogOpen(true)
  }

  function openEditDialog(combo: ComboData) {
    setSelectedCombo(combo)
    setFormName(combo.name)
    setFormSelectedModels(combo.models)
    setFormConnectionIds(combo.connectionIds || [])
    setDialogOpen(true)
  }

  function toggleConnection(connectionId: string) {
    setFormConnectionIds((current) =>
      current.includes(connectionId)
        ? current.filter((id) => id !== connectionId)
        : [...current, connectionId]
    )
  }

  async function handleSave() {
    const name = formName.trim()
    const modelsList = formSelectedModels

    if (!name) {
      toast.error('Combo name is required')
      return
    }

    if (modelsList.length === 0) {
      toast.error('At least one model is required')
      return
    }

    setSaving(true)
    try {
      const payload = {
        name,
        models: modelsList,
        connectionIds: formConnectionIds,
        setModels: true,
        setConnections: true,
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
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-violet-500/10">
                <GitBranch className="h-4 w-4 text-violet-500" />
              </div>
              {selectedCombo ? 'Edit Combo' : 'Create Combo'}
            </DialogTitle>
            <DialogDescription>
              Define the combo exactly as supported by the backend: name, ordered models, and optional connection restrictions.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-2">
            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor="combo-name" className="text-xs font-medium">Name</Label>
              <Input
                id="combo-name"
                value={formName}
                onChange={(event) => setFormName(event.target.value)}
                placeholder="e.g. coder-fallback"
                className="h-9"
              />
            </div>

            {/* Models Chain */}
            <div className="space-y-2">
              <Label className="text-xs font-medium">Models Chain</Label>
              <Popover open={modelComboboxOpen} onOpenChange={setModelComboboxOpen}>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={modelComboboxOpen}
                    className="w-full justify-between h-9 text-muted-foreground shadow-sm font-normal"
                  >
                    <span className="flex items-center gap-2">
                      <Layers className="h-3.5 w-3.5 opacity-50" />
                      Search and select to add a model
                    </span>
                    <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
                  <Command>
                    <CommandInput placeholder="Search models..." />
                    <CommandList>
                      <CommandEmpty>No model found.</CommandEmpty>
                      <CommandGroup>
                        {models.map((m) => (
                          <CommandItem
                            key={m.id}
                            value={`${m.provider} ${m.displayName || ''} ${m.id}`}
                            onSelect={() => {
                              setFormSelectedModels(prev => [...prev, m.id]);
                              setModelComboboxOpen(false);
                            }}
                          >
                            <div className="flex flex-col">
                              <span className="font-medium text-sm">{m.name || m.id}</span>
                              <span className="text-xs text-muted-foreground">({m.provider}) {m.id}</span>
                            </div>
                          </CommandItem>
                        ))}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
              
              {/* Selected models */}
              <div className="space-y-1.5 mt-3">
                 {formSelectedModels.map((m, idx) => (
                    <div key={`${m}-${idx}`} className="flex items-center justify-between border rounded-lg px-3 py-2 text-sm bg-muted/20 group/item hover:bg-muted/40 transition-colors">
                       <div className="flex items-center gap-2.5 min-w-0">
                           <span className="flex h-5 w-5 items-center justify-center rounded text-[10px] font-semibold bg-violet-500/10 text-violet-600 dark:text-violet-400 shrink-0">
                             {idx + 1}
                           </span>
                           <span className="font-medium truncate text-sm" title={modelNameMap.get(m) || m}>{modelNameMap.get(m) || m}</span>
                       </div>
                       <Button 
                          variant="ghost" 
                          size="icon" 
                          className="h-6 w-6 text-muted-foreground hover:text-destructive opacity-0 group-hover/item:opacity-100 transition-opacity shrink-0" 
                          onClick={() => setFormSelectedModels(prev => prev.filter((_, i) => i !== idx))}
                       >
                          <Trash2 className="h-3 w-3" />
                       </Button>
                    </div>
                 ))}
                 {formSelectedModels.length === 0 && (
                    <div className="text-xs text-muted-foreground p-4 border border-dashed rounded-lg text-center">
                      No models added yet. Use the dropdown above to add models.
                    </div>
                 )}
              </div>
              <p className="text-[11px] text-muted-foreground">
                Order matters — backend combo routing uses this list with the global strategy.
              </p>
            </div>

            {/* Connection restrictions */}
            <div className="space-y-2">
              <Label className="text-xs font-medium">Restrict to Connections (optional)</Label>
              <div className="max-h-44 space-y-1 overflow-y-auto rounded-lg border p-3">
                {connections.length === 0 ? (
                  <p className="text-sm text-muted-foreground py-2 text-center">No connections available.</p>
                ) : (
                  connections.map((connection) => (
                    <label 
                      key={connection.id} 
                      className="flex items-center gap-2.5 text-sm rounded-md px-2 py-1.5 hover:bg-muted/50 transition-colors cursor-pointer"
                    >
                      <Checkbox
                        checked={formConnectionIds.includes(connection.id)}
                        onCheckedChange={() => toggleConnection(connection.id)}
                      />
                      <span className="font-medium text-sm">{connection.name}</span>
                      <span className="text-xs text-muted-foreground ml-auto">{connection.provider}</span>
                    </label>
                  ))
                )}
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSave} disabled={saving} className="gap-1.5">
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
