import { useState } from 'react'
import { AlertTriangle, Loader2, Trash2, X } from 'lucide-react'

interface DeleteDialogProps {
  target: { id: string; name: string }
  onConfirm: (id: string) => Promise<void>
  onClose: () => void
}

export default function DeleteDialog({ target, onConfirm, onClose }: DeleteDialogProps) {
  const [loading, setLoading] = useState(false)

  const handleConfirm = async () => {
    setLoading(true)
    try {
      await onConfirm(target.id)
      onClose()
    } catch {
      setLoading(false)
    }
  }

  return (
    <div
      className="modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="delete-dialog-title"
      onClick={onClose}
    >
      <div
        className="modal-content sm:max-w-md"
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-[var(--danger-glow)] border border-[var(--danger)]/20 flex items-center justify-center">
              <AlertTriangle size={20} className="text-[var(--danger)]" />
            </div>
            <div>
              <h3 id="delete-dialog-title" className="modal-title">Remove Connection</h3>
              <p className="modal-subtitle">This action cannot be undone</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="btn-icon shrink-0"
            aria-label="Close"
          >
            <X size={18} />
          </button>
        </div>

        <div className="modal-body space-y-5">
          <div className="glass-sm p-4 text-center">
            <p className="text-sm text-[var(--text-muted)]">
              Are you sure you want to remove
            </p>
            <p className="text-base font-semibold mt-1" style={{ fontFamily: 'var(--font-heading)' }}>
              {target.name}
            </p>
            <p className="text-xs text-[var(--text-dim)] mt-2">
              All tokens, settings, and quota data for this connection will be permanently deleted.
            </p>
          </div>

          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="btn-ghost flex-1"
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleConfirm}
              disabled={loading}
              className="btn-danger flex-1"
            >
              {loading ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
              {loading ? 'Removing…' : 'Remove'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
