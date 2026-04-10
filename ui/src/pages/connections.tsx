import { useEffect, useState, useRef, useCallback, useMemo } from 'react'
import { api } from '../api'
import {
  Plus, Trash2, TestTube, Search, Upload, Shield, Settings2, ExternalLink,
  Loader2, Download, Globe, GitBranch, RefreshCw, Pencil, Check, X, BarChart2,
  Play, CheckCircle2, XCircle, Link2, AlertTriangle, MoreHorizontal, ArrowDownAZ
} from 'lucide-react'
import ModelSelector from '../components/ModelSelector'
import { getModelName } from '../models-config'

const PROVIDERS = [
  { id: 'kiro', name: 'Kiro AI', icon: 'KI' },
  { id: 'openai', name: 'OpenAI', icon: 'OA' },
  { id: 'openai-compatible', name: 'OpenAI Compatible', icon: 'OC' },
] as const

type ImportMode = 'detect' | 'file' | 'manual' | 'builder-id' | 'idc' | 'social'
type SortMode = 'name' | 'issues' | 'provider'

interface DeviceCodeState {
  sessionId: string; userCode: string; verificationUri: string;
  verificationUriComplete: string; expiresIn: number; interval: number
}
interface SocialLoginState {
  sessionId: string; loginUrl: string; provider: 'google' | 'github'
}

// ─── helpers ──────────────────────────────────────────────────────────────────

function secsToHuman(secs: number): string {
  if (secs <= 0) return '0s'
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.floor(secs / 60)}m`
  return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`
}

function getProviderMeta(provider: string) {
  return PROVIDERS.find(x => x.id === provider)
}

function ProviderBadge({ provider }: { provider: string }) {
  const p = getProviderMeta(provider)
  return (
    <span className="flex items-center gap-1 rounded bg-[var(--bg)] px-2 py-0.5 text-xs text-[var(--text-muted)]">
      <span>{p?.icon ?? '📦'}</span>
      {p?.name ?? provider}
    </span>
  )
}

function connectionAttentionRank(c: { isActive?: boolean; rateLimitedUntil?: string; expiresAt?: string; backoffLevel?: number; lastError?: string }) {
  if (!c.isActive) return 2
  const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
  const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date()
  const hasIssue = isRL || isExpired || (c.backoffLevel ?? 0) > 0 || !!c.lastError
  return hasIssue ? 0 : 1
}


// ─── Token bar ─────────────────────────────────────────────────────────────

function TokenBar({ conn }: { conn: any }) {
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 10000)
    return () => clearInterval(t)
  }, [])

  if (!conn.expiresAt && !conn.hasApiKey) return null

  if (conn.hasApiKey) {
    return (
      <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
        <span className="text-xs text-[var(--success)]">✓ API Key (permanent)</span>
      </div>
    )
  }

  const expMs = new Date(conn.expiresAt).getTime()
  const secsLeft = Math.ceil((expMs - now) / 1000)
  const expired = secsLeft <= 0
  const pct = conn.expiresIn ? Math.max(0, Math.min(100, ((expMs - now) / (conn.expiresIn * 1000)) * 100)) : 50
  const color = expired ? '#f87171' : pct < 15 ? '#fbbf24' : pct < 40 ? '#fb923c' : '#4ade80'

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-xs">
        <span className={expired ? 'text-red-400' : pct < 15 ? 'text-amber-400' : 'text-[var(--text-muted)]'}>
          {expired ? '✗ Token expired' : `Token: ${secsToHuman(secsLeft)} left (${Math.round(pct)}%)`}
        </span>
      </div>
      <div className="h-1 bg-[var(--border)] rounded-full overflow-hidden w-full">
        <div
          className="h-full rounded-full transition-all duration-1000"
          style={{ width: `${pct}%`, backgroundColor: color }}
        />
      </div>
    </div>
  )
}

// ─── Quota Panel ───────────────────────────────────────────────────────────

interface QuotaBucket {
  label: string
  used: number
  limit: number
  pct: number
  checkedAt?: string
  expiresAt?: string
}

function QuotaPanel({ data, loading }: { data: any; loading: boolean }) {
  if (loading) {
    return (
      <div className="flex items-center gap-2 text-xs text-[var(--text-muted)] py-1">
        <Loader2 size={12} className="animate-spin text-[var(--accent)]" />
        Checking quota…
      </div>
    )
  }
  if (!data) return null

  // Build bucket list
  const buckets: QuotaBucket[] = []

  if (data.quotaSupported !== false) {
    // API limits
    if (data.requestsLimit != null && data.requestsLimit >= 0) {
      buckets.push({
        label: 'requests',
        used: data.requestsLimit - (data.requestsRemaining ?? 0),
        limit: data.requestsLimit,
        pct: data.requestsPct ?? 0,
        checkedAt: data.resetRequests,
      })
    }
    if (data.freeTrialLimit != null && data.freeTrialLimit >= 0) {
      buckets.push({
        label: 'free trial',
        used: data.freeTrialLimit - (data.freeTrialRemaining ?? 0),
        limit: data.freeTrialLimit,
        pct: data.freeTrialPct ?? 0,
        expiresAt: data.freeTrialExpiresAt,
      })
    }
    if (data.tokensLimit != null && data.tokensLimit >= 0) {
      buckets.push({
        label: 'tokens',
        used: data.tokensLimit - (data.tokensRemaining ?? 0),
        limit: data.tokensLimit,
        pct: data.tokensPct ?? 0,
        checkedAt: data.resetTokens,
      })
    }
  }

  const color = (pct: number) =>
    pct >= 90 ? '#f87171' : pct >= 70 ? '#fbbf24' : '#4ade80'

  const fmt = (n: number) => n.toLocaleString()

  return (
    <div className="space-y-2 pt-0.5">
      {buckets.length === 0 && (!data.overageCharges || data.overageCharges <= 0) && (
        <p className="text-xs text-[var(--text-muted)] italic">
          {data.note || 'No quota information available for this connection.'}
        </p>
      )}
      {data.overageCharges > 0 && (
        <div className="text-xs text-amber-500 font-medium bg-amber-500/10 px-2 py-1 rounded inline-flex items-center gap-1">
          ⚠️ Overage Charges: ${data.overageCharges.toLocaleString()}
        </div>
      )}
      {buckets.map((b) => {
        const used = Math.max(0, Math.min(b.pct, 100))
        const c = color(used)
        return (
          <div key={b.label} className="space-y-1">
            <div className="flex items-center justify-between text-xs">
              <span className="flex items-center gap-1.5">
                <span
                  className="w-2 h-2 rounded-full flex-shrink-0"
                  style={{ backgroundColor: '#4ade80' }}
                />
                <span className="text-[var(--text-muted)]">{b.label}</span>
              </span>
              <div className="flex items-center gap-2 text-[var(--text-muted)]">
                <span className="font-mono">
                  {fmt(b.used)}&nbsp;/&nbsp;{fmt(b.limit)}
                </span>
                <span
                  className="font-semibold tabular-nums"
                  style={{ color: c }}
                >
                  {used}%
                </span>
                {b.checkedAt && (
                  <span className="text-[10px] opacity-60">resets {b.checkedAt}</span>
                )}
                {b.expiresAt && (
                  <span className="text-[10px] opacity-60">expires {b.expiresAt}</span>
                )}
              </div>
            </div>
            <div className="h-1.5 bg-[var(--border)] rounded-full overflow-hidden">
              <div
                className="h-full rounded-full transition-all duration-700"
                style={{ width: `${used}%`, backgroundColor: c }}
              />
            </div>
          </div>
        )
      })}
      {data.statusCode != null && data.statusCode !== 200 && (
        <p className="text-xs text-red-400">HTTP {data.statusCode} from upstream</p>
      )}
    </div>
  )
}

// ─── Status badges row ──────────────────────────────────────────────────────

