import { useState, useEffect } from 'react'
import { Download, Loader2, Settings2 } from 'lucide-react'
import { api } from '../../api'
import ModelSelector from '../ModelSelector'
import { getModelProviderId } from '@/lib/provider-registry'
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

interface EditModelsModalProps {
  conn: Connection
  onSave: () => void
  onClose: () => void
}

export default function EditModelsModal({ conn, onSave, onClose }: EditModelsModalProps) {
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [fetching, setFetching] = useState(false)
  const [saving, setSaving] = useState(false)
  const [fetchError, setFetchError] = useState('')

  useEffect(() => {
    const modelProvider = getModelProviderId(conn.provider)
    const withPrefix = (conn.supportedModels || []).map((m: string) =>
      m.includes('/') ? m : `${modelProvider}/${m}`
    )
    setSelectedModels(withPrefix)
  }, [conn.id, conn.supportedModels, conn.provider])

  const handleFetchModels = async () => {
    setFetching(true)
    setFetchError('')
    try {
      const res = await api.fetchConnectionModels(conn.id)
      const fetched = res.models || []
      const modelProvider = getModelProviderId(conn.provider)
      setSelectedModels(fetched.map((m: string) => m.includes('/') ? m : `${modelProvider}/${m}`))
    } catch (e: any) {
      setFetchError(e.message || 'Failed to fetch models')
    } finally {
      setFetching(false)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const modelProvider = getModelProviderId(conn.provider)
      const models = selectedModels.map(m => {
        if (m.startsWith(modelProvider + '/')) return m.slice(modelProvider.length + 1)
        const slash = m.indexOf('/')
        return slash >= 0 ? m.slice(slash + 1) : m
      })
      await api.updateConnection(conn.id, { supportedModels: models, setModels: true })
      onSave()
    } finally {
      setSaving(false)
    }
  }

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

        <div className="flex-1 overflow-y-auto space-y-4 py-2">
          {(conn.provider === 'openai' || conn.provider === 'openai-compatible') && (
            <div className="flex items-center gap-3">
              <Button variant="outline" size="sm" onClick={handleFetchModels} disabled={fetching} className="gap-2">
                {fetching ? <Loader2 size={12} className="animate-spin" /> : <Download size={12} />}
                Fetch from API
              </Button>
              {fetchError && <span className="text-xs text-amber-600">{fetchError}</span>}
            </div>
          )}
          <ModelSelector selected={selectedModels} onChange={setSelectedModels} provider={conn.provider} />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving} className="gap-2">
            {saving && <Loader2 size={14} className="animate-spin" />}
            {saving ? 'Saving…' : 'Save Models'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
