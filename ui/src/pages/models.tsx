import { useEffect, useState } from 'react'
import { api } from '../api'
import { Box, Plus, Trash2, Cpu, Zap } from 'lucide-react'

export default function Models() {
  const [models, setModels] = useState<any[]>([])
  const [aliases, setAliases] = useState<Record<string, string>>({})
  const [showAdd, setShowAdd] = useState(false)
  const [alias, setAlias] = useState('')
  const [model, setModel] = useState('')

  const load = () => {
    api.getModels().then(data => setModels(data || [])).catch(() => {})
    api.getAliases().then(data => setAliases(data || {})).catch(() => {})
  }
  useEffect(() => { load() }, [])

  const handleAddAlias = async () => {
    if (!alias || !model) return
    await api.setAlias(alias, model)
    setShowAdd(false)
    setAlias('')
    setModel('')
    load()
  }

  const handleDeleteAlias = async (name: string) => {
    await api.deleteAlias(name)
    load()
  }

  const kiroModels = models.filter((m: any) => m.provider === 'kiro')
  const openaiModels = models.filter((m: any) => m.provider === 'openai')
  const comboModels = models.filter((m: any) => m.provider === 'combo')
  const aliasModels = models.filter((m: any) => m.provider === 'alias')

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Models</h2>

      {/* Kiro Models */}
      <h3 className="text-lg font-semibold mb-3 flex items-center gap-2"><Box size={18} className="text-green-400" /> Kiro Models</h3>
      <div className="grid grid-cols-2 gap-2 mb-6">
        {kiroModels.map((m: any) => (
          <div key={m.id} className="bg-[var(--bg-card)] rounded-lg p-3 border border-[var(--border)]">
            <span className="font-mono text-sm">{m.id}</span>
            <span className="text-xs text-[var(--text-muted)] ml-2">{m.name}</span>
          </div>
        ))}
      </div>

      {/* OpenAI Models */}
      {openaiModels.length > 0 && (
        <>
          <h3 className="text-lg font-semibold mb-3 flex items-center gap-2"><Cpu size={18} className="text-emerald-400" /> OpenAI Models</h3>
          <div className="grid grid-cols-2 gap-2 mb-6">
            {openaiModels.map((m: any) => (
              <div key={m.id} className="bg-[var(--bg-card)] rounded-lg p-3 border border-[var(--border)]">
                <span className="font-mono text-sm">{m.id}</span>
                <span className="text-xs text-[var(--text-muted)] ml-2">{m.name}</span>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Combo Models */}
      {comboModels.length > 0 && (
        <>
          <h3 className="text-lg font-semibold mb-3 flex items-center gap-2"><Zap size={18} className="text-amber-400" /> Combo Models</h3>
          <div className="grid grid-cols-2 gap-2 mb-6">
            {comboModels.map((m: any) => (
              <div key={m.id} className="bg-[var(--bg-card)] rounded-lg p-3 border border-[var(--border)]">
                <span className="font-mono text-sm">{m.id}</span>
                <span className="text-xs text-[var(--text-muted)] ml-2">{m.models?.length} models</span>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Aliases */}
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-lg font-semibold">Aliases</h3>
        <button onClick={() => setShowAdd(!showAdd)} className="flex items-center gap-1.5 px-3 py-1.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">
          <Plus size={16} /> Add Alias
        </button>
      </div>

      {showAdd && (
        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] mb-4 flex gap-3 items-end">
          <div className="flex-1">
            <label className="block text-xs text-[var(--text-muted)] mb-1">Alias</label>
            <input value={alias} onChange={e => setAlias(e.target.value)} placeholder="sonnet" className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm" />
          </div>
          <div className="flex-1">
            <label className="block text-xs text-[var(--text-muted)] mb-1">Model</label>
            <input value={model} onChange={e => setModel(e.target.value)} placeholder="kr/claude-sonnet-4.5" className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm" />
          </div>
          <button onClick={handleAddAlias} className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">Save</button>
        </div>
      )}

      {Object.keys(aliases).length === 0 && aliasModels.length === 0 ? (
        <p className="text-[var(--text-muted)] text-sm">No aliases configured.</p>
      ) : (
        <div className="space-y-2">
          {Object.entries(aliases).map(([a, m]) => (
            <div key={a} className="bg-[var(--bg-card)] rounded-lg p-3 border border-[var(--border)] flex items-center justify-between">
              <div>
                <span className="font-mono text-sm">{a}</span>
                <span className="text-[var(--text-muted)] mx-2">→</span>
                <span className="font-mono text-sm text-[var(--text-muted)]">{m}</span>
              </div>
              <button onClick={() => handleDeleteAlias(a)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg transition-colors">
                <Trash2 size={14} className="text-[var(--danger)]" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
