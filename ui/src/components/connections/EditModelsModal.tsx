import { useState, useEffect } from 'react'
import { X, Download, Loader2, Settings2 } from 'lucide-react'
import { api } from '../../api'
import ModelSelector from '../ModelSelector'

interface EditModelsModalProps {
  conn: any
  onSave: () => void
  onClose: () => void
}

export default function EditModelsModal({ conn, onSave, onClose }: EditModelsModalProps) {
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [fetching, setFetching] = useState(false)
  const [saving, setSaving] = useState(false)
  const [fetchError, setFetchError] = useState('')

  // Initialize with current models (with provider prefix)
  useEffect(() => {
    const withPrefix = (conn.supportedModels || []).map((m: string) =>
      m.includes('/') ? m : `${conn.provider}/${m}`
    )
    setSelectedModels(withPrefix)
  }, [conn.id, conn.supportedModels, conn.provider])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const handleFetchModels = async () => {
    setFetching(true)
    setFetchError('')
    try {
      const res = await api.fetchConnectionModels(conn.id)
      const fetched = res.models || []
      const withPrefix = fetched.map((m: string) => m.includes('/') ? m : `${conn.provider}/${m}`)
      setSelectedModels(withPrefix)
    } catch (e: any) {
      if (conn.provider === 'openai' || conn.provider === 'openai-compatible') {
        const fallbacks = [
          'gpt-4o', 'gpt-4o-mini', 'o1', 'o1-mini', 'o3-mini', 'chatgpt-4o-latest',
          'gpt-4-turbo', 'gpt-3.5-turbo', 'claude-3-5-sonnet-20240620', 'claude-3-opus-20240229'
        ]
        setSelectedModels(fallbacks)
        setFetchError('Fetch failed. Loaded fallback model list.')
      } else {
        setFetchError(e.message || 'Failed to fetch models')
      }
    } finally {
      setFetching(false)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const models = selectedModels.map(m => {
        if (m.startsWith(conn.provider + '/')) return m.slice(conn.provider.length + 1)
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
    <div
      className="modal-overlay"
      role="presentation"
      onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="edit-models-title"
        className="modal-content sm:max-w-2xl"
        onMouseDown={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="modal-header">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-10 h-10 rounded-xl bg-[var(--purple-glow)] border border-[var(--purple)]/20 flex items-center justify-center shrink-0">
              <Settings2 size={18} className="text-[var(--purple)]" />
            </div>
            <div className="min-w-0">
              <h3 id="edit-models-title" className="modal-title truncate">
                Edit Models — {conn.name}
              </h3>
              <p className="modal-subtitle">
                Select which models this connection can serve. Leave empty to allow all.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="btn-icon shrink-0"
            aria-label="Close"
          >
            <X size={18} />
          </button>
        </div>

        {/* Content */}
        <div className="modal-body space-y-4">
          {/* Fetch from API button */}
          {(conn.provider === 'openai' || conn.provider === 'openai-compatible') && (
            <div className="flex items-center gap-3">
              <button
                onClick={handleFetchModels}
                disabled={fetching}
                className="btn-ghost text-xs"
              >
                {fetching ? <Loader2 size={12} className="animate-spin" /> : <Download size={12} />}
                Fetch from API
              </button>
              {fetchError && (
                <span className="text-xs text-[var(--warning)]">{fetchError}</span>
              )}
            </div>
          )}

          <ModelSelector
            selected={selectedModels}
            onChange={setSelectedModels}
            provider={conn.provider}
          />
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 px-5 py-4 border-t border-[var(--border)] shrink-0">
          <button
            onClick={onClose}
            className="btn-ghost"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="btn-primary"
          >
            {saving ? (
              <span className="flex items-center gap-2"><Loader2 size={14} className="animate-spin" /> Saving…</span>
            ) : 'Save Models'}
          </button>
        </div>
      </div>
    </div>
  )
}
