import { useEffect, useState } from 'react'
import { api } from '../api'
import { Plus, Trash2, Copy, Check } from 'lucide-react'

interface APIKeyItem {
  id: string
  name: string
  keyMasked: string
  isActive: boolean
  createdAt: string
}

export default function Keys() {
  const [keys, setKeys] = useState<APIKeyItem[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [newKey, setNewKey] = useState('')
  const [copied, setCopied] = useState('')

  const load = () => api.getKeys().then(data => setKeys(data || [])).catch(() => {})
  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    if (!name) return
    const res = await api.createKey(name)
    setNewKey(res.key)
    setName('')
    setShowAdd(false)
    load()
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Remove this API key?')) return
    await api.deleteKey(id)
    load()
  }

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text)
    setCopied(id)
    setTimeout(() => setCopied(''), 2000)
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold">API Keys</h2>
        <button onClick={() => { setShowAdd(!showAdd); setNewKey('') }} className="flex items-center gap-1.5 px-3 py-1.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">
          <Plus size={16} /> Generate Key
        </button>
      </div>

      {newKey && (
        <div className="bg-green-900/20 border border-green-800 rounded-xl p-4 mb-6">
          <p className="text-sm text-green-400 mb-2">New API key generated. Copy it now — it won't be shown again.</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-[var(--bg)] px-3 py-2 rounded-lg text-sm font-mono break-all">{newKey}</code>
            <button
              onClick={() => handleCopy(newKey, 'new')}
              aria-label="Copy new API key"
              className="p-2 hover:bg-[var(--bg-hover)] rounded-lg transition-colors"
            >
              {copied === 'new' ? <Check size={16} className="text-green-400" /> : <Copy size={16} />}
            </button>
          </div>
        </div>
      )}

      {showAdd && (
        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] mb-6 flex gap-3 items-end">
          <div className="flex-1">
            <label className="block text-xs text-[var(--text-muted)] mb-1">Key Name</label>
            <input value={name} onChange={e => setName(e.target.value)} placeholder="my-key" className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
          </div>
          <button onClick={handleCreate} className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">Generate</button>
        </div>
      )}

      {keys.length === 0 ? (
        <p className="text-[var(--text-muted)]">No API keys.</p>
      ) : (
        <div className="space-y-2">
          {keys.map((k) => (
            <div key={k.id} className="bg-[var(--bg-card)] rounded-lg p-3 border border-[var(--border)] flex items-center justify-between">
              <div className="flex items-center gap-3">
                <span className="font-medium text-sm">{k.name}</span>
                <code className="text-xs text-[var(--text-muted)] bg-[var(--bg)] px-2 py-0.5 rounded">{k.keyMasked}</code>
                <span className={`text-xs px-2 py-0.5 rounded ${k.isActive ? 'bg-green-900/40 text-green-400' : 'bg-red-900/40 text-red-400'}`}>
                  {k.isActive ? 'active' : 'disabled'}
                </span>
              </div>
              <div className="flex items-center gap-1">
                <button
                  onClick={() => handleDelete(k.id)}
                  aria-label={`Remove API key ${k.name}`}
                  className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg transition-colors"
                  title="Remove"
                >
                  <Trash2 size={14} className="text-[var(--danger)]" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
