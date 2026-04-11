import { useEffect, useMemo, useState } from 'react'
import { GitBranch, Link, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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

type ComboData = {
  id: string
  name: string
  models: string[]
  connectionIds?: string[]
  createdAt?: string
  updatedAt?: string
}

type ConnectionOption = {
  id: string
  name: string
  provider: string
}

type ModelOption = {
  id: string
  displayName: string
  provider: string
}

export default function CombosScreen() {
  const [combos, setCombos] = useState<ComboData[]>([])
  const [connections, setConnections] = useState<ConnectionOption[]>([])
  const [models, setModels] = useState<ModelOption[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ComboData | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [selectedCombo, setSelectedCombo] = useState<ComboData | null>(null)

  const [formName, setFormName] = useState('')
  const [formModels, setFormModels] = useState('')
  const [formConnectionIds, setFormConnectionIds] = useState<string[]>([])

  async function load() {
    setLoading(true)
    try {
      const [comboData, connectionData, modelData] = await Promise.all([
        goApi.getCombos(),
        goApi.getConnections(),
        goApi.getModels(),
      ])

      setCombos(Array.isArray(comboData) ? comboData : [])
      setConnections(Array.isArray(connectionData) ? connectionData : [])
      setModels(Array.isArray(modelData) ? modelData.filter((item) => item.provider !== 'alias' && item.provider !== 'combo') : [])
    } catch {
      toast.error('Failed to load combos')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const connectionNameMap = useMemo(
    () => new Map(connections.map((connection) => [connection.id, connection.name])),
    [connections]
  )

  const modelNameMap = useMemo(
    () => new Map(models.map((model) => [model.id, model.displayName])),
    [models]
  )

  function openCreateDialog() {
    setSelectedCombo(null)
    setFormName('')
    setFormModels('')
    setFormConnectionIds([])
    setDialogOpen(true)
  }

  function openEditDialog(combo: ComboData) {
    setSelectedCombo(combo)
    setFormName(combo.name)
    setFormModels(combo.models.join('\n'))
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
    const modelsList = formModels
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean)

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
      await load()
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
      await load()
    } catch {
      toast.error('Failed to delete combo')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-500/10">
            <GitBranch className="h-5 w-5 text-emerald-500" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Combos</h1>
            <p className="text-sm text-muted-foreground">
              Backend supports named model lists with optional connection restrictions.
            </p>
          </div>
        </div>
        <Button onClick={openCreateDialog} className="gap-2 self-start sm:self-auto">
          <Plus className="h-4 w-4" />
          Create Combo
        </Button>
      </div>

      {loading ? (
        <Card>
          <CardContent className="p-6 text-sm text-muted-foreground">Loading combos...</CardContent>
        </Card>
      ) : combos.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
          <GitBranch className="mb-4 h-12 w-12 text-muted-foreground/40" />
          <h3 className="mb-1 text-lg font-medium">No combos yet</h3>
          <p className="mb-4 max-w-md text-sm text-muted-foreground">
            Create a named list of model targets. The backend will resolve the combo according to the global combo strategy.
          </p>
          <Button onClick={openCreateDialog} variant="outline" className="gap-2">
            <Plus className="h-4 w-4" />
            Create Combo
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {combos.map((combo) => (
            <Card key={combo.id}>
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <CardTitle className="truncate text-base">{combo.name}</CardTitle>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {combo.models.length} model{combo.models.length === 1 ? '' : 's'}
                      {combo.connectionIds && combo.connectionIds.length > 0
                        ? ` · ${combo.connectionIds.length} restricted connection${combo.connectionIds.length === 1 ? '' : 's'}`
                        : ' · all matching connections'}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm" onClick={() => openEditDialog(combo)}>
                      Edit
                    </Button>
                    <Button
                      variant="outline"
                      size="icon"
                      className="text-destructive hover:text-destructive"
                      onClick={() => setDeleteTarget(combo)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Models</p>
                  <div className="space-y-1 rounded-md border bg-muted/20 p-3 text-sm">
                    {combo.models.map((modelId) => (
                      <div key={modelId} className="font-mono text-xs text-muted-foreground break-all">
                        {modelNameMap.get(modelId) || modelId}
                      </div>
                    ))}
                  </div>
                </div>

                <div className="space-y-2">
                  <p className="flex items-center gap-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    <Link className="h-3 w-3" />
                    Connections
                  </p>
                  <div className="rounded-md border bg-muted/20 p-3 text-sm text-muted-foreground">
                    {combo.connectionIds && combo.connectionIds.length > 0 ? (
                      <div className="space-y-1">
                        {combo.connectionIds.map((connectionId) => (
                          <div key={connectionId}>{connectionNameMap.get(connectionId) || connectionId}</div>
                        ))}
                      </div>
                    ) : (
                      <span>No restriction. Backend may use any compatible active connection.</span>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{selectedCombo ? 'Edit Combo' : 'Create Combo'}</DialogTitle>
            <DialogDescription>
              Define the combo exactly as supported by the backend: name, ordered models, and optional connection restrictions.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="combo-name">Name</Label>
              <Input
                id="combo-name"
                value={formName}
                onChange={(event) => setFormName(event.target.value)}
                placeholder="e.g. coder-fallback"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="combo-models">Models</Label>
              <Textarea
                id="combo-models"
                value={formModels}
                onChange={(event) => setFormModels(event.target.value)}
                rows={8}
                placeholder={'One model per line\nkr/Codex-sonnet-4-5-20250514\noai/gpt-4.1'}
                className="font-mono text-xs"
              />
              <p className="text-xs text-muted-foreground">
                Order matters. Backend combo routing uses this ordered list together with the global combo strategy.
              </p>
            </div>

            <div className="space-y-2">
              <Label>Restrict to Connections (optional)</Label>
              <div className="max-h-44 space-y-2 overflow-y-auto rounded-md border p-3">
                {connections.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No connections available.</p>
                ) : (
                  connections.map((connection) => (
                    <label key={connection.id} className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={formConnectionIds.includes(connection.id)}
                        onChange={() => toggleConnection(connection.id)}
                      />
                      <span>{connection.name}</span>
                      <span className="text-xs text-muted-foreground">{connection.provider}</span>
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
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Saving…' : selectedCombo ? 'Update Combo' : 'Create Combo'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
