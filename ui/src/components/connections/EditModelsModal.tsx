import { useState } from 'react'
import { AlertTriangle, Layers, Loader2, RefreshCw, Settings2 } from 'lucide-react'
import { api } from '../../api'
import ModelSelector, { type ModelTestState } from '../ModelSelector'
import { BulkApplyModelsModal } from './BulkApplyModelsModal'
import { diffModels, fetchModelList, loadedSummary, modelCount, supportsLiveModelFetch } from './model-fetch'
import { stripModelForConnection, withModelPrefix } from './model-prefix'
import { errorMessage } from './add/helpers'
import type { Connection } from '@/types/connections'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface EditModelsModalProps {
  conn: Connection
  /** Other connections that can receive this model list via "Apply to connections". */
  connections?: Connection[]
  onSave: () => void
  /** Called after a bulk apply so the parent can reload (modal stays open). */
  onBulkApplied?: () => void
  onClose: () => void
}

export default function EditModelsModal({ conn, connections = [], onSave, onBulkApplied, onClose }: EditModelsModalProps) {
  // Initialised once per open: a parent reload (e.g. after "Apply to connections")
  // must not wipe edits the user hasn't saved yet. The parent remounts per connection.
  const [selectedModels, setSelectedModels] = useState<string[]>(() => withModelPrefix(conn.supportedModels || [], conn))
  const [syncing, setSyncing] = useState(false)
  const [syncNote, setSyncNote] = useState<{ text: string; warn: boolean } | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [testResults, setTestResults] = useState<Record<string, ModelTestState>>({})
  const [testingAll, setTestingAll] = useState(false)
  const [testProgress, setTestProgress] = useState('')
  const [bulkOpen, setBulkOpen] = useState(false)

  const canSync = supportsLiveModelFetch(conn.provider)
  const busy = saving || syncing

  /**
   * Loads the provider's list straight into the selection. Nothing is saved:
   * the user reviews/edits the list and presses Save, or Cancel to discard it.
   */
  const handleSync = async () => {
    setSyncing(true)
    setSyncNote(null)
    try {
      const fetched = await fetchModelList(conn.id)
      if (!fetched.live) {
        setSyncNote({
          text: `Could not load the provider's list${fetched.note ? ` — ${fetched.note}` : ''}. Your selection was not changed.`,
          warn: true,
        })
        return
      }
      if (fetched.models.length === 0) {
        setSyncNote({ text: 'The provider returned no models. Your selection was not changed.', warn: true })
        return
      }
      const next = withModelPrefix(fetched.models, conn)
      setSyncNote({ text: loadedSummary(next.length, diffModels(selectedModels, next), selectedModels.length > 0), warn: false })
      setSelectedModels(next)
    } catch (e: unknown) {
      setSyncNote({ text: errorMessage(e, 'Failed to load models'), warn: true })
    } finally {
      setSyncing(false)
    }
  }

  const handleTestModel = async (modelId: string) => {
    if (testResults[modelId]?.status === 'loading') return
    setTestResults((prev) => ({ ...prev, [modelId]: { status: 'loading' } }))
    try {
      const res = await api.testModel(conn.id, modelId)
      setTestResults((prev) => ({
        ...prev,
        [modelId]: { status: res.status === 'ok' ? 'ok' : 'error', message: res.message },
      }))
    } catch (e: unknown) {
      setTestResults((prev) => ({
        ...prev,
        [modelId]: { status: 'error', message: errorMessage(e, 'Model test failed') },
      }))
    }
  }

  const handleTestAll = async () => {
    if (selectedModels.length === 0) return
    setTestingAll(true)
    const list = [...selectedModels]
    for (let i = 0; i < list.length; i++) {
      setTestProgress(`${i + 1}/${list.length}`)
      await handleTestModel(list[i])
    }
    setTestProgress('')
    setTestingAll(false)
  }

  const handleSave = async () => {
    setSaving(true)
    setSaveError('')
    try {
      const models = selectedModels.map((m) => stripModelForConnection(m, conn))
      await api.updateConnection(conn.id, { supportedModels: models, setModels: true })
      onSave()
    } catch (e: unknown) {
      setSaveError(errorMessage(e, 'Failed to save models'))
    } finally {
      setSaving(false)
    }
  }

  const otherConnections = connections.filter((c) => c.id !== conn.id)

  return (
    <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}>
      <DialogContent className="max-w-2xl max-h-[90vh] flex flex-col">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 border border-primary/20">
              <Settings2 className="h-5 w-5 text-primary" />
            </div>
            <div>
              <DialogTitle className="text-lg">Edit models — {conn.name}</DialogTitle>
              <DialogDescription className="mt-1">Select which models this connection can serve. Leave empty to allow all.</DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-3 py-2">
          {canSync && (
            <div className="flex flex-wrap items-center gap-3">
              <Button variant="outline" size="sm" onClick={handleSync} disabled={busy} className="gap-2">
                {syncing ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                {syncing ? 'Syncing…' : 'Sync from provider'}
              </Button>
              {syncNote && (
                <span role={syncNote.warn ? 'alert' : 'status'} className={syncNote.warn ? 'text-sm text-warning' : 'text-sm text-muted-foreground'}>
                  {syncNote.text}
                </span>
              )}
            </div>
          )}

          <ModelSelector
            selected={selectedModels}
            onChange={setSelectedModels}
            provider={conn.provider}
            routePrefix={conn.provider === 'openai-compatible' && conn.routePrefix ? conn.routePrefix : undefined}
            test={{
              results: testResults,
              onTest: handleTestModel,
              onTestAll: handleTestAll,
              running: testingAll,
              progress: testProgress,
            }}
          />
        </div>

        {saveError && (
          <div role="alert" className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            <AlertTriangle className="size-4 shrink-0" /> {saveError}
          </div>
        )}

        <div className="flex flex-col-reverse gap-2 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
          {otherConnections.length > 0 ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setBulkOpen(true)}
              className="gap-2 text-muted-foreground"
            >
              <Layers className="size-4" />
              Copy to other connections…
            </Button>
          ) : (
            <span />
          )}
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={onClose} disabled={busy}>Cancel</Button>
            <Button onClick={handleSave} disabled={busy} className="gap-2">
              {saving && <Loader2 className="size-4 animate-spin" />}
              {saving ? 'Saving…' : selectedModels.length === 0 ? 'Save (allow all)' : `Save ${modelCount(selectedModels.length)}`}
            </Button>
          </div>
        </div>
      </DialogContent>

      {bulkOpen && (
        <BulkApplyModelsModal
          source={conn}
          connections={otherConnections}
          models={selectedModels}
          onApplied={() => onBulkApplied?.()}
          onClose={() => setBulkOpen(false)}
        />
      )}
    </Dialog>
  )
}
