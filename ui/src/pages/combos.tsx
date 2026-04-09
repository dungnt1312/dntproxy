import { useEffect, useState, useCallback } from 'react'
import { api } from '../api'
import { Plus, Trash2, Users, Layers, ChevronDown, ChevronUp, Pencil, Check, X, Loader2 } from 'lucide-react'

export default function Combos() {
  const [combos, setCombos] = useState<any[]>([])
  const [conns, setConns] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [modelsStr, setModelsStr] = useState('')
  const [selectedConnIDs, setSelectedConnIDs] = useState<string[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const [editModels, setEditModels] = useState('')
  const [editConnIDs, setEditConnIDs] = useState<string[]>([])
  const [expandedEdit, setExpandedEdit] = useState<string | null>(null)

  const load = useCallback(() => {
    api.getCombos().then(d => setCombos(d || [])).catch(() => {})
    api.getConnections().then(d => setConns(d || [])).catch(() => {})
  }, [])

  useEffect(() => { load() }, [load])

  const parseModels = (s: string) => s.split('\n').map(x => x.trim()).filter(Boolean)

  const handleAdd = async () => {
    setError(''); setLoading(true)
    const models = parseModels(modelsStr)
    if (!name) { setError('Name is required'); setLoading(false); return }
    if (models.length === 0 && selectedConnIDs.length === 0) {
      setError('Add at least one model or select a connection'); setLoading(false); return
    }
    try {
      await api.createCombo({
        name,
        models: models.length > 0 ? models : undefined,
        connectionIds: selectedConnIDs.length > 0 ? selectedConnIDs : undefined,
      })
      setShowAdd(false); setName(''); setModelsStr(''); setSelectedConnIDs([]); load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Remove this combo?')) return
    await api.deleteCombo(id); load()
  }

  const startEdit = (c: any) => {
    setEditingId(c.id)
    setEditName(c.name)
    setEditModels((c.models || []).join('\n'))
    setEditConnIDs(c.connectionIds || [])
    setExpandedEdit(c.id)
  }

  const saveEdit = async (id: string) => {
    setLoading(true)
    try {
      await api.updateCombo(id, {
        name: editName,
        models: parseModels(editModels),
        setModels: true,
        connectionIds: editConnIDs,
        setConnections: true,
      })
      setEditingId(null); setExpandedEdit(null); load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const toggleConn = (id: string, arr: string[], setArr: (v: string[]) => void) => {
    setArr(arr.includes(id) ? arr.filter(x => x !== id) : [...arr, id])
  }

  const getConnName = (id: string) => conns.find(c => c.id === id)?.name ?? id

  // Connection selector component
  const ConnSelector = ({
    selected, onToggle
  }: { selected: string[]; onToggle: (id: string) => void }) => (
    <div className="space-y-1.5 max-h-48 overflow-y-auto">
      {conns.length === 0 && <p className="text-xs text-[var(--text-muted)] italic">No connections</p>}
      {conns.map(c => (
        <label key={c.id} className={`flex items-center gap-2.5 px-2.5 py-2 rounded-lg cursor-pointer transition-colors ${selected.includes(c.id) ? 'bg-[var(--accent)]/10 border border-[var(--accent)]/40' : 'bg-[var(--bg)] border border-transparent hover:border-[var(--border)]'}`}>
          <input type="checkbox" checked={selected.includes(c.id)} onChange={() => onToggle(c.id)} className="accent-[var(--accent)] w-3.5 h-3.5" />
          <div className="min-w-0">
            <div className="flex items-center gap-1.5 flex-wrap">
              <span className="text-sm font-medium truncate">{c.name}</span>
              <span className="text-xs text-[var(--text-muted)] bg-[var(--bg)] px-1.5 py-0.5 rounded">{c.provider}</span>
              {c.email && <span className="text-xs text-[var(--text-muted)] truncate">{c.email}</span>}
            </div>
            {/* Token bar mini */}
            {c.expiresAt && c.expiresIn && (() => {
              const pct = Math.max(0, Math.min(100, (new Date(c.expiresAt).getTime() - Date.now()) / (c.expiresIn * 1000) * 100))
              const color = pct < 15 ? '#f87171' : pct < 40 ? '#fbbf24' : '#4ade80'
              return (
                <div className="h-1 bg-[var(--border)] rounded-full mt-1 w-32">
                  <div className="h-full rounded-full" style={{ width: `${pct}%`, backgroundColor: color }} />
                </div>
              )
            })()}
          </div>
        </label>
      ))}
    </div>
  )

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold">Combos</h2>
        <button onClick={() => { setShowAdd(!showAdd); setError('') }}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">
          <Plus size={16} /> New Combo
        </button>
      </div>

      {/* Add form */}
      {showAdd && (
        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] mb-6 space-y-4">
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">Combo Name</label>
            <input value={name} onChange={e => setName(e.target.value)} placeholder="my-combo"
              className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
          </div>

          {/* Connections */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-2 flex items-center gap-1.5">
              <Users size={12} /> Accounts (optional — limit to specific connections)
            </label>
            <ConnSelector selected={selectedConnIDs} onToggle={id => toggleConn(id, selectedConnIDs, setSelectedConnIDs)} />
            {selectedConnIDs.length > 0 && (
              <p className="text-xs text-[var(--accent)] mt-1">✓ {selectedConnIDs.length} account{selectedConnIDs.length > 1 ? 's' : ''} selected</p>
            )}
          </div>

          {/* Models */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1 flex items-center gap-1.5">
              <Layers size={12} /> Models (one per line, in fallback order)
            </label>
            <textarea value={modelsStr} onChange={e => setModelsStr(e.target.value)} rows={4}
              placeholder={"kr/claude-sonnet-4.5\nkr/claude-haiku-4.5"}
              className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none resize-none" />
            <p className="text-xs text-[var(--text-muted)] mt-1">Empty = any model via selected accounts</p>
          </div>

          {error && <p className="text-sm text-[var(--danger)]">{error}</p>}
          <button onClick={handleAdd} disabled={loading}
            className="flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50 transition-colors">
            {loading ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />}
            {loading ? 'Creating…' : 'Create Combo'}
          </button>
        </div>
      )}

      {/* Combo list */}
      {combos.length === 0 ? (
        <p className="text-[var(--text-muted)]">No combos configured.</p>
      ) : (
        <div className="space-y-3">
          {combos.map((c: any) => {
            const isEditing = editingId === c.id
            const isExpanded = expandedEdit === c.id
            const hasConns = (c.connectionIds?.length ?? 0) > 0

            return (
              <div key={c.id} className="bg-[var(--bg-card)] rounded-xl border border-[var(--border)] overflow-hidden">
                {/* Header row */}
                <div className="flex items-center justify-between px-4 py-3 gap-3">
                  <div className="flex items-center gap-2 min-w-0 flex-1">
                    {isEditing ? (
                      <input value={editName} onChange={e => setEditName(e.target.value)}
                        className="bg-[var(--bg)] border border-[var(--accent)] rounded px-2 py-0.5 text-sm font-medium outline-none w-48"
                        onKeyDown={e => { if (e.key === 'Enter') saveEdit(c.id); if (e.key === 'Escape') setEditingId(null) }} />
                    ) : (
                      <span className="font-medium">{c.name}</span>
                    )}
                    {hasConns && (
                      <span className="text-xs bg-[var(--accent)]/10 text-[var(--accent)] px-1.5 py-0.5 rounded flex items-center gap-1">
                        <Users size={10} /> {c.connectionIds.length} acct
                      </span>
                    )}
                    <span className="text-xs text-[var(--text-muted)]">
                      {(c.models?.length ?? 0)} model{c.models?.length !== 1 ? 's' : ''}
                    </span>
                  </div>

                  <div className="flex items-center gap-1">
                    {isEditing ? (
                      <>
                        <button onClick={() => saveEdit(c.id)} disabled={loading} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg text-[var(--success)]">
                          {loading ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                        </button>
                        <button onClick={() => { setEditingId(null); setExpandedEdit(null) }} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg text-[var(--text-muted)]">
                          <X size={14} />
                        </button>
                      </>
                    ) : (
                      <button onClick={() => startEdit(c)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg text-[var(--text-muted)]" title="Edit">
                        <Pencil size={14} />
                      </button>
                    )}
                    <button onClick={() => setExpandedEdit(isExpanded && !isEditing ? null : c.id)}
                      className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg text-[var(--text-muted)]">
                      {isExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    </button>
                    <button onClick={() => handleDelete(c.id)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg">
                      <Trash2 size={14} className="text-[var(--danger)]" />
                    </button>
                  </div>
                </div>

                {/* Expanded / edit panel */}
                {isExpanded && (
                  <div className="border-t border-[var(--border)] px-4 py-3 space-y-4">
                    {isEditing ? (
                      <>
                        {/* Edit: account selector */}
                        <div>
                          <label className="block text-xs text-[var(--text-muted)] mb-2 flex items-center gap-1.5">
                            <Users size={12} /> Accounts
                          </label>
                          <ConnSelector selected={editConnIDs} onToggle={id => toggleConn(id, editConnIDs, setEditConnIDs)} />
                        </div>
                        {/* Edit: models */}
                        <div>
                          <label className="block text-xs text-[var(--text-muted)] mb-1 flex items-center gap-1.5">
                            <Layers size={12} /> Models (one per line)
                          </label>
                          <textarea value={editModels} onChange={e => setEditModels(e.target.value)} rows={5}
                            className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-xs font-mono focus:border-[var(--accent)] outline-none resize-none" />
                        </div>
                      </>
                    ) : (
                      <>
                        {/* View: accounts */}
                        {hasConns && (
                          <div>
                            <p className="text-xs text-[var(--text-muted)] mb-2 flex items-center gap-1"><Users size={11} /> Pinned Accounts</p>
                            <div className="flex flex-wrap gap-1.5">
                              {c.connectionIds.map((id: string) => (
                                <span key={id} className="text-xs bg-[var(--accent)]/10 text-[var(--accent)] px-2 py-0.5 rounded font-medium">
                                  {getConnName(id)}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}
                        {/* View: models */}
                        {(c.models?.length ?? 0) > 0 && (
                          <div>
                            <p className="text-xs text-[var(--text-muted)] mb-2 flex items-center gap-1"><Layers size={11} /> Models (fallback order)</p>
                            <div className="space-y-1">
                              {c.models.map((m: string, i: number) => (
                                <div key={i} className="text-xs text-[var(--text-muted)] flex items-center gap-2">
                                  <span className="w-5 text-right text-[var(--accent)] font-mono">{i + 1}.</span>
                                  <span className="font-mono text-[var(--text)]">{m}</span>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                        {!hasConns && (c.models?.length ?? 0) === 0 && (
                          <p className="text-xs text-[var(--text-muted)] italic">No models or accounts configured</p>
                        )}
                      </>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
