import { useEffect, useState, useCallback } from 'react'
import { api } from '../api'
import { Plus, Trash2, Users, Layers, Pencil, Check, Loader2, X } from 'lucide-react'
import ModelSelector from '../components/ModelSelector'
import { getModelName } from '../models-config'

export default function Combos() {
  const [combos, setCombos] = useState<any[]>([])
  const [conns, setConns] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [selectedConnIDs, setSelectedConnIDs] = useState<string[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const [editModels, setEditModels] = useState<string[]>([])
  const [editConnIDs, setEditConnIDs] = useState<string[]>([])

  const load = useCallback(() => {
    api.getCombos().then(d => setCombos(d || [])).catch(() => {})
    api.getConnections().then(d => setConns(d || [])).catch(() => {})
  }, [])

  useEffect(() => { load() }, [load])

  const getAllowedModels = (connIDs: string[]): string[] | undefined => {
    if (connIDs.length === 0) return undefined
    const union = new Set<string>()
    for (const id of connIDs) {
      const conn = conns.find(c => c.id === id)
      if (conn?.supportedModels?.length > 0) {
        conn.supportedModels.forEach((m: string) => {
          // SupportedModels stores bare IDs like "claude-sonnet-4.5"
          // MODELS_CONFIG uses "provider/model-id" format
          union.add(m.includes('/') ? m : `${conn.provider}/${m}`)
        })
      }
    }
    return union.size > 0 ? Array.from(union) : undefined
  }

  const resetCreateForm = () => {
    setName('')
    setSelectedModels([])
    setSelectedConnIDs([])
    setError('')
  }

  const openCreate = () => {
    setEditingId(null)
    resetCreateForm()
    setShowAdd(true)
  }

  const closeCreate = () => {
    setShowAdd(false)
    resetCreateForm()
  }

  const handleAdd = async () => {
    setError(''); setLoading(true)
    if (!name.trim()) { setError('Name is required'); setLoading(false); return }
    if (selectedModels.length === 0) { setError('Add at least one model'); setLoading(false); return }
    try {
      await api.createCombo({
        name: name.trim(),
        models: selectedModels,
        connectionIds: selectedConnIDs.length > 0 ? selectedConnIDs : undefined,
      })
      closeCreate()
      load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Remove this combo?')) return
    await api.deleteCombo(id); load()
  }

  const startEdit = (c: any) => {
    setShowAdd(false)
    setEditingId(c.id)
    setEditName(c.name)
    setEditModels(c.models || [])
    setEditConnIDs(c.connectionIds || [])
    setError('')
  }

  const cancelEdit = () => {
    setEditingId(null)
    setError('')
  }

  const saveEdit = async (id: string) => {
    if (editModels.length === 0) { setError('At least one model required'); return }
    setLoading(true)
    try {
      await api.updateCombo(id, {
        name: editName,
        models: editModels,
        setModels: true,
        connectionIds: editConnIDs,
        setConnections: true,
      })
      cancelEdit(); load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const toggleConn = (id: string, arr: string[], setArr: (v: string[]) => void) => {
    setArr(arr.includes(id) ? arr.filter(x => x !== id) : [...arr, id])
  }

  const ConnSelector = ({
    selected, onToggle,
  }: { selected: string[]; onToggle: (id: string) => void }) => (
    <div className="max-h-44 overflow-y-auto rounded-lg border border-[var(--border)] divide-y divide-[var(--border)] bg-[var(--bg)]">
      {conns.length === 0 && (
        <p className="text-xs text-[var(--text-muted)] px-3 py-3">No connections yet — add some under Connections.</p>
      )}
      {conns.map(c => {
        const isSelected = selected.includes(c.id)
        const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
        return (
          <label
            key={c.id}
            className={`flex items-center gap-3 px-3 py-2.5 cursor-pointer text-sm ${
              isSelected ? 'bg-[var(--accent)]/8' : 'hover:bg-[var(--bg-hover)]'
            }`}
          >
            <input
              type="checkbox"
              checked={isSelected}
              onChange={() => onToggle(c.id)}
              className="accent-[var(--accent)] w-3.5 h-3.5 shrink-0"
            />
            <span className="truncate font-medium min-w-0">{c.name}</span>
            <span className="text-xs text-[var(--text-muted)] shrink-0">{c.provider}</span>
            {isRL && (
              <span className="text-[10px] text-amber-500 shrink-0" title="Rate limited">RL</span>
            )}
            {!c.isActive && (
              <span className="text-[10px] text-[var(--text-muted)] shrink-0">off</span>
            )}
          </label>
        )
      })}
    </div>
  )

  const ComboForm = ({
    nameVal, setNameVal,
    models, setModels,
    connIDs, setConnIDs,
    onSave, onCancel,
    saveLabel,
  }: {
    nameVal: string; setNameVal: (v: string) => void
    models: string[]; setModels: (v: string[]) => void
    connIDs: string[]; setConnIDs: (v: string[]) => void
    onSave: () => void; onCancel: () => void
    saveLabel: string
  }) => {
    const allowedModels = getAllowedModels(connIDs)
    return (
      <div className="space-y-5">
        <div>
          <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Name</label>
          <input
            value={nameVal}
            onChange={e => setNameVal(e.target.value)}
            placeholder="e.g. fast-fallback"
            className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none"
          />
        </div>

        <details className="group rounded-lg border border-[var(--border)] bg-[var(--bg)] px-3 py-2.5 open:pb-3">
          <summary className="cursor-pointer list-none text-xs font-medium text-[var(--text-muted)] hover:text-[var(--text)] flex items-center gap-2 [&::-webkit-details-marker]:hidden">
            <Users size={12} className="opacity-70 shrink-0" />
            <span>Pin specific connections</span>
            <span className="font-normal text-[var(--text-muted)]">(optional)</span>
          </summary>
          <div className="pt-3 space-y-2">
            <ConnSelector
              selected={connIDs}
              onToggle={id => toggleConn(id, connIDs, setConnIDs)}
            />
            {connIDs.length > 0 && (
              <p className="text-xs text-[var(--text-muted)]">
                {allowedModels
                  ? `Model picker limited to ${allowedModels.length} id(s) from those accounts.`
                  : 'Those accounts place no model restriction — full catalog below.'}
              </p>
            )}
          </div>
        </details>

        <div>
          <label className="flex items-center gap-1.5 text-xs font-medium text-[var(--text-muted)] mb-1.5">
            <Layers size={12} className="opacity-70" />
            Models (fallback order)
          </label>
          <ModelSelector
            selected={models}
            onChange={setModels}
            allowedModels={allowedModels}
          />
        </div>

        {error && <p className="text-sm text-[var(--danger)]">{error}</p>}

        <div className="flex flex-wrap gap-2 pt-1 border-t border-[var(--border)]">
          <button
            type="button"
            onClick={onSave}
            disabled={loading}
            className="flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50 transition-colors"
          >
            {loading ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
            {loading ? 'Saving…' : saveLabel}
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2 bg-[var(--bg)] hover:bg-[var(--bg-hover)] rounded-lg text-sm text-[var(--text-muted)] transition-colors"
          >
            Cancel
          </button>
        </div>
      </div>
    )
  }

  const ModalChrome = ({
    title, subtitle, titleId, onClose, children,
  }: { title: string; subtitle?: string; titleId: string; onClose: () => void; children: React.ReactNode }) => (
    <div
      className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-black/50 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      onClick={onClose}
    >
      <div
        className="bg-[var(--bg-card)] border border-[var(--border)] sm:rounded-xl w-full sm:max-w-lg max-h-[min(92vh,720px)] flex flex-col shadow-xl"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3 px-4 pt-4 pb-3 border-b border-[var(--border)] shrink-0">
          <div className="min-w-0">
            <h3 id={titleId} className="text-lg font-semibold leading-tight">{title}</h3>
            {subtitle && (
              <p className="text-xs text-[var(--text-muted)] mt-1">{subtitle}</p>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-[var(--bg-hover)] text-[var(--text-muted)] shrink-0"
            aria-label="Close"
          >
            <X size={18} />
          </button>
        </div>
        <div className="overflow-y-auto px-4 py-4 flex-1 min-h-0">
          {children}
        </div>
      </div>
    </div>
  )

  return (
    <div className="max-w-3xl">
      <div className="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-4 mb-6">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Combos</h2>
          <p className="text-sm text-[var(--text-muted)] mt-1 max-w-md">
            Named model fallback chains for routing. Use the API with the combo name as the model id.
          </p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="flex items-center justify-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors shrink-0"
        >
          <Plus size={16} /> New combo
        </button>
      </div>

      {showAdd && (
        <ModalChrome
          title="New combo"
          subtitle="Pick a name, add models in order, optionally pin accounts."
          titleId="combo-dialog-new"
          onClose={closeCreate}
        >
          <ComboForm
            nameVal={name} setNameVal={setName}
            models={selectedModels} setModels={setSelectedModels}
            connIDs={selectedConnIDs} setConnIDs={setSelectedConnIDs}
            onSave={handleAdd}
            onCancel={closeCreate}
            saveLabel="Create"
          />
        </ModalChrome>
      )}

      {editingId && (
        <ModalChrome
          title="Edit combo"
          subtitle="Changes apply to new requests immediately."
          titleId="combo-dialog-edit"
          onClose={cancelEdit}
        >
          <ComboForm
            nameVal={editName} setNameVal={setEditName}
            models={editModels} setModels={setEditModels}
            connIDs={editConnIDs} setConnIDs={setEditConnIDs}
            onSave={() => saveEdit(editingId)}
            onCancel={cancelEdit}
            saveLabel="Save"
          />
        </ModalChrome>
      )}

      {combos.length === 0 ? (
        <div className="rounded-xl border border-dashed border-[var(--border)] bg-[var(--bg-card)]/50 px-6 py-10 text-center">
          <p className="text-[var(--text-muted)] text-sm">No combos yet.</p>
          <button
            type="button"
            onClick={openCreate}
            className="mt-3 text-sm text-[var(--accent)] hover:underline"
          >
            Create your first combo
          </button>
        </div>
      ) : (
        <ul className="space-y-2" role="list">
          {combos.map((c: any) => {
            const nModels = c.models?.length ?? 0
            const nConns = c.connectionIds?.length ?? 0
            const summaryParts: string[] = []
            if (nModels > 0) summaryParts.push(`${nModels} model${nModels !== 1 ? 's' : ''}`)
            summaryParts.push(
              nConns > 0
                ? `${nConns} pinned account${nConns !== 1 ? 's' : ''}`
                : 'Any active account',
            )

            return (
              <li
                key={c.id}
                className="rounded-xl border border-[var(--border)] bg-[var(--bg-card)] px-4 py-3 flex flex-col sm:flex-row sm:items-center gap-3"
              >
                <div className="min-w-0 flex-1">
                  <p className="font-medium truncate">{c.name}</p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">{summaryParts.join(' · ')}</p>
                  {nModels > 0 && (
                    <p className="text-[11px] text-[var(--text-muted)] font-mono truncate mt-1.5" title={c.models.join(', ')}>
                      {c.models.slice(0, 3).map((m: string) => getModelName(m)).join(' → ')}
                      {nModels > 3 ? ' …' : ''}
                    </p>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0 sm:pl-2">
                  <button
                    type="button"
                    onClick={() => startEdit(c)}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-[var(--text)]"
                  >
                    <Pencil size={14} /> Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => handleDelete(c.id)}
                    className="p-2 rounded-lg hover:bg-[var(--bg-hover)] text-[var(--danger)]"
                    title="Delete combo"
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
