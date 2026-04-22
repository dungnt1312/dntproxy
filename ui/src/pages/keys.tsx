import { useEffect, useState } from 'react'
import { api } from '../api'
import { Plus, Trash2, Copy, Check, KeyRound, X } from 'lucide-react'

interface APIKeyItem {
  id: string
  name: string
  key: string
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
      <div className="page-header">
        <div>
          <h2 className="page-title">API Keys</h2>
          <p className="page-subtitle">Generate keys to authenticate requests to the proxy.</p>
        </div>
        <button onClick={() => { setShowAdd(!showAdd); setNewKey('') }} className="btn-primary">
          <Plus size={16} /> Generate Key
        </button>
      </div>

      {newKey && (
        <div className="glass p-4 mb-6 border-[var(--success)]/20 animate-slide-up">
          <p className="text-sm text-[var(--success)] mb-2 font-medium">New API key generated. Copy it now — it won't be shown again.</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 glass-input font-mono text-xs break-all py-2">{newKey}</code>
            <button
              onClick={() => handleCopy(newKey, 'new')}
              aria-label="Copy new API key"
              className="btn-icon"
            >
              {copied === 'new' ? <Check size={16} className="text-[var(--success)]" /> : <Copy size={16} />}
            </button>
          </div>
        </div>
      )}

      {showAdd && (
        <div className="glass p-4 mb-6 animate-slide-up">
          <div className="flex gap-3 items-end">
            <div className="flex-1">
              <label className="block text-xs text-[var(--text-muted)] mb-1.5 font-medium">Key Name</label>
              <input value={name} onChange={e => setName(e.target.value)} placeholder="my-key" className="glass-input w-full" />
            </div>
            <button onClick={handleCreate} className="btn-primary py-[10px]">Generate</button>
            <button onClick={() => setShowAdd(false)} className="btn-icon"><X size={16} /></button>
          </div>
        </div>
      )}

      {keys.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon" style={{ background: 'var(--warning-glow)', color: 'var(--warning)' }}>
            <KeyRound size={24} />
          </div>
          <h3>No API keys</h3>
          <p>Generate a key to authenticate requests to your proxy server.</p>
          <button onClick={() => setShowAdd(true)} className="btn-primary">
            <Plus size={16} /> Generate Key
          </button>
        </div>
      ) : (
        <div className="space-y-2">
          {keys.map((k) => (
            <div key={k.id} className="glass-sm px-4 py-3 flex items-center justify-between hover:border-[var(--border-hover)] transition-all group">
              <div className="flex items-center gap-3">
                <span className="font-medium text-sm">{k.name}</span>
                <code className="text-[11px] text-[var(--text-dim)] bg-white/[0.03] px-2 py-0.5 rounded font-mono">{k.key}</code>
                <span className={`chip text-[10px] ${k.isActive ? 'chip-success' : 'chip-danger'}`}>
                  {k.isActive ? 'active' : 'disabled'}
                </span>
              </div>
              <button
                onClick={() => handleDelete(k.id)}
                aria-label={`Remove API key ${k.name}`}
                className="btn-icon text-[var(--danger)] opacity-0 group-hover:opacity-100 transition-opacity hover:bg-[var(--danger-glow)]"
                title="Remove"
              >
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
