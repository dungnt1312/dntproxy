import { useEffect, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { Plus, Trash2, Users, Layers, Pencil, Check, Loader2, X, ChevronRight, Link2, ArrowRight } from 'lucide-react'
import ModelSelector from '../components/ModelSelector'
import { getModelName } from '../models-config'
import { getProviderLabel } from '../components/connections/helpers'

export default function Combos() {
  const navigate = useNavigate()
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
    <div className="max-h-44 overflow-y-auto glass-sm divide-y divide-[var(--border)]">
      {conns.length === 0 && (
        <div className="px-3 py-3 text-center">
          <p className="text-xs text-[var(--text-dim)] mb-2">No connections available</p>
          <button
            onClick={() => navigate('/connections')}
            className="text-xs text-[var(--accent)] hover:text-[var(--accent-hover)] font-medium cursor-pointer flex items-center gap-1 mx-auto"
          >
            <Link2 size={10} /> Add connections <ChevronRight size={10} />
          </button>
        </div>
      )}
      {conns.map(c => {
        const isSelected = selected.includes(c.id)
        const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
        return (
          <label
            key={c.id}
            className={`flex items-center gap-3 px-3 py-2.5 cursor-pointer text-sm transition-colors ${
              isSelected ? 'bg-[var(--accent-glow)]' : 'hover:bg-white/[0.02]'
            }`}
          >
            <input
              type="checkbox"
              checked={isSelected}
              onChange={() => onToggle(c.id)}
              className="accent-[var(--accent)] w-3.5 h-3.5 shrink-0 cursor-pointer"
            />
            <span className="truncate font-medium min-w-0">{c.name}</span>
            <span className="chip chip-muted text-[10px] shrink-0">{getProviderLabel(c.provider)}</span>
            {isRL && (
              <span className="chip chip-warning text-[10px] shrink-0">RL</span>
            )}
            {!c.isActive && (
              <span className="chip chip-muted text-[10px] shrink-0">off</span>
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
            className="glass-input w-full"
          />
        </div>

        <details className="group glass-sm p-3 open:pb-3">
          <summary className="cursor-pointer list-none text-xs font-medium text-[var(--text-muted)] hover:text-[var(--text)] flex items-center gap-2 [&::-webkit-details-marker]:hidden transition-colors">
            <Users size={12} className="opacity-60 shrink-0" />
            <span>Pin specific connections</span>
            <span className="chip chip-muted text-[10px] ml-1">optional</span>
          </summary>
          <div className="pt-3 space-y-2">
            <ConnSelector
              selected={connIDs}
              onToggle={id => toggleConn(id, connIDs, setConnIDs)}
            />
            {connIDs.length > 0 && (
              <p className="text-xs text-[var(--text-dim)]">
                {allowedModels
                  ? `Model picker limited to ${allowedModels.length} model(s) from selected accounts.`
                  : 'Selected accounts have no model restriction — full catalog below.'}
              </p>
            )}
          </div>
        </details>

        <div>
          <label className="flex items-center gap-1.5 text-xs font-medium text-[var(--text-muted)] mb-1.5">
            <Layers size={12} className="opacity-60" />
            Models (fallback order)
          </label>
          <ModelSelector
            selected={models}
            onChange={setModels}
            allowedModels={allowedModels}
          />
        </div>

        {error && <p className="text-sm text-[var(--danger)]">{error}</p>}

        <div className="flex flex-wrap gap-2 pt-3 border-t border-[var(--border)]">
          <button
            type="button"
            onClick={onSave}
            disabled={loading}
            className="btn-primary"
          >
            {loading ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
            {loading ? 'Saving…' : saveLabel}
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="btn-ghost"
          >
            Cancel
          </button>
        </div>
      </div>
    )
  }

  const ModalChrome = ({
    title, subtitle, titleId, icon, onClose, children,
  }: { title: string; subtitle?: string; titleId: string; icon?: React.ReactNode; onClose: () => void; children: React.ReactNode }) => (
    <div
      className="modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      onClick={onClose}
    >
      <div
        className="modal-content sm:max-w-lg"
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <div className="flex items-center gap-3 min-w-0">
            {icon && (
              <div className="w-10 h-10 rounded-xl bg-[var(--purple-glow)] border border-[var(--purple)]/20 flex items-center justify-center shrink-0">
                {icon}
              </div>
            )}
            <div className="min-w-0">
              <h3 id={titleId} className="modal-title">{title}</h3>
              {subtitle && (
                <p className="modal-subtitle">{subtitle}</p>
              )}
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
        <div className="modal-body">
          {children}
        </div>
      </div>
    </div>
  )

  return (
    <div className="max-w-4xl">
      {/* Header */}
      <div className="page-header">
        <div>
          <h2 className="page-title">Combos</h2>
          <p className="page-subtitle">
            Named model fallback chains. Use the combo name as the model ID in your API calls.
          </p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="btn-primary shrink-0"
        >
          <Plus size={16} /> New combo
        </button>
      </div>

      {showAdd && (
        <ModalChrome
          title="New combo"
          subtitle="Pick a name, add models in order, optionally pin accounts."
          titleId="combo-dialog-new"
          icon={<Layers size={18} className="text-[var(--purple)]" />}
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
          icon={<Pencil size={16} className="text-[var(--purple)]" />}
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
        <div className="empty-state">
          <div className="empty-state-icon" style={{ background: 'var(--purple-glow)', color: 'var(--purple)' }}>
            <Layers size={24} />
          </div>
          <h3>No combos yet</h3>
          <p>
            Create a model fallback chain to automatically retry requests with different models.
          </p>
          <button
            type="button"
            onClick={openCreate}
            className="btn-primary"
          >
            <Plus size={16} /> Create your first combo
          </button>
        </div>
      ) : (
        <ul className="space-y-3" role="list">
          {combos.map((c: any) => {
            const nModels = c.models?.length ?? 0
            const nConns = c.connectionIds?.length ?? 0
            const pinnedConns = nConns > 0 
              ? c.connectionIds.map((id: string) => conns.find(cn => cn.id === id)).filter(Boolean)
              : []

            return (
              <li
                key={c.id}
                className="glass p-4 flex flex-col sm:flex-row sm:items-center gap-3 transition-all hover:border-[var(--border-hover)] group"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <p className="font-semibold truncate" style={{ fontFamily: 'var(--font-heading)' }}>{c.name}</p>
                    <span className="chip chip-muted text-[10px]">
                      {nModels} model{nModels !== 1 ? 's' : ''}
                    </span>
                    {nConns > 0 && (
                      <span className="chip chip-accent text-[10px]">
                        <Link2 size={8} /> {nConns} pinned
                      </span>
                    )}
                  </div>

                  {/* Model chain visualization */}
                  {nModels > 0 && (
                    <div className="flex items-center gap-1 mt-2 flex-wrap">
                      {c.models.slice(0, 4).map((m: string, i: number) => (
                        <span key={i} className="flex items-center gap-1">
                          <span className="px-2 py-0.5 rounded-md text-[10px] font-mono bg-white/[0.04] text-[var(--text-muted)] border border-[var(--border)]">
                            {getModelName(m)}
                          </span>
                          {i < Math.min(nModels, 4) - 1 && (
                            <ArrowRight size={10} className="text-[var(--text-dim)]" />
                          )}
                        </span>
                      ))}
                      {nModels > 4 && (
                        <span className="text-[10px] text-[var(--text-dim)]">+{nModels - 4} more</span>
                      )}
                    </div>
                  )}

                  {/* Pinned connections */}
                  {pinnedConns.length > 0 && (
                    <div className="flex items-center gap-1.5 mt-2 flex-wrap">
                      <span className="text-[10px] text-[var(--text-dim)]">via:</span>
                      {pinnedConns.slice(0, 3).map((cn: any) => (
                        <span key={cn.id} className="chip chip-muted text-[10px]">
                          {cn.name}
                        </span>
                      ))}
                      {pinnedConns.length > 3 && (
                        <span className="text-[10px] text-[var(--text-dim)]">+{pinnedConns.length - 3}</span>
                      )}
                    </div>
                  )}
                </div>

                <div className="flex items-center gap-1 shrink-0 sm:pl-2 opacity-60 group-hover:opacity-100 transition-opacity">
                  <button
                    type="button"
                    onClick={() => startEdit(c)}
                    className="btn-ghost text-xs px-3 py-1.5"
                  >
                    <Pencil size={13} /> Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => handleDelete(c.id)}
                    className="btn-icon text-[var(--danger)] hover:bg-[var(--danger-glow)]"
                    title="Delete combo"
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
              </li>
            )
          })}
        </ul>
      )}

      {/* Quick links */}
      {combos.length > 0 && (
        <div className="glass-sm p-4 mt-6 flex items-center justify-between text-xs text-[var(--text-muted)]">
          <span>Tip: Use combo name as model ID in your API calls, e.g. <code className="bg-white/[0.05] px-1.5 py-0.5 rounded text-[var(--accent)] font-mono">{combos[0]?.name}</code></span>
          <button 
            onClick={() => navigate('/models')}
            className="text-[var(--accent)] hover:text-[var(--accent-hover)] font-medium flex items-center gap-1 cursor-pointer"
          >
            View all models <ChevronRight size={12} />
          </button>
        </div>
      )}
    </div>
  )
}
