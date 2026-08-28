import { useState, useEffect } from 'react'
import { Download, Layers, Loader2, Settings2 } from 'lucide-react'
import { api } from '../../api'
import ModelSelector, { type ModelTestState } from '../ModelSelector'
import { BulkApplyModelsModal } from './BulkApplyModelsModal'
import { stripModelForConnection } from './BulkModelsModal'
import { getModelProviderId } from '@/lib/provider-registry'
import type { Connection } from '@/types/connections'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

function connectionModelPrefix(conn: Connection): string {
  if (conn.provider === 'openai-compatible' && conn.routePrefix) return conn.routePrefix
  return getModelProviderId(conn.provider)
}

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
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [fetching, setFetching] = useState(false)
  const [saving, setSaving] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [testResults, setTestResults] = useState<Record<string, ModelTestState>>({})
  const [testingAll, setTestingAll] = useState(false)
  const [testProgress, setTestProgress] = useState('')
  const [bulkOpen, setBulkOpen] = useState(false)

  useEffect(() => {
    const prefix = connectionModelPrefix(conn)
    const withPrefix = (conn.supportedModels || []).map((m: string) =>
      m.includes('/') ? m : `${prefix}/${m}`
    )
    setSelectedModels(withPrefix)
  }, [conn.id, conn.supportedModels, conn.provider, conn.routePrefix])

  const handleFetchModels = async () => {
    setFetching(true)
    setFetchError('')
    try {
      const res = await api.fetchConnectionModels(conn.id)
      const fetched = res.models || []
      const prefix = connectionModelPrefix(conn)
      setSelectedModels(fetched.map((m: string) => m.includes('/') ? m : `${prefix}/${m}`))
    } catch (e: any) {
      setFetchError(e.message || 'Failed to fetch models')
    } finally {
      setFetching(false)
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
    } catch (e: any) {
      setTestResults((prev) => ({
        ...prev,
        [modelId]: { status: 'error', message: e.message || 'Model test failed' },
      }))
    }
  }

  const handleTestAll = async () => {
    if (selectedModels.length === 0) return
    setTestingAll(true)
    for (let i = 0; i < selectedModels.length; i++) {
      setTestProgress(`${i + 1}/${selectedModels.length}`)
      await handleTestModel(selectedModels[i])
    }
    setTestingAll(false)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const models = selectedModels.map((m) => stripModelForConnection(m, conn))
      await api.updateConnection(conn.id, { supportedModels: models, setModels: true })
      onSave()
    } finally {
      setSaving(false)
    }
  }

  const otherConnections = connections.filter((c) => c.id !== conn.id)

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent className="max-w-2xl max-h-[90vh] flex flex-col">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 border border-primary/20">
              <Settings2 className="h-5 w-5 text-primary" />
            </div>
            <div>
              <DialogTitle className="text-lg">Edit Models — {conn.name}</DialogTitle>
              <DialogDescription className="text-xs mt-1">Select which models this connection can serve. Leave empty to allow all.</DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-3 py-2">
          {(conn.provider === 'openai' || conn.provider === 'openai-compatible') && (
            <div className="flex items-center gap-3">
              <Button variant="outline" size="sm" onClick={handleFetchModels} disabled={fetching} className="gap-2">
                {fetching ? <Loader2 size={12} className="animate-spin" /> : <Download size={12} />}
                Fetch from API
              </Button>
              {fetchError && <span className="text-xs text-amber-600">{fetchError}</span>}
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

        <div className="flex flex-col-reverse gap-2 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
          {otherConnections.length > 0 ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setBulkOpen(true)}
              className="gap-2 text-muted-foreground"
            >
              <Layers size={14} />
              Apply to connections…
            </Button>
          ) : (
            <span />
          )}
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={onClose}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving} className="gap-2">
              {saving && <Loader2 size={14} className="animate-spin" />}
              {saving ? 'Saving…' : 'Save Models'}
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