function StatusRow({ conn }: { conn: any }) {
  const isRL = conn.rateLimitedUntil && new Date(conn.rateLimitedUntil) > new Date()
  const rlSecs = isRL ? Math.ceil((new Date(conn.rateLimitedUntil).getTime() - Date.now()) / 1000) : 0
  const lockCount = conn.modelLocks
    ? Object.values(conn.modelLocks).filter((e: any) => new Date(e) > new Date()).length
    : 0

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {/* Rate limit */}
      {isRL ? (
        <span className="inline-flex items-center gap-1 text-xs bg-amber-400/10 text-amber-400 border border-amber-400/20 px-1.5 py-0.5 rounded">
          ⏱ Rate limit: {secsToHuman(rlSecs)}
        </span>
      ) : (
        <span className="text-xs text-[var(--success)]">✓ No rate limit</span>
      )}

      {/* Backoff */}
      {conn.backoffLevel > 0 && (
        <span className="inline-flex items-center gap-1 text-xs bg-orange-400/10 text-orange-400 border border-orange-400/20 px-1.5 py-0.5 rounded">
          Backoff: {conn.backoffLevel}/7
        </span>
      )}

      {/* Model locks */}
      {lockCount > 0 && (
        <span className="inline-flex items-center gap-1 text-xs bg-orange-400/10 text-orange-400 border border-orange-400/20 px-1.5 py-0.5 rounded">
          🔒 {lockCount} model{lockCount > 1 ? 's' : ''} locked
        </span>
      )}

      {/* Last error */}
      {conn.lastError && (
        <span className="text-xs text-red-400 truncate max-w-xs" title={conn.lastError}>
          ✗ {conn.lastError.slice(0, 60)}{conn.lastError.length > 60 ? '…' : ''}
        </span>
      )}
    </div>
  )
}



// ─── Inline name editor ────────────────────────────────────────────────────

function InlineName({ conn, onRename }: { conn: any; onRename: (id: string, name: string) => Promise<void> }) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(conn.name)
  const [saving, setSaving] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { if (editing) inputRef.current?.focus() }, [editing])

  const save = async () => {
    if (!value.trim() || value === conn.name) { setEditing(false); return }
    setSaving(true)
    await onRename(conn.id, value.trim())
    setSaving(false)
    setEditing(false)
  }

  const cancel = () => { setValue(conn.name); setEditing(false) }

  if (editing) {
    return (
      <div className="flex items-center gap-1">
        <input
          ref={inputRef}
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') save(); if (e.key === 'Escape') cancel() }}
          className="bg-[var(--bg)] border border-[var(--accent)] rounded px-2 py-0.5 text-sm font-medium outline-none w-52"
        />
        <button
          onClick={save}
          disabled={saving}
          aria-label="Save connection name"
          className="p-0.5 text-[var(--success)] hover:opacity-80"
        >
          {saving ? <Loader2 size={12} className="animate-spin" /> : <Check size={13} />}
        </button>
        <button
          onClick={cancel}
          aria-label="Cancel editing connection name"
          className="p-0.5 text-[var(--text-muted)] hover:opacity-80"
        >
          <X size={13} />
        </button>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-1 group/name">
      <span className="font-medium text-sm">{conn.name}</span>
      <button
        onClick={() => setEditing(true)}
        aria-label={`Rename ${conn.name}`}
        className="opacity-0 group-hover/name:opacity-100 p-0.5 text-[var(--text-muted)] hover:text-[var(--text)] transition-opacity"
        title="Rename"
      >
        <Pencil size={11} />
      </button>
    </div>
  )
}

// ─── Main component ────────────────────────────────────────────────────────

