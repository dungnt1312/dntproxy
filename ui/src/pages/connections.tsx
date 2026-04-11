import { useEffect, useState, useMemo } from 'react'
import { api } from '../api'
import {
  Plus, Search, Link2, AlertTriangle, ChevronDown
} from 'lucide-react'
import { getProviderLabel } from '../components/connections/helpers'
import ConnectionCard from '../components/connections/ConnectionCard'
import AddConnectionModal from '../components/connections/AddConnectionModal'
import EditModelsModal from '../components/connections/EditModelsModal'
import DeleteDialog from '../components/connections/DeleteDialog'
import { AwsLogo, OpenAILogo, CustomLogo } from '../components/connections/helpers'

export default function Connections() {
  const [conns, setConns] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [editModelsConn, setEditModelsConn] = useState<any | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null)
  const [success, setSuccess] = useState('')

  const [quotaResult, setQuotaResult] = useState<Record<string, any>>({})
  const [searchQuery, setSearchQuery] = useState('')
  const [autoRefreshQuota, setAutoRefreshQuota] = useState(false)
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({})

  const toggleGroup = (id: string) => {
    setCollapsedGroups(prev => ({ ...prev, [id]: !prev[id] }))
  }

  // ── Stats ──────────────────────────────────────────────────────────────────
  const connectionStats = useMemo(() => {
    const total = conns.length
    const active = conns.filter((c: any) => c.isActive).length
    const needsAttention = conns.filter((c: any) => {
      if (!c.isActive) return false
      const rl = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
      const exp = c.expiresAt && new Date(c.expiresAt) < new Date()
      return rl || exp || (c.backoffLevel ?? 0) > 0 || !!c.lastError
    }).length
    return { total, active, needsAttention }
  }, [conns])

  // ── Filter & Sort ──────────────────────────────────────────────────────────
  const filteredConns = useMemo(() => {
    let list = conns
    const q = searchQuery.trim().toLowerCase()
    if (!q) return list
    return list.filter((c: any) => {
      const providerLabel = getProviderLabel(c.provider).toLowerCase()
      const hay = [
        c.name, c.email, c.baseUrl, c.providerName, c.authMethod, providerLabel,
        ...(c.supportedModels || []),
      ].filter(Boolean).join(' ').toLowerCase()
      return hay.includes(q)
    })
  }, [conns, searchQuery])

  const groupedConns = useMemo(() => {
    const list = [...filteredConns]
    list.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
    
    const groups: Record<string, any[]> = {}
    list.forEach((c: any) => {
      const p = c.provider === 'openai-compatible' ? 'other' : c.provider
      if (!groups[p]) groups[p] = []
      groups[p].push(c)
    })

    const result: any[] = []
    if (groups['kiro']) result.push({ id: 'kiro', label: 'AWS / Kiro', items: groups['kiro'], icon: <AwsLogo size={16} />, color: '#FF9900' })
    if (groups['openai']) result.push({ id: 'openai', label: 'OpenAI', items: groups['openai'], icon: <OpenAILogo size={16} />, color: '#10a37f' })
    if (groups['other']) result.push({ id: 'other', label: 'Custom API', items: groups['other'], icon: <CustomLogo size={16} />, color: '#a855f7' })
    
    // Put any unknown providers in "other"
    Object.keys(groups).forEach(k => {
      if (!['kiro', 'openai', 'other'].includes(k)) {
        if (!result.find(r => r.id === 'other')) result.push({ id: 'other', label: 'Custom API', items: [], icon: <CustomLogo size={16} />, color: '#a855f7' })
        const otherGroup = result.find(r => r.id === 'other')
        if (otherGroup) otherGroup.items.push(...groups[k])
      }
    })

    return result
  }, [filteredConns])

  // ── Load & Auto-Refresh ────────────────────────────────────────────────────
  const load = () => api.getConnections().then(d => {
    setConns(d || [])
    d?.forEach((c: any) => {
      if (c.isActive) {
        api.getUsage(c.id)
          .then(res => setQuotaResult(prev => ({ ...prev, [c.id]: res })))
          .catch(e => setQuotaResult(prev => ({ ...prev, [c.id]: { error: e.message } })))
      }
    })
  }).catch(() => {})

  useEffect(() => { load() }, [])

  useEffect(() => {
    if (!autoRefreshQuota) return
    const t = setInterval(() => {
      conns.forEach((c: any) => {
        if (c.isActive) {
          api.getUsage(c.id)
            .then(res => setQuotaResult(prev => ({ ...prev, [c.id]: res })))
            .catch(e => setQuotaResult(prev => ({ ...prev, [c.id]: { error: e.message } })))
        }
      })
    }, 10000)
    return () => clearInterval(t)
  }, [autoRefreshQuota, conns])

  useEffect(() => {
    if (!success) return
    const t = setTimeout(() => setSuccess(''), 4000)
    return () => clearTimeout(t)
  }, [success])

  // ── Handlers ───────────────────────────────────────────────────────────────
  const handleAddSuccess = (msg: string) => {
    setShowAdd(false)
    setSuccess(msg)
    load()
  }

  const handleDeleteConfirm = async (id: string) => {
    await api.deleteConnection(id)
    load()
  }

  const handleModelsEditSave = () => {
    setEditModelsConn(null)
    load()
  }

  return (
    <div>
      {success && (
        <div className="toast">
          <div className="toast-content toast-success">
            <span className="w-2 h-2 rounded-full bg-[var(--success)]" />
            {success}
          </div>
        </div>
      )}

      {/* Header */}
      <div className="page-header">
        <div className="min-w-0">
          <h2 className="page-title">Connections</h2>
          <p className="page-subtitle">
            Manage your AI provider accounts. Connect Kiro, OpenAI or any compatible endpoints.
          </p>
        </div>
        <button type="button" onClick={() => setShowAdd(true)} className="btn-primary shrink-0">
          <Plus size={16} /> Add Connection
        </button>
      </div>

      {/* Summary Cards */}
      {conns.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
          <div className="glass stat-card stat-card-blue cursor-default">
            <div className="text-xs text-[var(--text-muted)] font-medium mb-1 flex items-center gap-1.5">
              <Link2 size={12} className="opacity-50" /> Total Connections
            </div>
            <div className="text-2xl font-bold" style={{ fontFamily: 'var(--font-heading)' }}>{connectionStats.total}</div>
          </div>
          <div className="glass stat-card stat-card-green cursor-default">
            <div className="text-xs text-[var(--success)] font-medium mb-1 flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-[var(--success)]" /> Active
            </div>
            <div className="text-2xl font-bold text-[var(--success)]" style={{ fontFamily: 'var(--font-heading)' }}>{connectionStats.active}</div>
          </div>
          <div className={`glass stat-card cursor-default ${connectionStats.needsAttention > 0 ? 'stat-card-danger' : ''}`}>
            <div className={`text-xs font-medium mb-1 flex items-center gap-1.5 ${connectionStats.needsAttention > 0 ? 'text-[var(--danger)]' : 'text-[var(--text-dim)]'}`}>
              <AlertTriangle size={12} className={connectionStats.needsAttention > 0 ? '' : 'opacity-40'} /> Issues
            </div>
            <div className={`text-2xl font-bold ${connectionStats.needsAttention > 0 ? 'text-[var(--danger)]' : 'text-[var(--text-dim)]'}`} style={{ fontFamily: 'var(--font-heading)' }}>
              {connectionStats.needsAttention}
            </div>
          </div>
        </div>
      )}

      {/* Toolbar */}
      {conns.length > 0 && (
        <div className="glass-sm p-3 mb-6">
          <div className="flex flex-col sm:flex-row sm:items-center gap-3">
            <div className="relative flex-1 min-w-0">
              <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-dim)] pointer-events-none" />
              <input
                type="search"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                placeholder="Search by name, email, or model…"
                className="glass-input w-full pl-9"
              />
            </div>
            <div className="flex items-center gap-4 shrink-0">
              <label className="flex cursor-pointer items-center gap-2 text-xs text-[var(--text-muted)] hover:text-[var(--text)] transition-colors select-none font-medium">
                <input
                  type="checkbox"
                  checked={autoRefreshQuota}
                  onChange={e => setAutoRefreshQuota(e.target.checked)}
                  className="accent-[var(--accent)] w-3.5 h-3.5 rounded cursor-pointer"
                />
                Auto refresh Quotas
              </label>
            </div>
          </div>
        </div>
      )}

      {/* List */}
      {conns.length === 0 ? (
        <div className="empty-state py-12">
          <div className="empty-state-icon">
            <Link2 size={24} strokeWidth={2} />
          </div>
          <h3>No connections yet</h3>
          <p>
            Add Kiro AI or OpenAI accounts to begin routing requests.
          </p>
          <button type="button" onClick={() => setShowAdd(true)} className="btn-primary mt-2">
            <Plus size={16} /> Connect Now
          </button>
        </div>
      ) : filteredConns.length === 0 ? (
        <div className="glass p-12 text-center">
          <p className="text-sm text-[var(--text-muted)]">No connections matching "{searchQuery.trim()}"</p>
          <button type="button" onClick={() => setSearchQuery('')} className="mt-3 text-sm font-medium text-[var(--accent)] hover:text-[var(--accent-hover)] cursor-pointer">
            Clear filters
          </button>
        </div>
      ) : (
        <div className="space-y-6">
          {groupedConns.map(group => {
            const isCollapsed = collapsedGroups[group.id]
            return (
            <div key={group.id} className="animate-fade-in">
              <div 
                className="flex items-center gap-2 mb-3 px-1 cursor-pointer select-none group/col"
                onClick={() => toggleGroup(group.id)}
              >
                <div className="w-7 h-7 rounded-lg flex items-center justify-center transition-transform group-hover/col:scale-105" style={{ backgroundColor: `${group.color}15`, border: `1px solid ${group.color}25` }}>
                  {group.icon}
                </div>
                <h3 className="text-sm font-semibold" style={{ fontFamily: 'var(--font-heading)' }}>{group.label}</h3>
                <span className="chip chip-muted text-[10px]">{group.items.length}</span>
                <div className="ml-auto">
                  <ChevronDown className={`h-4 w-4 text-[var(--text-muted)] transition-transform ${isCollapsed ? '-rotate-90' : ''}`} />
                </div>
              </div>
              
              {!isCollapsed && (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                  {group.items.map((c: any) => (
                    <ConnectionCard
                      key={c.id}
                      conn={c}
                      initialQuotaResult={quotaResult[c.id]}
                      onReload={load}
                      onDelete={(id, name) => setDeleteTarget({ id, name })}
                      onEditModels={setEditModelsConn}
                    />
                  ))}
                </div>
              )}
            </div>
            )
          })}
        </div>
      )}

      {showAdd && (
        <AddConnectionModal
          onSuccess={handleAddSuccess}
          onClose={() => setShowAdd(false)}
        />
      )}

      {editModelsConn && (
        <EditModelsModal
          conn={editModelsConn}
          onSave={handleModelsEditSave}
          onClose={() => setEditModelsConn(null)}
        />
      )}

      {deleteTarget && (
        <DeleteDialog
          target={deleteTarget}
          onConfirm={handleDeleteConfirm}
          onClose={() => setDeleteTarget(null)}
        />
      )}
    </div>
  )
}
