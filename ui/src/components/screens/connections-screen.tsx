import { useEffect, useState, useMemo } from 'react'
import { api } from '../../api'
import { Plus, Search, Link2, AlertTriangle } from 'lucide-react'
import { getProviderLabel } from '../connections/helpers'
import ConnectionCard from '../connections/ConnectionCard'
import AddConnectionModal from '../connections/AddConnectionModal'
import EditModelsModal from '../connections/EditModelsModal'
import DeleteDialog from '../connections/DeleteDialog'
import { AwsLogo, OpenAILogo, CustomLogo } from '../connections/helpers'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

export default function ConnectionsScreen() {
  const [conns, setConns] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [editModelsConn, setEditModelsConn] = useState<any | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null)
  const [quotaResult, setQuotaResult] = useState<Record<string, any>>({})
  const [searchQuery, setSearchQuery] = useState('')
  const [autoRefreshQuota, setAutoRefreshQuota] = useState(false)

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

  // ── Filter ─────────────────────────────────────────────────────────────────
  const filteredConns = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q) return conns
    return conns.filter((c: any) => {
      const providerLabel = getProviderLabel(c.provider).toLowerCase()
      const hay = [c.name, c.email, c.baseUrl, c.providerName, c.authMethod, providerLabel, ...(c.supportedModels || [])]
        .filter(Boolean).join(' ').toLowerCase()
      return hay.includes(q)
    })
  }, [conns, searchQuery])

  const groupedConns = useMemo(() => {
    const list = [...filteredConns].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
    const groups: Record<string, any[]> = {}
    list.forEach((c: any) => {
      const p = c.provider === 'openai-compatible' ? 'other' : c.provider
      if (!groups[p]) groups[p] = []
      groups[p].push(c)
    })
    const result: any[] = []
    if (groups['kiro']) result.push({ id: 'kiro', label: 'AWS / Kiro', items: groups['kiro'], icon: <AwsLogo size={16} />, colorClass: 'bg-orange-500/10 border-orange-500/20 text-orange-600' })
    if (groups['openai']) result.push({ id: 'openai', label: 'OpenAI', items: groups['openai'], icon: <OpenAILogo size={16} />, colorClass: 'bg-emerald-500/10 border-emerald-500/20 text-emerald-600' })
    if (groups['other']) result.push({ id: 'other', label: 'Custom API', items: groups['other'], icon: <CustomLogo size={16} />, colorClass: 'bg-purple-500/10 border-purple-500/20 text-purple-600' })
    Object.keys(groups).forEach(k => {
      if (!['kiro', 'openai', 'other'].includes(k)) {
        const otherGroup = result.find(r => r.id === 'other')
        if (otherGroup) otherGroup.items.push(...groups[k])
        else result.push({ id: 'other', label: 'Custom API', items: groups[k], icon: <CustomLogo size={16} />, colorClass: 'bg-purple-500/10 border-purple-500/20 text-purple-600' })
      }
    })
    return result
  }, [filteredConns])

  // ── Load ───────────────────────────────────────────────────────────────────
  const load = () => api.getConnections().then(d => {
    setConns(d || [])
    d?.forEach((c: any) => {
      if (c.isActive) {
        api.checkQuota(c.id)
          .then(res => setQuotaResult(prev => ({ ...prev, [c.id]: res })))
          .catch(() => {})
      }
    })
  }).catch(() => {})

  useEffect(() => { load() }, [])

  useEffect(() => {
    if (!autoRefreshQuota) return
    const t = setInterval(() => {
      conns.forEach((c: any) => {
        if (c.isActive) {
          api.checkQuota(c.id)
            .then(res => setQuotaResult(prev => ({ ...prev, [c.id]: res })))
            .catch(() => {})
        }
      })
    }, 10000)
    return () => clearInterval(t)
  }, [autoRefreshQuota, conns])

  // ── Handlers ───────────────────────────────────────────────────────────────
  const handleAddSuccess = (msg: string) => {
    setShowAdd(false)
    toast.success(msg)
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
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
            <Link2 className="h-5 w-5 text-primary" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Connections</h1>
            <p className="text-sm text-muted-foreground">
              Manage your AI provider accounts. Connect Kiro, OpenAI or any compatible endpoints.
            </p>
          </div>
        </div>
        <Button onClick={() => setShowAdd(true)} className="gap-2 self-start sm:self-auto">
          <Plus className="h-4 w-4" /> Add Connection
        </Button>
      </div>

      {/* Stats */}
      {conns.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
                <Link2 size={12} /> Total Connections
              </div>
              <div className="text-2xl font-bold">{connectionStats.total}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-2 text-xs text-emerald-600 mb-1">
                <span className="h-2 w-2 rounded-full bg-emerald-500" /> Active
              </div>
              <div className="text-2xl font-bold text-emerald-600">{connectionStats.active}</div>
            </CardContent>
          </Card>
          <Card className={connectionStats.needsAttention > 0 ? 'border-destructive/40' : ''}>
            <CardContent className="p-4">
              <div className={cn('flex items-center gap-2 text-xs mb-1', connectionStats.needsAttention > 0 ? 'text-destructive' : 'text-muted-foreground')}>
                <AlertTriangle size={12} /> Issues
              </div>
              <div className={cn('text-2xl font-bold', connectionStats.needsAttention > 0 ? 'text-destructive' : 'text-muted-foreground')}>
                {connectionStats.needsAttention}
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Toolbar */}
      {conns.length > 0 && (
        <div className="flex flex-col sm:flex-row sm:items-center gap-3 rounded-lg border bg-muted/30 p-3">
          <div className="relative flex-1 min-w-0">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
            <Input
              type="search"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Search by name, email, or model…"
              className="pl-9 h-8 text-sm"
            />
          </div>
          <label className="flex cursor-pointer items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors select-none shrink-0">
            <input
              type="checkbox"
              checked={autoRefreshQuota}
              onChange={e => setAutoRefreshQuota(e.target.checked)}
              className="w-3.5 h-3.5 rounded cursor-pointer"
            />
            Auto refresh Quotas
          </label>
        </div>
      )}

      {/* List */}
      {conns.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-primary/10 mb-4">
            <Link2 className="h-6 w-6 text-primary" />
          </div>
          <h3 className="text-lg font-semibold mb-1">No connections yet</h3>
          <p className="text-sm text-muted-foreground mb-4 max-w-sm">
            Add Kiro AI or OpenAI accounts to begin routing requests.
          </p>
          <Button onClick={() => setShowAdd(true)} variant="outline" className="gap-2">
            <Plus size={16} /> Connect Now
          </Button>
        </div>
      ) : filteredConns.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border py-12 text-center">
          <p className="text-sm text-muted-foreground">No connections matching "{searchQuery.trim()}"</p>
          <Button variant="link" onClick={() => setSearchQuery('')} className="mt-2 text-sm">
            Clear filters
          </Button>
        </div>
      ) : (
        <div className="space-y-6">
          {groupedConns.map(group => (
            <div key={group.id}>
              <div className="flex items-center gap-2 mb-3">
                <div className={cn('flex h-7 w-7 items-center justify-center rounded-lg border', group.colorClass)}>
                  {group.icon}
                </div>
                <h3 className="text-sm font-semibold">{group.label}</h3>
                <Badge variant="secondary" className="text-[10px] h-5">{group.items.length}</Badge>
              </div>
              <div className="flex flex-col gap-2">
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
            </div>
          ))}
        </div>
      )}

      {showAdd && <AddConnectionModal onSuccess={handleAddSuccess} onClose={() => setShowAdd(false)} />}
      {editModelsConn && <EditModelsModal conn={editModelsConn} onSave={handleModelsEditSave} onClose={() => setEditModelsConn(null)} />}
      {deleteTarget && <DeleteDialog target={deleteTarget} onConfirm={handleDeleteConfirm} onClose={() => setDeleteTarget(null)} />}
    </div>
  )
}