export default function Connections() {
  const [conns, setConns] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [provider, setProvider] = useState('kiro')
  const [importMode, setImportMode] = useState<ImportMode>('detect')
  const [form, setForm] = useState({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
  const [openaiForm, setOpenaiForm] = useState({ name: '', apiKey: '', supportedModels: '' })
  const [customForm, setCustomForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [testResult, setTestResult] = useState<Record<string, any>>({})
  const [quotaResult, setQuotaResult] = useState<Record<string, any>>({})
  const [quotaLoading, setQuotaLoading] = useState<Record<string, boolean>>({})
  const [expandedModels, setExpandedModels] = useState<Record<string, boolean>>({})
  const [editModels, setEditModels] = useState<Record<string, string>>({})
  const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null)
  const [polling, setPolling] = useState(false)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [idcForm, setIdcForm] = useState({ startUrl: '', region: '' })
  const [socialLogin, setSocialLogin] = useState<SocialLoginState | null>(null)
  const [socialCallbackUrl, setSocialCallbackUrl] = useState('')
  const [socialProvider, setSocialProvider] = useState<'google' | 'github'>('google')
  const [fetchingModels, setFetchingModels] = useState<Record<string, boolean>>({})
  const [fetchedModels, setFetchedModels] = useState<Record<string, string[]>>({})
  const [modelTestResults, setModelTestResults] = useState<Record<string, Record<string, { status: 'ok' | 'error' | 'loading'; message?: string }>>>({})
  const [openaiMode, setOpenaiMode] = useState<'oauth' | 'apikey'>('oauth')
  const [openaiOAuthSession, setOpenaiOAuthSession] = useState<{sessionId: string; authUrl: string} | null>(null)
  const openaiPollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const [autoRefreshQuota, setAutoRefreshQuota] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [sortMode, setSortMode] = useState<SortMode>('name')
  const [menuOpenFor, setMenuOpenFor] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null)
  const [deleteConfirmName, setDeleteConfirmName] = useState('')
  const [deleteBusy, setDeleteBusy] = useState(false)

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

  const filteredConns = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q) return conns
    return conns.filter((c: any) => {
      const providerLabel = getProviderLabel(c.provider).toLowerCase()
      const hay = [
        c.name,
        c.email,
        c.baseUrl,
        c.providerName,
        c.authMethod,
        providerLabel,
        ...(c.supportedModels || []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      return hay.includes(q)
    })
  }, [conns, searchQuery])

  const sortedConns = useMemo(() => {
    const list = [...filteredConns]
    if (sortMode === 'name') {
      list.sort((a: { name: string }, b: { name: string }) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
    } else if (sortMode === 'provider') {
      list.sort((a: { provider: string; name: string }, b: { provider: string; name: string }) =>
        a.provider.localeCompare(b.provider) || a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
    } else {
      list.sort((a, b) => {
        const d = connectionAttentionRank(a) - connectionAttentionRank(b)
        return d !== 0 ? d : a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
      })
    }
    return list
  }, [filteredConns, sortMode])

  useEffect(() => {
    if (!menuOpenFor) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuOpenFor(null)
    }
    const onMouseDown = (e: MouseEvent) => {
      const el = e.target as HTMLElement | null
      if (el?.closest?.('[data-connection-menu-root]')) return
      setMenuOpenFor(null)
    }
    window.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onMouseDown)
    return () => {
      window.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onMouseDown)
    }
  }, [menuOpenFor])

  useEffect(() => {
    if (!deleteTarget) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !deleteBusy) {
        setDeleteTarget(null)
        setDeleteConfirmName('')
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [deleteTarget, deleteBusy])

  useEffect(() => {
    if (!deleteTarget) return
    const id = requestAnimationFrame(() => {
      document.getElementById('delete-confirm-input')?.focus()
    })
    return () => cancelAnimationFrame(id)
  }, [deleteTarget?.id])

  useEffect(() => {
    if (!autoRefreshQuota) return
    const t = setInterval(() => {
      conns.forEach((c: any) => {
        if (c.isActive) {
          api.checkQuota(c.id)
            .then(res => setQuotaResult(prev => ({ ...prev, [c.id]: res })))
            .catch(e => setQuotaResult(prev => ({ ...prev, [c.id]: { error: e.message } })))
        }
      })
    }, 10000)
    return () => clearInterval(t)
  }, [autoRefreshQuota, conns])

  const load = () => api.getConnections().then(d => {
    setConns(d || [])
    d?.forEach((c: any) => {
      if (c.isActive) {
        api.checkQuota(c.id)
          .then(res => setQuotaResult(prev => ({ ...prev, [c.id]: res })))
          .catch(e => setQuotaResult(prev => ({ ...prev, [c.id]: { error: e.message } })))
      }
    })
  }).catch(() => {})
  useEffect(() => { load() }, [])
  useEffect(() => { return () => { if (pollTimerRef.current) clearTimeout(pollTimerRef.current) } }, [])

  const resetForm = () => {
    setForm({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
    setOpenaiForm({ name: '', apiKey: '', supportedModels: '' })
    setCustomForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setIdcForm({ startUrl: '', region: '' })
    setDeviceCode(null); setPolling(false)
    setSocialLogin(null); setSocialCallbackUrl('')
    setOpenaiOAuthSession(null)
    if (openaiPollRef.current) { clearInterval(openaiPollRef.current); openaiPollRef.current = null }
    setError(''); setSuccess('')
    if (pollTimerRef.current) { clearTimeout(pollTimerRef.current); pollTimerRef.current = null }
  }

  useEffect(() => {
    if (!showAdd) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setShowAdd(false)
        resetForm()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [showAdd])

  const parseSupportedModels = (str: string) => str.split('\n').map(s => s.trim()).filter(Boolean)

  // ── Builder ID ─────────────────────────────────────────────────────────────
  const handleStartBuilderID = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.startBuilderID()
      setDeviceCode(res); setPolling(true); startPolling(res.sessionId, res.interval)
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── IDC ────────────────────────────────────────────────────────────────────
  const handleStartIDC = async () => {
    if (!idcForm.startUrl) { setError('Start URL is required'); return }
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.startIDC({ startUrl: idcForm.startUrl, region: idcForm.region || undefined })
      setDeviceCode(res); setPolling(true); startPolling(res.sessionId, res.interval)
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── Poll ───────────────────────────────────────────────────────────────────
  const startPolling = useCallback((sessionId: string, interval: number) => {
    const ms = Math.max(interval, 3) * 1000
    const poll = async () => {
      try {
        const res = await api.pollAuth(sessionId)
        if (res.status === 'pending') { pollTimerRef.current = setTimeout(poll, ms); return }
        if (res.status === 'success') {
          setSuccess(`Connected! ${res.email ? `(${res.email})` : res.name}`)
          setDeviceCode(null); setPolling(false); setShowAdd(false); resetForm(); load(); return
        }
        setError(res.errorDescription || res.error || 'Authorization failed')
        setDeviceCode(null); setPolling(false)
      } catch (e: any) { setError(e.message); setDeviceCode(null); setPolling(false) }
    }
    pollTimerRef.current = setTimeout(poll, ms)
  }, [])

  // ── Social ─────────────────────────────────────────────────────────────────
  const handleStartSocial = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.startSocialLogin(socialProvider)
      setSocialLogin({ ...res, provider: socialProvider })
      window.open(res.loginUrl, '_blank')
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleExchangeSocial = async () => {
    if (!socialLogin || !socialCallbackUrl) return
    setLoading(true); setError('')
    try {
      await api.exchangeSocialCode({ sessionId: socialLogin.sessionId, callbackUrl: socialCallbackUrl })
      setSuccess('Social login connected!'); setSocialLogin(null); setSocialCallbackUrl('')
      setShowAdd(false); resetForm(); load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── Detect ─────────────────────────────────────────────────────────────────
  const handleDetect = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.detectKiroToken()
      if (res.found) {
        await api.importConnection({ refreshToken: res.refreshToken, clientId: res.clientId || '', clientSecret: res.clientSecret || '', region: res.region || '', authMethod: res.authMethod || 'builder-id' })
        setSuccess('Connection imported!'); setShowAdd(false); resetForm(); load()
      } else {
        setError(res.error || 'No Kiro token found.')
      }
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── File upload ────────────────────────────────────────────────────────────
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return
    setLoading(true); setError(''); setSuccess('')
    try {
      const data = JSON.parse(await file.text())
      if (!data.refreshToken) { setError('Invalid file: missing refreshToken'); return }
      await api.importConnection({ refreshToken: data.refreshToken, clientId: data.clientId || '', clientSecret: data.clientSecret || '', region: data.region || '', authMethod: data.authMethod?.toLowerCase() || 'builder-id' })
      setSuccess('Imported!'); setShowAdd(false); resetForm(); load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── Manual ─────────────────────────────────────────────────────────────────
  const handleManualImport = async () => {
    setLoading(true); setError(''); setSuccess('')
    try { await api.importConnection(form); setSuccess('Imported!'); setShowAdd(false); resetForm(); load() }
    catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── OpenAI ─────────────────────────────────────────────────────────────────
  const handleAddOpenAI = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const models = parseSupportedModels(openaiForm.supportedModels)
      await api.addOpenAIConnection({ name: openaiForm.name || undefined, apiKey: openaiForm.apiKey, supportedModels: models.length > 0 ? models : undefined })
      setSuccess('OpenAI added!'); setShowAdd(false); resetForm(); load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── Custom ─────────────────────────────────────────────────────────────────
  const handleAddCustom = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const models = parseSupportedModels(customForm.supportedModels)
      await api.addCustomConnection({ name: customForm.name || undefined, apiKey: customForm.apiKey || undefined, baseUrl: customForm.baseUrl, supportedModels: models.length > 0 ? models : undefined })
      setSuccess('Custom added!'); setShowAdd(false); resetForm(); load()
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  // ── Card actions ───────────────────────────────────────────────────────────
  const openDeleteDialog = (id: string, name: string) => {
    setMenuOpenFor(null)
    setDeleteTarget({ id, name })
    setDeleteConfirmName('')
  }

  const closeDeleteDialog = () => {
    if (deleteBusy) return
    setDeleteTarget(null)
    setDeleteConfirmName('')
  }

  const confirmDeleteConnection = async () => {
    if (!deleteTarget || deleteConfirmName !== deleteTarget.name) return
    setDeleteBusy(true)
    try {
      await api.deleteConnection(deleteTarget.id)
      setDeleteTarget(null)
      setDeleteConfirmName('')
      load()
    } finally {
      setDeleteBusy(false)
    }
  }

  const handleTest = async (id: string) => {
    setTestResult(prev => ({ ...prev, [id]: { loading: true } }))
    try {
      const res = await api.testConnection(id)
      setTestResult(prev => ({ ...prev, [id]: res }))
    } catch (e: any) {
      setTestResult(prev => ({ ...prev, [id]: { status: 'error', message: e.message } }))
    }
  }

  const handleCheckQuota = async (id: string) => {
    setQuotaLoading(prev => ({ ...prev, [id]: true }))
    // toggle off if already showing
    if (quotaResult[id] && !quotaLoading[id]) {
      setQuotaResult(prev => { const n = { ...prev }; delete n[id]; return n })
      setQuotaLoading(prev => ({ ...prev, [id]: false }))
      return
    }
    try {
      const res = await api.checkQuota(id)
      setQuotaResult(prev => ({ ...prev, [id]: res }))
    } catch (e: any) {
      setQuotaResult(prev => ({ ...prev, [id]: { error: e.message } }))
    } finally {
      setQuotaLoading(prev => ({ ...prev, [id]: false }))
    }
  }

  const handleToggle = async (id: string, isActive: boolean) => {
    await api.updateConnection(id, { isActive: !isActive }); load()
  }

  const handleRename = async (id: string, name: string) => {
    await api.updateConnection(id, { name }); load()
  }

  const handleResetCooldown = async (id: string) => {
    try { await api.resetCooldown(id); load() } catch (e: any) { console.error(e.message) }
  }

  const handleSaveModels = async (id: string) => {
    const conn = conns.find(c => c.id === id)
    const rawModels = editModels[id] ? editModels[id].split('\n').filter(Boolean) : []
    // Strip provider prefix before saving — backend stores bare IDs
    const models = rawModels.map(m => {
      if (conn && m.startsWith(conn.provider + '/')) return m.slice(conn.provider.length + 1)
      const slash = m.indexOf('/')
      return slash >= 0 ? m.slice(slash + 1) : m
    })
    await api.updateConnection(id, { supportedModels: models, setModels: true })
    setExpandedModels(prev => ({ ...prev, [id]: false })); load()
  }

  const toggleModelEdit = (conn: any) => {
    const id = conn.id
    if (expandedModels[id]) { setExpandedModels(prev => ({ ...prev, [id]: false })) }
    else {
      // Convert bare IDs to provider/id format for ModelSelector
      const withPrefix = (conn.supportedModels || []).map((m: string) =>
        m.includes('/') ? m : `${conn.provider}/${m}`
      )
      setEditModels(prev => ({ ...prev, [id]: withPrefix.join('\n') }))
      setExpandedModels(prev => ({ ...prev, [id]: true }))
    }
  }

  const handleFetchModels = async (conn: any) => {
    setFetchingModels(prev => ({ ...prev, [conn.id]: true }))
    try {
      const res = await api.fetchConnectionModels(conn.id)
      const fetched = res.models || []
      setFetchedModels(prev => ({ ...prev, [conn.id]: fetched }))
      // Add provider prefix for ModelSelector display
      const withPrefix = fetched.map((m: string) => m.includes('/') ? m : `${conn.provider}/${m}`)
      setEditModels(prev => ({ ...prev, [conn.id]: withPrefix.join('\n') }))
      setExpandedModels(prev => ({ ...prev, [conn.id]: true }))
    } catch (e: any) {
      if (conn.provider === 'openai' || conn.provider === 'openai-compatible') {
        const fallbacks = [
          'gpt-4o', 'gpt-4o-mini', 'o1', 'o1-mini', 'o3-mini', 'chatgpt-4o-latest', 
          'gpt-4-turbo', 'gpt-3.5-turbo', 'claude-3-5-sonnet-20240620', 'claude-3-opus-20240229'
        ]
        setFetchedModels(prev => ({ ...prev, [conn.id]: fallbacks }))
        setEditModels(prev => ({ ...prev, [conn.id]: fallbacks.join('\n') }))
        setExpandedModels(prev => ({ ...prev, [conn.id]: true }))
      }
      setTestResult(prev => ({ ...prev, [conn.id]: { status: 'error', message: 'Fetch skipped/failed. Fallback list loaded.' } }))
    } finally { setFetchingModels(prev => ({ ...prev, [conn.id]: false })) }
  }

  const handleTestModel = async (connId: string, model: string) => {
    setModelTestResults(prev => ({
      ...prev,
      [connId]: { ...prev[connId], [model]: { status: 'loading' } }
    }))
    try {
      const res = await api.testModel(connId, model)
      setModelTestResults(prev => ({
        ...prev,
        [connId]: { ...prev[connId], [model]: { status: res.status === 'ok' ? 'ok' : 'error', message: res.message } }
      }))
    } catch (e: any) {
      setModelTestResults(prev => ({
        ...prev,
        [connId]: { ...prev[connId], [model]: { status: 'error', message: e.message } }
      }))
    }
  }

  const handleTestAllModels = async (connId: string, models: string[]) => {
    for (const model of models) {
      await handleTestModel(connId, model)
    }
  }

  const getProviderLabel = (p: string) => getProviderMeta(p)?.name ?? p

  // ── DeviceCode panel (shared) ──────────────────────────────────────────────
  const DeviceCodePanel = () => deviceCode ? (
    <div className="space-y-3">
      <div className="bg-[var(--bg)] rounded-lg p-4 border border-[var(--accent)]/30">
        <p className="text-xs text-[var(--text-muted)] mb-2">Enter this code on the authorization page:</p>
        <div className="text-2xl font-mono font-bold tracking-[0.3em] text-center text-[var(--accent)]">
          {deviceCode.userCode}
        </div>
      </div>
      <a href={deviceCode.verificationUriComplete || deviceCode.verificationUri} target="_blank" rel="noopener noreferrer"
        className="flex items-center justify-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm text-white transition-colors w-full">
        <ExternalLink size={16} /> Open Authorization Page
      </a>
      {polling && (
        <div className="flex items-center gap-2 text-sm text-[var(--text-muted)]">
          <Loader2 size={14} className="animate-spin text-[var(--accent)]" />
          Waiting for authorization… (polling every {deviceCode.interval}s)
        </div>
      )}
    </div>
  ) : null

  // ── Render ─────────────────────────────────────────────────────────────────
  return (
    <div>
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between mb-6">
        <div className="min-w-0 space-y-1">
          <h2 className="text-2xl font-bold tracking-tight">Connections</h2>
          <p className="text-sm text-[var(--text-muted)] max-w-xl">
            Kết nối tài khoản Kiro, OpenAI hoặc API tương thích. Bật/tắt từng kết nối, kiểm tra quota và model.
          </p>
        </div>
        <button
          type="button"
          onClick={() => { setShowAdd(!showAdd); resetForm() }}
          className="shrink-0 flex items-center justify-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors shadow-sm shadow-[var(--accent)]/20"
        >
          <Plus size={16} /> {showAdd ? 'Đóng form' : 'Add Connection'}
        </button>
      </div>

      {/* Toolbar: stats + search + quota option */}
      {conns.length > 0 && (
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:gap-4 mb-5">
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <span className="inline-flex items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--bg-card)] px-2.5 py-1.5 text-[var(--text-muted)]">
              <span className="font-medium text-[var(--text)] tabular-nums">{connectionStats.total}</span>
              total
            </span>
            <span className="inline-flex items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--bg-card)] px-2.5 py-1.5 text-[var(--text-muted)]">
              <span className="h-1.5 w-1.5 rounded-full bg-[var(--success)]" aria-hidden />
              <span className="font-medium text-[var(--success)] tabular-nums">{connectionStats.active}</span>
              active
            </span>
            {connectionStats.needsAttention > 0 && (
              <span className="inline-flex items-center gap-1.5 rounded-lg border border-amber-500/30 bg-amber-500/10 px-2.5 py-1.5 text-amber-400">
                <AlertTriangle size={12} className="shrink-0" aria-hidden />
                <span className="font-medium tabular-nums">{connectionStats.needsAttention}</span>
                need attention
              </span>
            )}
          </div>
          <div className="relative flex flex-1 min-w-0 flex-col gap-2 sm:max-w-md sm:flex-row sm:items-center sm:gap-3">
            <div className="relative min-w-0 flex-1">
              <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" aria-hidden />
              <input
                type="search"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                placeholder="Tìm theo tên, provider, email, URL, model…"
                aria-label="Filter connections"
                className="w-full rounded-lg border border-[var(--border)] bg-[var(--bg)] py-2 pl-9 pr-3 text-sm outline-none transition-colors placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40"
              />
            </div>
            <div className="relative shrink-0">
              <ArrowDownAZ size={14} className="pointer-events-none absolute left-2.5 top-1/2 z-[1] -translate-y-1/2 text-[var(--text-muted)]" aria-hidden />
              <select
                value={sortMode}
                onChange={e => setSortMode(e.target.value as SortMode)}
                aria-label="Sort connections"
                title="Sort connections"
                className="w-full cursor-pointer appearance-none rounded-lg border border-[var(--border)] bg-[var(--bg)] py-2 pl-8 pr-8 text-sm text-[var(--text)] outline-none transition-colors focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 sm:w-[11.5rem]"
              >
                <option value="name">Name (A–Z)</option>
                <option value="issues">Issues first</option>
                <option value="provider">Provider</option>
              </select>
              <span className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] text-[10px]" aria-hidden>
                ▼
              </span>
            </div>
          </div>
          <label className="flex cursor-pointer items-center gap-2 text-sm text-[var(--text-muted)] transition-colors hover:text-[var(--text)] lg:shrink-0">
            <input
              type="checkbox"
              checked={autoRefreshQuota}
              onChange={e => setAutoRefreshQuota(e.target.checked)}
              className="accent-[var(--accent)] h-3.5 w-3.5 rounded border-[var(--border)]"
            />
            Auto refresh quota
          </label>
        </div>
      )}

      {/* Add form */}
      {showAdd && (
        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] mb-6 space-y-4 shadow-lg shadow-black/20">
          <div className="flex items-start justify-between gap-3 border-b border-[var(--border)] pb-3">
            <div>
              <h3 className="text-sm font-semibold text-[var(--text)]">Thêm kết nối mới</h3>
              <p className="mt-0.5 text-xs text-[var(--text-muted)]">Chọn provider và phương thức đăng nhập / import.</p>
            </div>
            <button
              type="button"
              onClick={() => { setShowAdd(false); resetForm() }}
              className="shrink-0 rounded-lg p-2 text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text)]"
              aria-label="Đóng form thêm kết nối"
            >
              <X size={18} />
            </button>
          </div>
          {/* Provider tabs */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-2">Provider</label>
            <div className="flex gap-2 flex-wrap">
              {PROVIDERS.map(p => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => { setProvider(p.id); resetForm() }}
                  className={`flex items-center gap-2 rounded-lg border px-4 py-2 text-sm transition-colors ${
                    provider === p.id
                      ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]'
                      : 'border-[var(--border)] hover:border-[var(--bg-hover)]'
                  }`}
                >
                  <span>{p.icon}</span>
                  {p.name}
                </button>
              ))}
            </div>
          </div>

          {/* ── Kiro ─────────────────────────────────────────────────────────── */}
          {provider === 'kiro' && (
            <>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-2">Import Method</label>
                <div className="flex gap-2 flex-wrap">
                  {([
                    { id: 'detect' as ImportMode, label: 'Detect IDE', icon: <Search size={13} /> },
                    { id: 'builder-id' as ImportMode, label: 'Builder ID', icon: <ExternalLink size={13} /> },
                    { id: 'social' as ImportMode, label: 'Social Login', icon: <Globe size={13} /> },
                    { id: 'idc' as ImportMode, label: 'IAM IDC', icon: <Shield size={13} /> },
                    { id: 'file' as ImportMode, label: 'Import File', icon: <Upload size={13} /> },
                    { id: 'manual' as ImportMode, label: 'Paste Token', icon: null },
                  ]).map(m => (
                    <button key={m.id}
                      onClick={() => { setImportMode(m.id); setDeviceCode(null); setPolling(false); setSocialLogin(null); setError(''); setSuccess('') }}
                      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors ${importMode === m.id ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg)] text-[var(--text-muted)] hover:text-[var(--text)]'}`}>
                      {m.icon}{m.label}
                    </button>
                  ))}
                </div>
              </div>

              {/* Detect */}
              {importMode === 'detect' && (
                <div className="space-y-3">
                  <p className="text-sm text-[var(--text-muted)]">Auto-detect token từ <code className="text-xs bg-[var(--bg)] px-1.5 rounded">~/.aws/sso/cache/kiro-auth-token.json</code></p>
                  <button onClick={handleDetect} disabled={loading}
                    className="flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                    <Search size={16} /> {loading ? 'Detecting…' : 'Detect & Import'}
                  </button>
                </div>
              )}

              {/* Builder ID */}
              {importMode === 'builder-id' && (
                <div className="space-y-3">
                  <p className="text-sm text-[var(--text-muted)]">Đăng nhập bằng AWS Builder ID (device code flow).</p>
                  {!deviceCode && (
                    <button onClick={handleStartBuilderID} disabled={loading}
                      className="flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                      {loading ? <Loader2 size={16} className="animate-spin" /> : <ExternalLink size={16} />}
                      {loading ? 'Starting…' : 'Start Builder ID Login'}
                    </button>
                  )}
                  <DeviceCodePanel />
                </div>
              )}

              {/* IDC */}
              {importMode === 'idc' && (
                <div className="space-y-3">
                  <p className="text-sm text-[var(--text-muted)]">Đăng nhập bằng AWS IAM Identity Center (Enterprise SSO).</p>
                  {!deviceCode && (
                    <>
                      <div className="grid grid-cols-2 gap-3">
                        <div>
                          <label className="block text-xs text-[var(--text-muted)] mb-1">Start URL *</label>
                          <input value={idcForm.startUrl} onChange={e => setIdcForm({ ...idcForm, startUrl: e.target.value })}
                            placeholder="https://mycompany.awsapps.com/start"
                            className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
                        </div>
                        <div>
                          <label className="block text-xs text-[var(--text-muted)] mb-1">Region</label>
                          <input value={idcForm.region} onChange={e => setIdcForm({ ...idcForm, region: e.target.value })}
                            placeholder="us-east-1"
                            className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
                        </div>
                      </div>
                      <button onClick={handleStartIDC} disabled={loading || !idcForm.startUrl}
                        className="flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                        {loading ? <Loader2 size={16} className="animate-spin" /> : <ExternalLink size={16} />}
                        {loading ? 'Starting…' : 'Start IDC Login'}
                      </button>
                    </>
                  )}
                  <DeviceCodePanel />
                </div>
              )}

              {/* Social */}
              {importMode === 'social' && (
                <div className="space-y-3">
                  <p className="text-sm text-[var(--text-muted)]">Đăng nhập bằng Google hoặc GitHub qua Kiro social auth.</p>
                  {!socialLogin && (
                    <>
                      <div className="flex gap-2">
                        {(['google', 'github'] as const).map(p => (
                          <button key={p} onClick={() => setSocialProvider(p)}
                            className={`flex items-center gap-2 px-4 py-2 rounded-lg border text-sm transition-colors ${socialProvider === p ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]' : 'border-[var(--border)]'}`}>
                            {p === 'google' ? <Globe size={14} /> : <GitBranch size={14} />}
                            {p === 'google' ? 'Google' : 'GitHub'}
                          </button>
                        ))}
                      </div>
                      <button onClick={handleStartSocial} disabled={loading}
                        className="flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                        {loading ? <Loader2 size={16} className="animate-spin" /> : <Globe size={16} />}
                        Login with {socialProvider === 'google' ? 'Google' : 'GitHub'}
                      </button>
                    </>
                  )}
                  {socialLogin && (
                    <div className="space-y-3">
                      <div className="bg-[var(--bg)] rounded-lg p-3 border border-[var(--accent)]/30 text-xs text-[var(--text-muted)] space-y-1">
                        <p>1. Trang login đã mở trong browser.</p>
                        <p>2. Sau khi login, browser redirect về <code className="bg-[var(--bg-card)] px-1 rounded">kiro://…</code></p>
                        <p>3. Copy toàn bộ URL đó và paste vào ô bên dưới.</p>
                      </div>
                      <div>
                        <label className="block text-xs text-[var(--text-muted)] mb-1">Callback URL</label>
                        <input value={socialCallbackUrl} onChange={e => setSocialCallbackUrl(e.target.value)}
                          placeholder="kiro://kiro.kiroAgent/authenticate-success?code=…&state=…"
                          className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
                      </div>
                      <div className="flex gap-2">
                        <button onClick={handleExchangeSocial} disabled={loading || !socialCallbackUrl}
                          className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                          {loading ? 'Exchanging…' : 'Submit'}
                        </button>
                        <a href={socialLogin.loginUrl} target="_blank" rel="noopener noreferrer"
                          className="flex items-center gap-1.5 px-4 py-2 bg-[var(--bg)] hover:bg-[var(--bg-hover)] rounded-lg text-sm text-[var(--text-muted)]">
                          <ExternalLink size={14} /> Re-open
                        </a>
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* File */}
              {importMode === 'file' && (
                <div className="space-y-3">
                  <p className="text-sm text-[var(--text-muted)]">Upload file <code className="text-xs bg-[var(--bg)] px-1.5 rounded">kiro-auth-token.json</code></p>
                  <label className="flex items-center gap-2 px-4 py-3 bg-[var(--bg)] border border-dashed border-[var(--border)] rounded-lg cursor-pointer hover:border-[var(--accent)] transition-colors">
                    <Upload size={16} className="text-[var(--text-muted)]" />
                    <span className="text-sm text-[var(--text-muted)]">{loading ? 'Importing…' : 'Choose JSON file…'}</span>
                    <input type="file" accept=".json" onChange={handleFileUpload} className="hidden" disabled={loading} />
                  </label>
                </div>
              )}

              {/* Manual */}
              {importMode === 'manual' && (
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-1">Refresh Token *</label>
                    <input value={form.refreshToken} onChange={e => setForm({ ...form, refreshToken: e.target.value })}
                      placeholder="aorAAAAAG…"
                      className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-1">Auth Method</label>
                      <select value={form.authMethod} onChange={e => setForm({ ...form, authMethod: e.target.value })}
                        className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm">
                        <option value="builder-id">AWS Builder ID</option>
                        <option value="idc">AWS IAM Identity Center</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-1">Region</label>
                      <input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })}
                        placeholder="us-east-1"
                        className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
                    </div>
                  </div>
                  <button onClick={handleManualImport} disabled={loading || !form.refreshToken}
                    className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                    {loading ? 'Validating…' : 'Import'}
                  </button>
                </div>
              )}
            </>
          )}

          {/* ── OpenAI ───────────────────────────────────────────────────────── */}
          {provider === 'openai' && (
            <div className="space-y-3">
              {/* Mode toggle */}
              <div className="flex gap-2">
                <button onClick={() => { setOpenaiMode('oauth'); setOpenaiOAuthSession(null); setError(''); setSuccess('') }}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors ${openaiMode === 'oauth' ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg)] text-[var(--text-muted)] hover:text-[var(--text)]'}`}>
                  <Globe size={13} /> Login with OpenAI
                </button>
                <button onClick={() => { setOpenaiMode('apikey'); setOpenaiOAuthSession(null); setError(''); setSuccess('') }}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors ${openaiMode === 'apikey' ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg)] text-[var(--text-muted)] hover:text-[var(--text)]'}`}>
                  <Shield size={13} /> API Key
                </button>
              </div>

              {/* OAuth flow */}
              {openaiMode === 'oauth' && (
                <div className="space-y-3">
                  {!openaiOAuthSession ? (
                    <>
                      <div className="bg-[var(--bg)] rounded-lg px-3 py-2 text-xs text-[var(--text-muted)] space-y-1">
                        <p>🔐 Đăng nhập bằng tài khoản OpenAI (OAuth PKCE flow).</p>
                        <p>Sau khi click, browser sẽ mở trang login OpenAI. Đăng nhập xong, dntproxy sẽ tự nhận token.</p>
                      </div>
                      <button onClick={async () => {
                        setLoading(true); setError(''); setSuccess('')
                        try {
                          const res = await api.startOpenAIOAuth()
                          setOpenaiOAuthSession({ sessionId: res.sessionId, authUrl: res.authUrl })
                          window.open(res.authUrl, '_blank')
                          // Start polling
                          openaiPollRef.current = setInterval(async () => {
                            try {
                              const poll = await api.pollOpenAIOAuth(res.sessionId)
                              if (poll.status === 'pending') return
                              if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                              if (poll.status === 'success') {
                                setSuccess(`Connected! ${poll.email || poll.name || ''}`)
                                setOpenaiOAuthSession(null); setShowAdd(false); resetForm(); load()
                              } else {
                                setError(poll.error || 'Authorization failed')
                                setOpenaiOAuthSession(null)
                              }
                            } catch (e: any) {
                              if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                              setError(e.message); setOpenaiOAuthSession(null)
                            }
                          }, 2000)
                        } catch (e: any) { setError(e.message) }
                        finally { setLoading(false) }
                      }} disabled={loading}
                        className="flex items-center gap-2 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                        {loading ? <Loader2 size={16} className="animate-spin" /> : <Globe size={16} />}
                        {loading ? 'Starting…' : 'Login with OpenAI →'}
                      </button>
                    </>
                  ) : (
                    <div className="space-y-3">
                      <div className="bg-[var(--bg)] rounded-lg px-3 py-2 border border-[var(--accent)]/30 text-sm">
                        <div className="flex items-center gap-2 text-[var(--text-muted)] mb-2">
                          <Loader2 size={14} className="animate-spin text-[var(--accent)]" />
                          Đang chờ đăng nhập trong browser…
                        </div>
                        <p className="text-xs text-[var(--text-muted)]">Browser không mở? <a href={openaiOAuthSession.authUrl} target="_blank" rel="noopener noreferrer" className="text-[var(--accent)] underline">Click để mở thủ công</a></p>
                      </div>
                      <button onClick={() => { if (openaiPollRef.current) clearInterval(openaiPollRef.current); setOpenaiOAuthSession(null); setError('') }}
                        className="px-3 py-1.5 bg-[var(--bg)] hover:bg-[var(--bg-hover)] rounded-lg text-xs text-[var(--text-muted)]">
                        Cancel
                      </button>
                    </div>
                  )}
                </div>
              )}

              {/* API Key mode */}
              {openaiMode === 'apikey' && (
                <div className="space-y-3">
                  <div className="bg-[var(--bg)] rounded-lg px-3 py-2 text-xs text-[var(--text-muted)]">
                    Cần API Key? <a href="https://platform.openai.com/api-keys" target="_blank" rel="noopener noreferrer" className="text-[var(--accent)] underline inline-flex items-center gap-0.5">Tạo tại platform.openai.com <ExternalLink size={10} /></a>
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-1">Name (optional)</label>
                    <input value={openaiForm.name} onChange={e => setOpenaiForm({ ...openaiForm, name: e.target.value })}
                      placeholder="My OpenAI Account"
                      className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-1">API Key *</label>
                    <input type="password" value={openaiForm.apiKey} onChange={e => setOpenaiForm({ ...openaiForm, apiKey: e.target.value })}
                      placeholder="sk-proj-…"
                      className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
                  </div>

                  <button onClick={handleAddOpenAI} disabled={loading || !openaiForm.apiKey}
                    className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                    {loading ? 'Adding…' : 'Add OpenAI'}
                  </button>
                </div>
              )}
            </div>
          )}

          {/* ── OpenAI Compatible ─────────────────────────────────────────────── */}
          {provider === 'openai-compatible' && (
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">Name (optional)</label>
                <input value={customForm.name} onChange={e => setCustomForm({ ...customForm, name: e.target.value })}
                  placeholder="My Custom API"
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">Base URL *</label>
                <input value={customForm.baseUrl} onChange={e => setCustomForm({ ...customForm, baseUrl: e.target.value })}
                  placeholder="https://api.together.xyz"
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">API Key (optional)</label>
                <input type="password" value={customForm.apiKey} onChange={e => setCustomForm({ ...customForm, apiKey: e.target.value })}
                  placeholder="sk-…"
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
              </div>

              <button onClick={handleAddCustom} disabled={loading || !customForm.baseUrl}
                className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50">
                {loading ? 'Adding…' : 'Add Custom'}
              </button>
            </div>
          )}

          {error && <p className="text-sm text-[var(--danger)]">{error}</p>}
          {success && <p className="text-sm text-[var(--success)]">{success}</p>}
        </div>
      )}

      {/* ── Connection cards ─────────────────────────────────────────────────── */}
      {conns.length === 0 ? (
        <div className="rounded-xl border border-dashed border-[var(--border)] bg-[var(--bg-card)]/60 px-6 py-12 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-[var(--accent)]/15 text-[var(--accent)]">
            <Link2 size={24} strokeWidth={2} aria-hidden />
          </div>
          <h3 className="mt-4 text-base font-semibold text-[var(--text)]">Chưa có kết nối</h3>
          <p className="mx-auto mt-2 max-w-sm text-sm text-[var(--text-muted)]">
            Thêm Kiro hoặc OpenAI để proxy bắt đầu route request. Bạn có thể dùng Builder ID, OAuth hoặc API key.
          </p>
          <button
            type="button"
            onClick={() => { setShowAdd(true); resetForm() }}
            className="mt-6 inline-flex items-center gap-2 rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--accent-hover)]"
          >
            <Plus size={16} /> Add Connection
          </button>
        </div>
      ) : filteredConns.length === 0 ? (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-card)] px-6 py-10 text-center">
          <p className="text-sm text-[var(--text-muted)]">Không có kết nối khớp “{searchQuery.trim()}”.</p>
          <button
            type="button"
            onClick={() => setSearchQuery('')}
            className="mt-3 text-sm font-medium text-[var(--accent)] underline-offset-2 hover:underline"
          >
            Xóa bộ lọc
          </button>
        </div>
      ) : (
        <div className="space-y-3">
          {sortedConns.map((c: any) => {
            const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
            const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date()
            const hasIssue = isRL || isExpired || c.backoffLevel > 0 || c.lastError
            const statusDot = !c.isActive ? 'bg-[var(--text-muted)]'
              : isRL ? 'bg-amber-400 animate-pulse'
              : hasIssue ? 'bg-red-400'
              : (c.hasToken || c.hasApiKey) ? 'bg-[var(--success)]'
              : 'bg-[var(--text-muted)]'

            return (
              <div key={c.id}
                className={`bg-[var(--bg-card)] rounded-xl border transition-colors ${!c.isActive ? 'opacity-60 border-[var(--border)]' : isRL ? 'border-amber-500/40' : hasIssue ? 'border-red-500/20' : 'border-[var(--border)]'}`}>

                {/* Main row */}
                <div className="flex items-start justify-between p-4 gap-3">
                  <div className="flex items-start gap-3 min-w-0 flex-1">
                    {/* Toggle switch */}
                    <button onClick={() => handleToggle(c.id, c.isActive)}
                      aria-label={`${c.isActive ? 'Disable' : 'Enable'} connection ${c.name}`}
                      title={c.isActive ? 'Disable' : 'Enable'}
                      className={`mt-0.5 flex-shrink-0 relative w-9 h-5 rounded-full transition-colors duration-200 ${c.isActive ? 'bg-[var(--accent)]' : 'bg-[var(--border)]'}`}>
                      <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform duration-200 ${c.isActive ? 'translate-x-4' : 'translate-x-0'}`} />
                    </button>

                    {/* Status dot */}
                    <span className={`mt-1.5 w-2 h-2 rounded-full flex-shrink-0 ${statusDot}`} />

                    {/* Info block */}
                    <div className="min-w-0 flex-1 space-y-1.5">
                      {/* Name + badges */}
                      <div className="flex items-center gap-2 flex-wrap">
                        <InlineName conn={c} onRename={handleRename} />
                        <ProviderBadge provider={c.provider} />
                        {(c.providerName || c.authMethod) && (
                          <span className="text-xs bg-[var(--bg)] text-[var(--text-muted)] px-2 py-0.5 rounded">{c.providerName || c.authMethod}</span>
                        )}
                        {c.authType === 'apikey' && (
                          <span className="text-xs text-amber-400 bg-amber-400/10 px-2 py-0.5 rounded">API Key</span>
                        )}
                        {isExpired && (
                          <span className="text-xs text-red-400 bg-red-400/10 px-2 py-0.5 rounded">Token expired</span>
                        )}
                      </div>

                      {/* Sub info */}
                      {(c.email || c.baseUrl) && (
                        <div className="text-xs text-[var(--text-muted)]">
                          {c.email && <span>{c.email}</span>}
                          {c.baseUrl && <span className="font-mono ml-2">{c.baseUrl}</span>}
                        </div>
                      )}

                      {/* Token expiry bar — always visible */}
                      <TokenBar conn={c} />

                      {/* Status row — rate limit, backoff, errors */}
                      <StatusRow conn={c} />

                      {/* Reset cooldown inline */}
                      {(isRL || c.backoffLevel > 0) && (
                        <button onClick={() => handleResetCooldown(c.id)}
                          className="flex items-center gap-1 text-xs text-amber-400 hover:text-amber-300 transition-colors">
                          <RefreshCw size={11} /> Reset cooldown
                        </button>
                      )}

                      {/* Test result */}
                      {testResult[c.id] && (
                        <div className={`text-xs px-2 py-1 rounded ${testResult[c.id].status === 'ok' ? 'bg-green-900/30 text-green-400' : 'bg-red-900/30 text-red-400'}`}>
                          {testResult[c.id].loading ? 'Testing…' : testResult[c.id].status === 'ok'
                            ? `✓ OK${testResult[c.id].email ? ` (${testResult[c.id].email})` : ''}`
                            : `✗ ${testResult[c.id].message}`}
                        </div>
                      )}

                      {/* Model chips with test status */}
                      {c.supportedModels?.length > 0 && !expandedModels[c.id] && (
                        <div className="space-y-1.5">
                          <div className="flex items-center gap-2">
                            <span className="text-xs text-[var(--text-muted)]">Models ({c.supportedModels.length})</span>
                            <button
                              onClick={() => handleTestAllModels(c.id, c.supportedModels)}
                              className="flex items-center gap-1 text-xs text-[var(--accent)] hover:text-[var(--accent-hover)] transition-colors"
                              title="Test all models"
                            >
                              <Play size={10} /> Test All
                            </button>
                          </div>
                          <div className="flex flex-wrap gap-1.5">
                            {c.supportedModels.map((m: string, i: number) => {
                              const testResult = modelTestResults[c.id]?.[m]
                              return (
                                <div key={i} className="group/model relative inline-flex items-center gap-1">
                                  <span
                                    className={`text-xs font-mono px-1.5 py-0.5 rounded cursor-default ${
                                      testResult?.status === 'ok' ? 'bg-green-500/10 text-green-400 border border-green-500/20'
                                      : testResult?.status === 'error' ? 'bg-red-500/10 text-red-400 border border-red-500/20'
                                      : 'bg-[var(--accent)]/10 text-[var(--accent)]'
                                    }`}
                                    title={testResult?.message || m}
                                  >
                                    {testResult?.status === 'ok' && <CheckCircle2 size={10} className="inline mr-0.5 -mt-px" />}
                                    {testResult?.status === 'error' && <XCircle size={10} className="inline mr-0.5 -mt-px" />}
                                    {getModelName(m)}
                                  </span>
                                  <button
                                    onClick={(e) => { e.stopPropagation(); handleTestModel(c.id, m) }}
                                    disabled={testResult?.status === 'loading'}
                                    aria-label={`Test model ${m} on connection ${c.name}`}
                                    className="opacity-0 group-hover/model:opacity-100 absolute -top-1 -right-1 p-0.5 bg-[var(--bg-card)] border border-[var(--border)] rounded-full transition-opacity"
                                    title={`Test ${m}`}
                                  >
                                    {testResult?.status === 'loading'
                                      ? <Loader2 size={9} className="animate-spin text-[var(--accent)]" />
                                      : <Play size={9} className="text-[var(--accent)]" />
                                    }
                                  </button>
                                </div>
                              )
                            })}
                          </div>
                        </div>
                      )}
                    </div>
                  </div>

                  <div className="relative shrink-0" data-connection-menu-root>
                    <button
                      type="button"
                      onClick={() => setMenuOpenFor(menuOpenFor === c.id ? null : c.id)}
                      aria-expanded={menuOpenFor === c.id}
                      aria-haspopup="menu"
                      aria-label={`More actions for ${c.name}`}
                      className="rounded-lg p-2 text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text)]"
                    >
                      <MoreHorizontal size={18} />
                    </button>
                    {menuOpenFor === c.id && (
                      <div
                        role="menu"
                        className="absolute right-0 z-50 mt-1 min-w-[13.5rem] rounded-lg border border-[var(--border)] bg-[var(--bg-card)] py-1 shadow-xl ring-1 ring-black/20"
                      >
                        <button
                          type="button"
                          role="menuitem"
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-[var(--text)] hover:bg-[var(--bg-hover)]"
                          onClick={() => { handleTest(c.id); setMenuOpenFor(null) }}
                        >
                          <TestTube size={14} className="text-[var(--text-muted)]" />
                          Test connection
                        </button>
                        <button
                          type="button"
                          role="menuitem"
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-[var(--text)] hover:bg-[var(--bg-hover)] disabled:opacity-50"
                          disabled={quotaLoading[c.id]}
                          onClick={() => { void handleCheckQuota(c.id); setMenuOpenFor(null) }}
                        >
                          {quotaLoading[c.id]
                            ? <Loader2 size={14} className="animate-spin text-[var(--accent)]" />
                            : <BarChart2 size={14} className={quotaResult[c.id] ? 'text-[var(--accent)]' : 'text-[var(--text-muted)]'} />}
                          {quotaResult[c.id] ? 'Hide quota' : 'Show quota'}
                        </button>
                        <button
                          type="button"
                          role="menuitem"
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-[var(--text)] hover:bg-[var(--bg-hover)]"
                          onClick={() => { toggleModelEdit(c); setMenuOpenFor(null) }}
                        >
                          <Settings2 size={14} className={c.supportedModels?.length > 0 ? 'text-[var(--accent)]' : 'text-[var(--text-muted)]'} />
                          Allowed models
                        </button>
                        {(c.provider === 'openai' || c.provider === 'openai-compatible') && (
                          <button
                            type="button"
                            role="menuitem"
                            className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-[var(--text)] hover:bg-[var(--bg-hover)] disabled:opacity-50"
                            disabled={!!fetchingModels[c.id]}
                            onClick={() => { void handleFetchModels(c); setMenuOpenFor(null) }}
                          >
                            {fetchingModels[c.id]
                              ? <Loader2 size={14} className="animate-spin text-[var(--accent)]" />
                              : <Download size={14} className="text-[var(--text-muted)]" />}
                            Fetch models from API
                          </button>
                        )}
                        <div className="my-1 h-px bg-[var(--border)]" role="separator" />
                        <button
                          type="button"
                          role="menuitem"
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-[var(--danger)] hover:bg-red-500/10"
                          onClick={() => openDeleteDialog(c.id, c.name)}
                        >
                          <Trash2 size={14} />
                          Remove connection…
                        </button>
                      </div>
                    )}
                  </div>
                </div>

                {/* Quota panel */}
                {(quotaLoading[c.id] || quotaResult[c.id]) && (
                  <div className="border-t border-[var(--border)] px-4 py-3">
                    <div className="flex items-center gap-1.5 mb-2">
                      <BarChart2 size={12} className="text-[var(--accent)]" />
                      <span className="text-xs font-medium text-[var(--text-muted)]">Quota</span>
                    </div>
                    {quotaResult[c.id]?.error ? (
                      <p className="text-xs text-red-400">✗ {quotaResult[c.id].error}</p>
                    ) : (
                      <QuotaPanel data={quotaResult[c.id]} loading={quotaLoading[c.id]} />
                    )}
                  </div>
                )}

                {/* Model edit panel */}
                {expandedModels[c.id] && (
                  <div className="border-t border-[var(--border)] px-4 py-3 space-y-2">
                    <div className="flex items-center justify-between">
                      <label className="text-xs text-[var(--text-muted)]">Allowed Models</label>
                      {(c.provider === 'openai' || c.provider === 'openai-compatible') && (
                        <button onClick={() => handleFetchModels(c)} disabled={fetchingModels[c.id]}
                          className="flex items-center gap-1 px-2 py-0.5 bg-[var(--bg)] hover:bg-[var(--bg-hover)] rounded text-xs text-[var(--text-muted)] transition-colors">
                          {fetchingModels[c.id] ? <Loader2 size={12} className="animate-spin" /> : <Download size={12} />}
                          Fetch from API
                        </button>
                      )}
                    </div>
                    <ModelSelector
                      selected={editModels[c.id] ? editModels[c.id].split('\n').filter(Boolean) : []}
                      onChange={(models) => setEditModels(prev => ({ ...prev, [c.id]: models.join('\n') }))}
                      provider={c.provider}
                    />
                    {fetchedModels[c.id] && (
                      <p className="text-xs text-[var(--success)]">Fetched {fetchedModels[c.id].length} models from API</p>
                    )}
                    <div className="flex gap-2 pt-2">
                      <button onClick={() => handleSaveModels(c.id)}
                        className="px-3 py-1 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-xs">Save</button>
                      <button onClick={() => setExpandedModels(prev => ({ ...prev, [c.id]: false }))}
                        className="px-3 py-1 bg-[var(--bg)] hover:bg-[var(--bg-hover)] rounded-lg text-xs text-[var(--text-muted)]">Cancel</button>
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {deleteTarget && (
        <div
          className="fixed inset-0 z-[200] flex items-center justify-center bg-black/60 p-4"
          role="presentation"
          onMouseDown={e => { if (e.target === e.currentTarget) closeDeleteDialog() }}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="delete-connection-title"
            className="w-full max-w-md rounded-xl border border-[var(--border)] bg-[var(--bg-card)] p-5 shadow-2xl"
            onMouseDown={e => e.stopPropagation()}
          >
            <h3 id="delete-connection-title" className="text-lg font-semibold text-[var(--text)]">
              Remove connection?
            </h3>
            <p className="mt-2 text-sm text-[var(--text-muted)]">
              Hành động này không hoàn tác. Nhập đúng tên kết nối để xác nhận:
            </p>
            <p className="mt-2 rounded-lg border border-[var(--border)] bg-[var(--bg)] px-3 py-2 font-mono text-sm text-[var(--text)]">
              {deleteTarget.name}
            </p>
            <label htmlFor="delete-confirm-input" className="mt-4 block text-xs font-medium text-[var(--text-muted)]">
              Tên kết nối
            </label>
            <input
              id="delete-confirm-input"
              type="text"
              value={deleteConfirmName}
              onChange={e => setDeleteConfirmName(e.target.value)}
              autoComplete="off"
              placeholder={deleteTarget.name}
              disabled={deleteBusy}
              className="mt-1 w-full rounded-lg border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm outline-none focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 disabled:opacity-50"
            />
            <div className="mt-5 flex flex-wrap justify-end gap-2">
              <button
                type="button"
                onClick={closeDeleteDialog}
                disabled={deleteBusy}
                className="rounded-lg border border-[var(--border)] bg-[var(--bg)] px-4 py-2 text-sm text-[var(--text)] transition-colors hover:bg-[var(--bg-hover)] disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => void confirmDeleteConnection()}
                disabled={deleteBusy || deleteConfirmName !== deleteTarget.name}
                className="rounded-lg bg-[var(--danger)] px-4 py-2 text-sm font-medium text-white transition-colors hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {deleteBusy ? 'Removing…' : 'Remove connection'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

