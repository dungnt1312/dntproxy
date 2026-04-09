import { useEffect, useState } from 'react'
import { api } from '../api'
import { Plus, Trash2 } from 'lucide-react'

export default function Combos() {
  const [combos, setCombos] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [modelsStr, setModelsStr] = useState('')
  const [error, setError] = useState('')

  const load = () => api.getCombos().then(data => setCombos(data || [])).catch(() => {})
  useEffect(() => { load() }, [])

  const handleAdd = async () => {
    setError('')
    const models = modelsStr.split('\n').map(s => s.trim()).filter(Boolean)
    if (!name || models.length === 0) { setError('Name and at least 1 model required'); return }
    try {
      await api.createCombo({ name, models })
      setShowAdd(false)
      setName('')
      setModelsStr('')
      load()
    } catch (e: any) { setError(e.message) }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Remove this combo?')) return
    await api.deleteCombo(id)
    load()
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold">Combos</h2>
        <button onClick={() => setShowAdd(!showAdd)} className="flex items-center gap-1.5 px-3 py-1.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">
          <Plus size={16} /> New Combo
        </button>
      </div>

      {showAdd && (
        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] mb-6 space-y-3">
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">Combo Name</label>
            <input value={name} onChange={e => setName(e.target.value)} placeholder="my-combo" className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
          </div>
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">Models (one per line, in fallback order)</label>
            <textarea value={modelsStr} onChange={e => setModelsStr(e.target.value)} rows={4} placeholder={"kr/claude-sonnet-4.5\nkr/claude-haiku-4.5"} className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none resize-none" />
          </div>
          {error && <p className="text-sm text-[var(--danger)]">{error}</p>}
          <button onClick={handleAdd} className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">Create</button>
        </div>
      )}

      {combos.length === 0 ? (
        <p className="text-[var(--text-muted)]">No combos configured.</p>
      ) : (
        <div className="space-y-3">
          {combos.map((c: any) => (
            <div key={c.id} className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)]">
              <div className="flex items-center justify-between mb-2">
                <span className="font-medium">{c.name}</span>
                <button onClick={() => handleDelete(c.id)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg transition-colors">
                  <Trash2 size={14} className="text-[var(--danger)]" />
                </button>
              </div>
              <div className="space-y-1">
                {c.models?.map((m: string, i: number) => (
                  <div key={i} className="text-xs text-[var(--text-muted)] flex items-center gap-2">
                    <span className="w-5 text-right text-[var(--accent)]">{i + 1}.</span>
                    <span className="font-mono">{m}</span>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
