import os

filepath = 'pages/connections.tsx'
with open(filepath, 'r', encoding='utf-8') as f:
    text = f.read()

# We want to replace from "// ─── Token bar" down to the end.
import re

start_marker = "// ─── Token bar ─────────────────────────────────────────────────────────────"

new_code = """// ─── Token bar ─────────────────────────────────────────────────────────────

function TokenBar({ conn }: { conn: any }) {
  if (!conn.expiresAt && !conn.hasApiKey) return null

  if (conn.hasApiKey) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-[var(--success)] bg-[var(--success)]/10 border border-[var(--success)]/20 px-2 py-0.5 rounded w-fit">
        <Shield size={11} /> API Key
      </div>
    )
  }

  const expMs = new Date(conn.expiresAt).getTime()
  const secsLeft = Math.ceil((expMs - Date.now()) / 1000)
  const expired = secsLeft <= 0

  if (expired) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-red-400 bg-red-400/10 border border-red-400/20 px-2 py-0.5 rounded w-fit">
        <X size={11} /> Token expired
      </div>
    )
  }

  return (
    <div className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] bg-[var(--bg)] border border-[var(--border)] px-2 py-0.5 rounded w-fit">
      <Check size={11} className="text-[var(--success)]" /> Token valid ({secsLeft > 3600*24 ? `${Math.floor(secsLeft/(3600*24))}d left` : secsToHuman(secsLeft)})
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
                  style={{ backgroundColor: c }}
                />
                <span className="text-[var(--text-muted)]">{b.label}</span>
              </span>
              <div className="flex items-center gap-2 text-[var(--text-muted)]">
                <span className="font-mono">
                  {fmt(b.used)}&nbsp;/&nbsp;{fmt(b.limit)}
                </span>
              </div>
            </div>
            <div className="h-1 bg-[var(--border)] rounded-full overflow-hidden">
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
    <div className="flex items-center gap-2 flex-wrap mt-1">
      {/* Rate limit */}
      {isRL && (
        <span className="inline-flex items-center gap-1 text-[10px] bg-amber-400/10 text-amber-400 border border-amber-400/20 px-1.5 py-0.5 rounded">
          ⏱ RL: {secsToHuman(rlSecs)}
        </span>
      )}

      {/* Backoff */}
      {conn.backoffLevel > 0 && (
        <span className="inline-flex items-center gap-1 text-[10px] bg-orange-400/10 text-orange-400 border border-orange-400/20 px-1.5 py-0.5 rounded">
          Backoff: {conn.backoffLevel}/7
        </span>
      )}

      {/* Model locks */}
      {lockCount > 0 && (
        <span className="inline-flex items-center gap-1 text-[10px] bg-orange-400/10 text-orange-400 border border-orange-400/20 px-1.5 py-0.5 rounded">
          🔒 {lockCount} models
        </span>
      )}

      {/* Last error */}
      {conn.lastError && (
        <span className="text-[10px] text-red-400 bg-red-400/10 border border-red-400/20 px-1.5 py-0.5 rounded truncate max-w-[150px]" title={conn.lastError}>
          ✗ {conn.lastError.slice(0, 40)}{conn.lastError.length > 40 ? '…' : ''}
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
          className="bg-[var(--bg)] border border-[var(--accent)] rounded px-2 py-0.5 text-sm font-medium outline-none w-48"
          onClick={e => e.stopPropagation()}
        />
        <button
          onClick={e => { e.stopPropagation(); save() }}
          disabled={saving}
          aria-label="Save connection name"
          className="p-0.5 text-[var(--success)] hover:opacity-80 disabled:opacity-50"
        >
          {saving ? <Loader2 size={12} className="animate-spin" /> : <Check size={13} />}
        </button>
        <button
          onClick={e => { e.stopPropagation(); cancel() }}
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
      <span className="font-semibold text-[var(--text)] line-clamp-1 truncate">{conn.name}</span>
      <button
        onClick={(e) => { e.stopPropagation(); setEditing(true) }}
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
  const [expandedQuota, setExpandedQuota] = useState<Record<string, boolean>>({})

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

  const parseSupportedModels = (str: string) => str.split('\\n').map(s => s.trim()).filter(Boolean)

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
    const rawModels = editModels[id] ? editModels[id].split('\\n').filter(Boolean) : []
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
      const withPrefix = (conn.supportedModels || []).map((m: string) =>
        m.includes('/') ? m : `${conn.provider}/${m}`
      )
      setEditModels(prev => ({ ...prev, [id]: withPrefix.join('\\n') }))
      setExpandedModels(prev => ({ ...prev, [id]: true }))
    }
  }

  const handleFetchModels = async (conn: any) => {
    setFetchingModels(prev => ({ ...prev, [conn.id]: true }))
    try {
      const res = await api.fetchConnectionModels(conn.id)
      const fetched = res.models || []
      setFetchedModels(prev => ({ ...prev, [conn.id]: fetched }))
      const withPrefix = fetched.map((m: string) => m.includes('/') ? m : `${conn.provider}/${m}`)
      setEditModels(prev => ({ ...prev, [conn.id]: withPrefix.join('\\n') }))
      setExpandedModels(prev => ({ ...prev, [conn.id]: true }))
    } catch (e: any) {
      if (conn.provider === 'openai' || conn.provider === 'openai-compatible') {
        const fallbacks = [
          'gpt-4o', 'gpt-4o-mini', 'o1', 'o1-mini', 'o3-mini', 'chatgpt-4o-latest', 
          'gpt-4-turbo', 'gpt-3.5-turbo', 'claude-3-5-sonnet-20240620', 'claude-3-opus-20240229'
        ]
        setFetchedModels(prev => ({ ...prev, [conn.id]: fallbacks }))
        setEditModels(prev => ({ ...prev, [conn.id]: fallbacks.join('\\n') }))
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
    <div className="space-y-3 mt-4">
      <div className="bg-[var(--bg)] rounded-xl p-5 border border-[var(--accent)]/30 text-center">
        <p className="text-sm text-[var(--text-muted)] mb-3">Enter this code on the authorization page:</p>
        <div className="text-3xl font-mono font-bold tracking-[0.3em] text-[var(--accent)] mb-4">
          {deviceCode.userCode}
        </div>
        <a href={deviceCode.verificationUriComplete || deviceCode.verificationUri} target="_blank" rel="noopener noreferrer"
          className="inline-flex items-center justify-center gap-2 px-6 py-2.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium text-white transition-colors shadow-lg shadow-[var(--accent)]/20">
          <ExternalLink size={16} /> Open Authorization Page
        </a>
      </div>
      {polling && (
        <div className="flex items-center justify-center gap-2 text-sm text-[var(--accent)] bg-[var(--accent)]/5 py-2 rounded-lg border border-[var(--accent)]/10">
          <Loader2 size={14} className="animate-spin" />
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
            Manage your AI provider accounts. Connect Kiro, OpenAI or any compatible endpoints.
          </p>
        </div>
        <button
          type="button"
          onClick={() => { setShowAdd(true); resetForm() }}
          disabled={showAdd}
          className="shrink-0 flex items-center justify-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors shadow-sm shadow-[var(--accent)]/20 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Plus size={16} /> Add Connection
        </button>
      </div>

      {/* Summary Cards */}
      {conns.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
          <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-white/5 shadow-sm relative overflow-hidden group hover:border-white/10 transition-colors">
            <div className="text-xs text-[var(--text-muted)] font-medium mb-1">Total Connections</div>
            <div className="text-2xl font-bold flex items-center gap-2">
              {connectionStats.total} <Link2 size={18} className="text-[var(--text-muted)] opacity-30" />
            </div>
            <div className="absolute right-0 bottom-0 w-24 h-24 bg-white/5 rounded-full blur-2xl -mr-12 -mb-12" />
          </div>
          <div className="bg-emerald-950/20 rounded-xl p-4 border border-emerald-900/40 shadow-sm relative overflow-hidden group hover:border-emerald-500/20 transition-colors">
            <div className="text-xs text-emerald-400 font-medium mb-1 flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-emerald-500" /> Active
            </div>
            <div className="text-2xl font-bold text-emerald-400">{connectionStats.active}</div>
            <div className="absolute right-0 bottom-0 w-24 h-24 bg-emerald-500/10 rounded-full blur-2xl -mr-12 -mb-12 group-hover:bg-emerald-500/20 transition-colors" />
          </div>
          <div className={`rounded-xl p-4 border border-rose-900/40 shadow-sm relative overflow-hidden group transition-colors ${connectionStats.needsAttention > 0 ? 'bg-rose-950/20 hover:border-rose-500/20' : 'bg-[var(--bg-card)] border-white/5 opacity-50'}`}>
            <div className={`text-xs font-medium mb-1 flex items-center gap-1.5 ${connectionStats.needsAttention > 0 ? 'text-rose-400' : 'text-[var(--text-muted)]'}`}>
              <AlertTriangle size={12} className={connectionStats.needsAttention > 0 ? '' : 'opacity-50'} /> Issues
            </div>
            <div className={`text-2xl font-bold ${connectionStats.needsAttention > 0 ? 'text-rose-400' : 'text-[var(--text-muted)]'}`}>{connectionStats.needsAttention}</div>
            {connectionStats.needsAttention > 0 && <div className="absolute right-0 bottom-0 w-24 h-24 bg-rose-500/10 rounded-full blur-2xl -mr-12 -mb-12 group-hover:bg-rose-500/20 transition-colors" />}
          </div>
        </div>
      )}

      {/* Toolbar: search + sort + refresh quota */}
      {conns.length > 0 && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4 mb-5">
          <div className="relative flex-1 min-w-0">
            <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)] pointer-events-none" />
            <input
              type="search"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Search by name, provider, email, URL, model…"
              className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg py-2 pl-9 pr-3 text-sm focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:text-[var(--text-muted)]/60"
            />
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <div className="relative">
              <ArrowDownAZ size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] pointer-events-none" />
              <select
                value={sortMode}
                onChange={e => setSortMode(e.target.value as SortMode)}
                className="w-36 bg-[var(--bg-card)] border border-[var(--border)] rounded-lg py-2 pl-8 pr-7 text-sm appearance-none cursor-pointer focus:border-[var(--accent)] outline-none transition-colors"
              >
                <option value="name">Name (A-Z)</option>
                <option value="issues">Issues First</option>
                <option value="provider">By Provider</option>
              </select>
              <span className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] pointer-events-none text-[10px]">▼</span>
            </div>
            <label className="flex cursor-pointer items-center gap-2 text-sm text-[var(--text-muted)] hover:text-[var(--text)] transition-colors select-none">
              <input
                type="checkbox"
                checked={autoRefreshQuota}
                onChange={e => setAutoRefreshQuota(e.target.checked)}
                className="accent-[var(--accent)] w-3.5 h-3.5 rounded"
              />
              Auto check quota
            </label>
          </div>
        </div>
      )}

      {/* Add Wizard Modal */}
      {showAdd && (
        <div className="rounded-xl border border-white/10 bg-[var(--bg-card)] shadow-2xl mb-8 overflow-hidden backdrop-blur-sm relative animate-in slide-in-from-top-4 fade-in duration-200">
          {/* Header */}
          <div className="flex items-center justify-between p-5 border-b border-white/5 bg-gradient-to-r from-transparent to-white/[0.02]">
            <div>
              <h3 className="text-lg font-semibold text-[var(--text)]">Add New Connection</h3>
              <p className="text-sm text-[var(--text-muted)] mt-0.5">Follow the steps below to configure your AI provider account.</p>
            </div>
            <button
              onClick={() => { setShowAdd(false); resetForm() }}
              className="p-2 rounded-lg text-[var(--text-muted)] hover:bg-[var(--bg)] transition-colors self-start"
              aria-label="Close"
            >
              <X size={20} />
            </button>
          </div>

          <div className="p-6 sm:p-8 space-y-8">
            {/* Step 1: Provider */}
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <div className="w-5 h-5 rounded-full bg-[var(--accent)]/20 text-[var(--accent)] text-xs flex items-center justify-center font-bold">1</div>
                <h4 className="font-medium text-sm">Select Provider</h4>
              </div>
              
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                {[
                  { id: 'kiro', name: 'Kiro AI', icon: 'KI', desc: 'AWS Builder ID, Social, IDC', color: '#3b82f6' },
                  { id: 'openai', name: 'OpenAI', icon: 'OA', desc: 'OAuth Login, API Key', color: '#10a37f' },
                  { id: 'openai-compatible', name: 'Custom API', icon: 'OC', desc: 'Any OpenAI-compatible server', color: '#a855f7' }
                ].map(p => (
                  <button
                    key={p.id}
                    onClick={() => { setProvider(p.id); resetForm() }}
                    className={`relative p-5 rounded-xl border text-left transition-all overflow-hidden group ${
                      provider === p.id
                        ? `border-[${p.color}] bg-[${p.color}]/5 ring-1 ring-[${p.color}]/50`
                        : 'border-[var(--border)] bg-[var(--bg)] hover:border-white/20'
                    }`}
                    style={provider === p.id ? { borderColor: p.color, backgroundColor: `${p.color}0A`, boxShadow: `0 0 0 1px ${p.color}40` } : {}}
                  >
                    <div className="absolute -top-6 -right-6 w-24 h-24 rounded-full opacity-20 blur-xl transition-all group-hover:opacity-40" style={{ backgroundColor: p.color }} />
                    {provider === p.id && (
                      <div className="absolute top-3 right-3" style={{ color: p.color }}>
                        <CheckCircle2 size={18} />
                      </div>
                    )}
                    <div className="flex items-center gap-4 mb-2">
                      <div className="w-12 h-12 rounded-full flex items-center justify-center font-bold text-lg shadow-inner z-10" 
                           style={{ backgroundColor: `${p.color}15`, color: p.color, border: `1px solid ${p.color}30` }}>
                        {p.icon}
                      </div>
                      <div className="z-10">
                        <div className="font-semibold text-base">{p.name}</div>
                      </div>
                    </div>
                    <div className="text-xs text-[var(--text-muted)] line-clamp-1 mt-3 z-10 relative">{p.desc}</div>
                  </button>
                ))}
              </div>
            </div>

            {/* Step 2: Auth Method (for kiro) */}
            {provider === 'kiro' && (
              <div className="space-y-4 animate-in fade-in slide-in-from-top-2 duration-300">
                <div className="flex items-center gap-2">
                  <div className="w-5 h-5 rounded-full bg-[var(--accent)]/20 text-[var(--accent)] text-xs flex items-center justify-center font-bold">2</div>
                  <h4 className="font-medium text-sm">Authentication Method</h4>
                </div>
                
                <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
                  {[
                    { id: 'detect' as ImportMode, label: 'Auto Detect', desc: 'Scan IDE tokens automatically', icon: <Search size={20} /> },
                    { id: 'builder-id' as ImportMode, label: 'Builder ID', desc: 'Authenticate via AWS Builder ID', icon: <ExternalLink size={20} /> },
                    { id: 'social' as ImportMode, label: 'Social Login', desc: 'Login with Google or GitHub', icon: <Globe size={20} /> },
                    { id: 'idc' as ImportMode, label: 'IAM IDC', desc: 'Enterprise Identity Center SSO', icon: <Shield size={20} /> },
                    { id: 'file' as ImportMode, label: 'Import File', desc: 'Upload kiro-auth-token.json', icon: <Upload size={20} /> },
                    { id: 'manual' as ImportMode, label: 'Paste Token', desc: 'Manual config and refresh token', icon: <Link2 size={20} /> }
                  ].map(m => (
                    <button
                      key={m.id}
                      onClick={() => { setImportMode(m.id); setDeviceCode(null); setPolling(false); setSocialLogin(null); setError(''); setSuccess('') }}
                      className={`flex flex-col p-4 rounded-xl border text-left transition-colors ${
                        importMode === m.id
                          ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)] shadow-[0_0_0_1px_rgba(59,130,246,0.3)]'
                          : 'border-[var(--border)] bg-[var(--bg)] text-[var(--text-muted)] hover:border-white/20 hover:text-[var(--text)]'
                      }`}
                    >
                      <div className="mb-3 p-2 rounded-lg bg-[var(--bg-card)] max-w-fit border border-[var(--border)] shadow-sm">
                        {m.icon}
                      </div>
                      <div className="font-semibold text-sm mb-0.5 text-[var(--text)]">{m.label}</div>
                      <div className="text-xs opacity-70 line-clamp-2 pr-2">{m.desc}</div>
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Step 3: Form Fields */}
            <div className="space-y-4 animate-in fade-in slide-in-from-top-2 duration-300">
              <div className="flex items-center gap-2">
                <div className="w-5 h-5 rounded-full bg-[var(--accent)]/20 text-[var(--accent)] text-xs flex items-center justify-center font-bold">
                  {provider === 'kiro' ? '3' : '2'}
                </div>
                <h4 className="font-medium text-sm">Configure</h4>
              </div>

              <div className="p-6 rounded-xl border border-[var(--border)] bg-[var(--bg)]">
                
                {/* ── Kiro Form ── */}
                {provider === 'kiro' && (
                  <div>
                    {importMode === 'detect' && (
                      <div className="space-y-4">
                        <p className="text-sm text-[var(--text-muted)]">Automatically discover credentials from <code className="text-xs bg-[var(--bg-card)] px-1.5 py-0.5 rounded border border-[var(--border)] font-mono">~/.aws/sso/cache/kiro-auth-token.json</code></p>
                        <button onClick={handleDetect} disabled={loading}
                          className="flex items-center gap-2 px-5 py-2.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white w-fit">
                          {loading ? <Loader2 size={16} className="animate-spin" /> : <Search size={16} />} 
                          {loading ? 'Detecting…' : 'Scan & Import'}
                        </button>
                      </div>
                    )}

                    {importMode === 'builder-id' && (
                      <div className="space-y-4">
                        <p className="text-sm text-[var(--text-muted)]">Authenticate via AWS Builder ID (Device Code Flow).</p>
                        {!deviceCode && (
                          <button onClick={handleStartBuilderID} disabled={loading}
                            className="flex items-center gap-2 px-5 py-2.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white w-fit">
                            {loading ? <Loader2 size={16} className="animate-spin" /> : <ExternalLink size={16} />}
                            {loading ? 'Starting Flow…' : 'Start Builder ID Login'}
                          </button>
                        )}
                        <DeviceCodePanel />
                      </div>
                    )}

                    {importMode === 'idc' && (
                      <div className="space-y-5">
                        <p className="text-sm text-[var(--text-muted)]">Authenticate via AWS IAM Identity Center (Enterprise SSO).</p>
                        {!deviceCode && (
                          <>
                            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 max-w-2xl">
                              <div>
                                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Start URL <span className="text-red-400">*</span></label>
                                <input value={idcForm.startUrl} onChange={e => setIdcForm({ ...idcForm, startUrl: e.target.value })}
                                  placeholder="https://mycompany.awsapps.com/start"
                                  className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                              </div>
                              <div>
                                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Region</label>
                                <input value={idcForm.region} onChange={e => setIdcForm({ ...idcForm, region: e.target.value })}
                                  placeholder="us-east-1"
                                  className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                              </div>
                            </div>
                            <button onClick={handleStartIDC} disabled={loading || !idcForm.startUrl}
                              className="flex items-center gap-2 px-5 py-2.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white w-fit">
                              {loading ? <Loader2 size={16} className="animate-spin" /> : <ExternalLink size={16} />}
                              {loading ? 'Starting Flow…' : 'Start IDC Login'}
                            </button>
                          </>
                        )}
                        <DeviceCodePanel />
                      </div>
                    )}

                    {importMode === 'social' && (
                      <div className="space-y-5">
                        <p className="text-sm text-[var(--text-muted)]">Authenticate with Google or GitHub via Kiro Identity.</p>
                        {!socialLogin && (
                          <>
                            <div className="flex gap-3">
                              {(['google', 'github'] as const).map(p => (
                                <button key={p} onClick={() => setSocialProvider(p)}
                                  className={`flex flex-col sm:flex-row items-center justify-center gap-2 px-6 py-3 rounded-lg border text-sm font-medium transition-all ${socialProvider === p ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]' : 'border-[var(--border)] bg-[var(--bg-card)] hover:border-white/20'}`}>
                                  {p === 'google' ? <Globe size={18} /> : <GitBranch size={18} />}
                                  {p === 'google' ? 'Google' : 'GitHub'}
                                </button>
                              ))}
                            </div>
                            <button onClick={handleStartSocial} disabled={loading}
                              className="flex items-center gap-2 px-5 py-2.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white w-fit">
                              {loading ? <Loader2 size={16} className="animate-spin" /> : <Globe size={16} />}
                              Login with {socialProvider === 'google' ? 'Google' : 'GitHub'}
                            </button>
                          </>
                        )}
                        {socialLogin && (
                          <div className="space-y-4 max-w-2xl">
                            <div className="bg-[var(--bg-card)] rounded-lg p-4 border border-blue-500/20 text-sm text-[var(--text-muted)] space-y-2">
                              <p className="flex items-center gap-2"><span className="w-5 h-5 rounded bg-blue-500/10 text-blue-400 flex items-center justify-center text-xs font-bold">1</span> The login page has been opened in your browser.</p>
                              <p className="flex items-center gap-2 text-[var(--text)]"><span className="w-5 h-5 rounded bg-blue-500/10 text-blue-400 flex items-center justify-center text-xs font-bold">2</span> After logging in, you will be redirected to an address starting with <code className="bg-[var(--bg)] px-1.5 py-0.5 rounded border border-[var(--border)] font-mono text-xs">kiro://</code>.</p>
                              <p className="flex items-center gap-2"><span className="w-5 h-5 rounded bg-blue-500/10 text-blue-400 flex items-center justify-center text-xs font-bold">3</span> Copy that entire URL and paste it below.</p>
                            </div>
                            <div>
                              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Callback URL</label>
                              <input value={socialCallbackUrl} onChange={e => setSocialCallbackUrl(e.target.value)}
                                placeholder="kiro://kiro.kiroAgent/authenticate-success?code=…&state=…"
                                className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                            </div>
                            <div className="flex gap-3">
                              <button onClick={handleExchangeSocial} disabled={loading || !socialCallbackUrl}
                                className="px-5 py-2.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white">
                                {loading ? 'Processing…' : 'Submit Callback URI'}
                              </button>
                              <a href={socialLogin.loginUrl} target="_blank" rel="noopener noreferrer"
                                className="flex items-center gap-1.5 px-4 py-2.5 bg-[var(--bg-card)] hover:bg-[var(--border)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-muted)] transition-colors">
                                <ExternalLink size={14} /> Re-open browser
                              </a>
                            </div>
                          </div>
                        )}
                      </div>
                    )}

                    {importMode === 'file' && (
                      <div className="space-y-4">
                        <p className="text-sm text-[var(--text-muted)]">Upload the <code className="text-xs bg-[var(--bg-card)] px-1.5 py-0.5 rounded border border-[var(--border)] font-mono">kiro-auth-token.json</code> configuration file.</p>
                        <label className="flex flex-col items-center justify-center gap-3 px-6 py-10 bg-[var(--bg-card)] border-2 border-dashed border-[var(--border)] rounded-xl cursor-pointer hover:border-[var(--accent)] hover:bg-[var(--bg-card)]/80 transition-colors w-full max-w-md group">
                          <div className="w-12 h-12 rounded-full bg-[var(--bg)] flex items-center justify-center group-hover:scale-110 transition-transform">
                            <Upload size={20} className="text-[var(--text-muted)] group-hover:text-[var(--accent)]" />
                          </div>
                          <span className="text-sm font-medium text-[var(--text)]">{loading ? 'Processing File…' : 'Click to select JSON file'}</span>
                          <span className="text-xs text-[var(--text-muted)]">Max file size 1MB</span>
                          <input type="file" accept=".json" onChange={handleFileUpload} className="hidden" disabled={loading} />
                        </label>
                      </div>
                    )}

                    {importMode === 'manual' && (
                      <div className="space-y-5 max-w-2xl">
                        <div>
                          <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Refresh Token <span className="text-red-400">*</span></label>
                          <textarea value={form.refreshToken} onChange={e => setForm({ ...form, refreshToken: e.target.value })}
                            placeholder="eyJhbGciOiJIUzI1NiIsInR..."
                            rows={3}
                            className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50 resize-y" />
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                          <div>
                            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Auth Method</label>
                            <select value={form.authMethod} onChange={e => setForm({ ...form, authMethod: e.target.value })}
                              className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all appearance-none cursor-pointer">
                              <option value="builder-id">AWS Builder ID</option>
                              <option value="idc">AWS IAM Identity Center</option>
                              <option value="social">Social Login</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Region (if applicable)</label>
                            <input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })}
                              placeholder="us-east-1"
                              className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                          </div>
                        </div>
                        <button onClick={handleManualImport} disabled={loading || !form.refreshToken}
                          className="px-5 py-2.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white">
                          {loading ? 'Validating…' : 'Import Configuration'}
                        </button>
                      </div>
                    )}
                  </div>
                )}

                {/* ── OpenAI Form ── */}
                {provider === 'openai' && (
                  <div className="space-y-6 max-w-2xl">
                    {/* Mode toggle */}
                    <div className="flex p-1 bg-[var(--bg-card)] rounded-lg border border-[var(--border)] w-fit">
                      <button onClick={() => { setOpenaiMode('oauth'); setOpenaiOAuthSession(null); setError(''); setSuccess('') }}
                        className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${openaiMode === 'oauth' ? 'bg-[var(--accent)] text-white shadow-sm' : 'text-[var(--text-muted)] hover:text-[var(--text)]'}`}>
                        <Globe size={16} /> OAuth Flow
                      </button>
                      <button onClick={() => { setOpenaiMode('apikey'); setOpenaiOAuthSession(null); setError(''); setSuccess('') }}
                        className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${openaiMode === 'apikey' ? 'bg-[var(--accent)] text-white shadow-sm' : 'text-[var(--text-muted)] hover:text-[var(--text)]'}`}>
                        <Shield size={16} /> API Key
                      </button>
                    </div>

                    {/* OAuth flow */}
                    {openaiMode === 'oauth' && (
                      <div className="space-y-5">
                        {!openaiOAuthSession ? (
                          <>
                            <div className="bg-[var(--bg-card)] rounded-xl p-5 border border-[var(--border)] shadow-sm">
                              <div className="flex items-center gap-3 text-[var(--text)] mb-2 font-medium">
                                <Shield size={18} className="text-[#10a37f]" /> Secure OAuth Login
                              </div>
                              <p className="text-sm text-[var(--text-muted)] leading-relaxed">
                                Connect your parent OpenAI account safely. You will be redirected to the official OpenAI login page. Once authorized, dntproxy will securely store your token.
                              </p>
                            </div>
                            <button onClick={async () => {
                              setLoading(true); setError(''); setSuccess('')
                              try {
                                const res = await api.startOpenAIOAuth()
                                setOpenaiOAuthSession({ sessionId: res.sessionId, authUrl: res.authUrl })
                                window.open(res.authUrl, '_blank')
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
                              className="flex items-center gap-2 px-5 py-2.5 bg-[#10a37f] hover:bg-[#0d8a6b] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white w-fit shadow-md shadow-[#10a37f]/20">
                              {loading ? <Loader2 size={16} className="animate-spin" /> : <ExternalLink size={16} />}
                              {loading ? 'Starting…' : 'Proceed to OpenAI.com'}
                            </button>
                          </>
                        ) : (
                          <div className="space-y-4">
                            <div className="bg-[var(--bg-card)] rounded-xl p-6 border border-[#10a37f]/30 text-center shadow-[0_0_15px_rgba(16,163,127,0.05)]">
                              <Loader2 size={24} className="animate-spin text-[#10a37f] mx-auto mb-4" />
                              <h4 className="font-medium text-[var(--text)] mb-1">Waiting for authorization</h4>
                              <p className="text-sm text-[var(--text-muted)] mb-5">Please complete the login in your browser.</p>
                              <a href={openaiOAuthSession.authUrl} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 text-sm font-medium text-[#10a37f] hover:underline">
                                Click here to open browser manually <ExternalLink size={14} />
                              </a>
                            </div>
                            <button onClick={() => { if (openaiPollRef.current) clearInterval(openaiPollRef.current); setOpenaiOAuthSession(null); setError('') }}
                              className="px-4 py-2 bg-[var(--bg-card)] hover:bg-[var(--border)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] transition-colors font-medium">
                              Cancel
                            </button>
                          </div>
                        )}
                      </div>
                    )}

                    {/* API Key mode */}
                    {openaiMode === 'apikey' && (
                      <div className="space-y-5">
                        <div className="bg-amber-500/10 text-amber-500/90 rounded-lg p-3 border border-amber-500/20 text-xs flex items-center gap-2">
                          <Shield size={14} className="shrink-0" />
                          <span>Need an API Key? Create one at <a href="https://platform.openai.com/api-keys" target="_blank" rel="noopener noreferrer" className="underline font-medium hover:text-amber-400">platform.openai.com</a>.</span>
                        </div>
                        
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                          <div className="sm:col-span-2">
                            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">API Key <span className="text-red-400">*</span></label>
                            <input type="password" value={openaiForm.apiKey} onChange={e => setOpenaiForm({ ...openaiForm, apiKey: e.target.value })}
                              placeholder="sk-proj-…"
                              className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                          </div>
                          <div className="sm:col-span-2">
                            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Display Name <span className="font-normal opacity-70">(Optional)</span></label>
                            <input value={openaiForm.name} onChange={e => setOpenaiForm({ ...openaiForm, name: e.target.value })}
                              placeholder="My OpenAI Key"
                              className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                          </div>
                        </div>

                        <button onClick={handleAddOpenAI} disabled={loading || !openaiForm.apiKey}
                          className="px-5 py-2.5 bg-[#10a37f] hover:bg-[#0d8a6b] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white shadow-md shadow-[#10a37f]/20">
                          {loading ? 'Adding Connection…' : 'Add OpenAI Configuration'}
                        </button>
                      </div>
                    )}
                  </div>
                )}

                {/* ── Custom URL Form ── */}
                {provider === 'openai-compatible' && (
                  <div className="space-y-5 max-w-2xl">
                    <div>
                      <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Base URL <span className="text-red-400">*</span></label>
                      <input value={customForm.baseUrl} onChange={e => setCustomForm({ ...customForm, baseUrl: e.target.value })}
                        placeholder="https://api.together.xyz/v1"
                        className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                      <p className="text-[10px] text-[var(--text-muted)] mt-1.5 ml-1">Must be the base path that includes chat/completions (e.g. end with /v1 typically).</p>
                    </div>
                    
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">API Key <span className="font-normal opacity-70">(Optional)</span></label>
                        <input type="password" value={customForm.apiKey} onChange={e => setCustomForm({ ...customForm, apiKey: e.target.value })}
                          placeholder="sk-…"
                          className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                      </div>
                      <div>
                        <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">Display Name <span className="font-normal opacity-70">(Optional)</span></label>
                        <input value={customForm.name} onChange={e => setCustomForm({ ...customForm, name: e.target.value })}
                          placeholder="Together AI / Local Llama"
                          className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3.5 py-2.5 text-sm focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/40 outline-none transition-all placeholder:opacity-50" />
                      </div>
                    </div>

                    <button onClick={handleAddCustom} disabled={loading || !customForm.baseUrl}
                      className="px-5 py-2.5 bg-[#a855f7] hover:bg-[#9333ea] rounded-lg text-sm font-medium transition-colors disabled:opacity-50 text-white shadow-md shadow-[#a855f7]/20">
                      {loading ? 'Adding Connection…' : 'Add Custom Configuration'}
                    </button>
                  </div>
                )}
              </div>

            </div>

            {/* Error/Success Feedback */}
            {error && (
              <div className="p-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-500 text-sm flex items-start gap-2">
                <AlertTriangle size={18} className="shrink-0 mt-0.5" />
                <span className="leading-relaxed">{error}</span>
              </div>
            )}
            {success && (
              <div className="p-4 rounded-lg bg-green-500/10 border border-green-500/20 text-green-500 text-sm flex items-center gap-2">
                <CheckCircle2 size={18} className="shrink-0" />
                <span>{success}</span>
              </div>
            )}
            
          </div>
        </div>
      )}

      {/* ── Connection Cards Grid ────────────────────────────────────────────── */}
      {conns.length === 0 ? (
        <div className="rounded-xl border border-dashed border-[var(--border)] bg-[var(--bg-card)]/30 px-6 py-16 text-center shadow-inner">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-[var(--accent)]/10 text-[var(--accent)] mb-4">
            <Link2 size={26} strokeWidth={2} />
          </div>
          <h3 className="text-lg font-semibold text-[var(--text)]">No connections yet</h3>
          <p className="mx-auto mt-2 max-w-sm text-sm text-[var(--text-muted)] leading-relaxed">
            Add Kiro AI or OpenAI accounts to begin routing requests. Select from auto-detect, Builder ID, OAuth or API Keys.
          </p>
          <button
            type="button"
            onClick={() => { setShowAdd(true); resetForm() }}
            className="mt-6 inline-flex items-center gap-2 rounded-lg bg-[var(--accent)] px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-[var(--accent-hover)] shadow-md shadow-[var(--accent)]/20 hover:shadow-[var(--accent)]/30 group"
          >
            <Plus size={18} className="group-hover:rotate-90 transition-transform" /> Connect Now
          </button>
        </div>
      ) : filteredConns.length === 0 ? (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-card)] px-6 py-12 text-center">
          <p className="text-sm text-[var(--text-muted)]">No connections matching “{searchQuery.trim()}”</p>
          <button
            type="button"
            onClick={() => setSearchQuery('')}
            className="mt-3 text-sm font-medium text-[var(--accent)] hover:text-[var(--accent-hover)]"
          >
            Clear filters
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {sortedConns.map((c: any) => {
            const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
            const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date()
            const hasIssue = isRL || isExpired || c.backoffLevel > 0 || c.lastError
            const statusColor = !c.isActive ? '#94a3b8' // gray
              : isRL ? '#fbbf24' // amber
              : hasIssue ? '#ef4444' // red
              : '#22c55e' // green
            
            const providerInfo = (() => {
              if (c.provider === 'kiro') return { icon: 'KI', bg: '#3b82f6', border: '#1d4ed8' }
              if (c.provider === 'openai') return { icon: 'OA', bg: '#10a37f', border: '#065f46' }
              return { icon: 'OC', bg: '#a855f7', border: '#6b21a8' } // Custom
            })()

            return (
              <div key={c.id}
                className={`bg-[var(--bg-card)] rounded-2xl flex flex-col transition-all duration-200 shadow-sm hover:shadow-lg hover:-translate-y-0.5 border ${
                  !c.isActive ? 'opacity-60 border-white/5 hover:border-white/10' : 
                  isRL ? 'border-amber-500/20 hover:border-amber-500/40 shadow-[0_0_15px_rgba(251,191,36,0.05)]' : 
                  hasIssue ? 'border-red-500/20 hover:border-red-500/40 shadow-[0_0_15px_rgba(239,68,68,0.05)]' : 
                  'border-white/5 hover:border-white/20'
                }`}>

                {/* Top: Avatar & Name */}
                <div className="p-4 pb-3 flex items-start gap-3">
                  <div className="relative shrink-0">
                    <div className="w-10 h-10 rounded-full flex items-center justify-center font-bold text-white shadow-inner bg-gradient-to-br"
                         style={{ from: providerInfo.bg, to: providerInfo.border, backgroundColor: providerInfo.bg, borderColor: providerInfo.border, borderWidth: '1px' }}>
                      {providerInfo.icon}
                    </div>
                    {/* Status Dot */}
                    <span 
                      className="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full border-2 border-[var(--bg-card)] z-10" 
                      style={{ backgroundColor: statusColor }}
                    />
                    {isRL && <span className="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full z-0 animate-ping" style={{ backgroundColor: statusColor, opacity: 0.6 }} />}
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between mb-0.5">
                      <InlineName conn={c} onRename={handleRename} />
                      <button onClick={() => handleToggle(c.id, c.isActive)}
                        aria-label={`${c.isActive ? 'Disable' : 'Enable'} ${c.name}`}
                        className={`relative w-8 h-4.5 rounded-full transition-colors duration-200 shrink-0 ${c.isActive ? 'bg-[var(--accent)]' : 'bg-[var(--border)]'}`}>
                        <span className={`absolute top-0.5 left-0.5 w-3.5 h-3.5 rounded-full bg-white shadow transition-transform duration-200 ${c.isActive ? 'translate-x-3.5' : 'translate-x-0'}`} />
                      </button>
                    </div>
                    
                    <div className="flex flex-wrap items-center gap-1.5 text-xs text-[var(--text-muted)] mt-1">
                      {c.authType === 'apikey' && (
                        <span className="bg-white/5 border border-white/5 px-1.5 py-0.5 rounded text-[10px] uppercase font-medium tracking-wide">API Key</span>
                      )}
                      {(c.providerName || c.authMethod) && c.authType !== 'apikey' && (
                        <span className="bg-white/5 border border-white/5 px-1.5 py-0.5 rounded text-[10px] uppercase font-medium tracking-wide">{c.providerName || c.authMethod}</span>
                      )}
                      {(c.email || c.baseUrl) && (
                        <span className="truncate pr-2 border-l border-white/10 pl-1.5">{c.email || c.baseUrl.replace('https://', '')}</span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Middle: Bars & Info */}
                <div className="px-4 py-2 flex-1 space-y-3">
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <TokenBar conn={c} />
                    <StatusRow conn={c} />
                    {testResult[c.id] && (
                      <div className={`px-2 py-0.5 rounded text-[10px] font-medium border ${testResult[c.id].status === 'ok' ? 'bg-green-500/10 text-green-400 border-green-500/20' : 'bg-red-500/10 text-red-400 border-red-500/20'}`}>
                        {testResult[c.id].loading ? 'Testing…' : testResult[c.id].status === 'ok' ? '✓ Tested OK' : '✗ Test Failed'}
                      </div>
                    )}
                  </div>

                  {/* Inline Quota (collapsed by default visually, expands if checked) */}
                  {(quotaLoading[c.id] || quotaResult[c.id]) && expandedQuota[c.id] && (
                    <div className="bg-black/20 rounded-lg p-3 border border-white/5 mt-2 animate-in fade-in slide-in-from-top-2">
                       {quotaResult[c.id]?.error ? (
                        <p className="text-xs text-red-400">✗ {quotaResult[c.id].error}</p>
                      ) : (
                        <QuotaPanel data={quotaResult[c.id]} loading={quotaLoading[c.id]} />
                      )}
                    </div>
                  )}

                  {/* Inline Models Panel */}
                  {expandedModels[c.id] && (
                    <div className="bg-black/20 rounded-lg p-3 border border-white/5 mt-2 animate-in fade-in slide-in-from-top-2">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-xs font-semibold text-[var(--text)]">Allowed Models</span>
                        {(c.provider === 'openai' || c.provider === 'openai-compatible') && (
                          <button onClick={() => handleFetchModels(c)} disabled={fetchingModels[c.id]}
                            className="flex items-center gap-1.5 px-2 py-1 bg-[var(--bg-card)] border border-[var(--border)] hover:bg-[var(--bg-hover)] rounded text-[10px] text-[var(--text-muted)] transition-colors font-medium">
                            {fetchingModels[c.id] ? <Loader2 size={10} className="animate-spin" /> : <Download size={10} />} Fetch API
                          </button>
                        )}
                      </div>
                      <div className="mb-2">
                        <ModelSelector
                          selected={editModels[c.id] ? editModels[c.id].split('\\n').filter(Boolean) : []}
                          onChange={(models) => setEditModels(prev => ({ ...prev, [c.id]: models.join('\\n') }))}
                          provider={c.provider}
                        />
                      </div>
                      <div className="flex items-center gap-2 mt-3 pt-2 border-t border-[var(--border)]">
                        <button onClick={() => handleSaveModels(c.id)}
                          className="px-3 py-1.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-md text-xs font-medium text-white transition-colors">Save Models</button>
                        <button onClick={() => setExpandedModels(prev => ({ ...prev, [c.id]: false }))}
                          className="px-3 py-1.5 bg-[var(--bg-card)] border border-[var(--border)] hover:bg-[var(--bg-hover)] rounded-md text-xs font-medium transition-colors">Cancel</button>
                      </div>
                    </div>
                  )}

                  {/* Model Chips (Collapsed view) */}
                  {c.supportedModels?.length > 0 && !expandedModels[c.id] && (
                    <div className="flex flex-wrap gap-1.5 pt-1">
                      {c.supportedModels.slice(0, 5).map((m: string, i: number) => {
                        const mTest = modelTestResults[c.id]?.[m]
                        return (
                          <div key={i} className="group/model relative">
                             <div className={`px-2 py-0.5 rounded text-[10px] font-mono cursor-default border flex items-center gap-1 transition-colors ${
                                mTest?.status === 'ok' ? 'bg-green-500/10 text-green-400 border-green-500/20'
                                : mTest?.status === 'error' ? 'bg-red-500/10 text-red-400 border-red-500/20'
                                : 'bg-white/5 text-[var(--text-muted)] border-white/5 hover:border-white/10 hover:text-[var(--text)]'
                              }`}>
                                {mTest?.status === 'loading' && <Loader2 size={8} className="animate-spin" />}
                                {getModelName(m)}
                             </div>
                             <div className="absolute opacity-0 group-hover/model:opacity-100 -top-full left-1/2 -translate-x-1/2 -mt-1 pb-1 pointer-events-none transition-opacity z-10">
                               <button onClick={(e) => { e.stopPropagation(); handleTestModel(c.id, m) }}
                                 className="pointer-events-auto bg-[var(--bg)] border border-[var(--accent)]/50 text-[var(--accent)] rounded p-1 shadow-lg hover:bg-[var(--accent)] hover:text-white transition-colors">
                                 <Play size={10} className="fill-current" />
                               </button>
                             </div>
                          </div>
                        )
                      })}
                      {c.supportedModels.length > 5 && (
                        <div className="px-1.5 py-0.5 rounded text-[10px] font-medium text-[var(--text-muted)] bg-white/5 border border-transparent">
                          +{c.supportedModels.length - 5}
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Bottom Action Bar */}
                <div className="flex items-center mt-2 border-t border-white/5 bg-black/10 rounded-b-2xl overflow-hidden divide-x divide-white/5">
                  <button onClick={() => handleTest(c.id)}
                    className="flex-1 flex items-center justify-center gap-1.5 py-3 text-xs font-medium text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-white/5 transition-colors group">
                    <TestTube size={14} className="group-hover:text-blue-400 opacity-70 group-hover:opacity-100 transition-colors" /> Test
                  </button>
                  <button onClick={() => {
                      if (!quotaResult[c.id]) handleCheckQuota(c.id);
                      setExpandedQuota(prev => ({ ...prev, [c.id]: !prev[c.id] }))
                    }}
                    className={`flex-1 flex items-center justify-center gap-1.5 py-3 text-xs font-medium transition-colors group ${expandedQuota[c.id] ? 'text-[var(--accent)] bg-[var(--accent)]/5' : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-white/5'}`}>
                    {quotaLoading[c.id] ? <Loader2 size={14} className="animate-spin" /> : <BarChart2 size={14} className={`opacity-70 group-hover:opacity-100 transition-colors ${expandedQuota[c.id] ? 'text-[var(--accent)]' : 'group-hover:text-emerald-400'}`} />} Quota
                  </button>
                  <button onClick={() => { setExpandedModels(prev => ({ ...prev, [c.id]: !prev[c.id] })); if(!expandedModels[c.id]) toggleModelEdit(c); }}
                    className={`flex-1 flex items-center justify-center gap-1.5 py-3 text-xs font-medium transition-colors group ${expandedModels[c.id] ? 'text-[var(--accent)] bg-[var(--accent)]/5' : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-white/5'}`}>
                    <Settings2 size={14} className={`opacity-70 group-hover:opacity-100 transition-colors ${expandedModels[c.id] ? 'text-[var(--accent)]' : 'group-hover:text-purple-400'}`} /> Models
                  </button>
                  <div className="relative" data-connection-menu-root>
                    <button onClick={() => setMenuOpenFor(menuOpenFor === c.id ? null : c.id)}
                      className="px-4 py-3 h-full flex items-center justify-center text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-white/5 transition-colors group">
                      <MoreHorizontal size={14} className="group-hover:text-white opacity-70 group-hover:opacity-100 transition-colors" />
                    </button>
                    {menuOpenFor === c.id && (
                      <div className="absolute bottom-full right-0 mb-1 w-36 bg-[var(--bg-card)] border border-[var(--border)] rounded-lg shadow-xl shadow-black/50 py-1 z-50 animate-in fade-in slide-in-from-bottom-2">
                        {(isRL || c.backoffLevel > 0) && (
                          <button onClick={() => { handleResetCooldown(c.id); setMenuOpenFor(null) }}
                            className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-medium text-amber-400 hover:bg-amber-500/10 transition-colors">
                            <RefreshCw size={12} /> Reset Cooldown
                          </button>
                        )}
                        <button onClick={() => openDeleteDialog(c.id, c.name)}
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-medium text-red-500 hover:bg-red-500/10 transition-colors">
                          <Trash2 size={12} /> Remove
                        </button>
                      </div>
                    )}
                  </div>
                </div>

              </div>
            )
          })}
        </div>
      )}

      {deleteTarget && (
        <div
          className="fixed inset-0 z-[200] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
          role="presentation"
          onMouseDown={e => { if (e.target === e.currentTarget) closeDeleteDialog() }}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="delete-connection-title"
            className="w-full max-w-md rounded-2xl border border-red-500/20 bg-[var(--bg-card)] p-6 shadow-2xl animate-in fade-in zoom-in-95 duration-200"
            onMouseDown={e => e.stopPropagation()}
          >
            <div className="flex items-center gap-3 mb-2 text-red-400">
               <div className="w-10 h-10 rounded-full bg-red-500/10 flex items-center justify-center shrink-0">
                  <AlertTriangle size={20} />
               </div>
               <h3 id="delete-connection-title" className="text-xl font-bold text-[var(--text)]">Delete Connection</h3>
            </div>
            
            <p className="mt-3 text-sm text-[var(--text-muted)] leading-relaxed">
              This action cannot be undone. All configured models for <strong className="text-[var(--text)]">{deleteTarget.name}</strong> will be removed. Please type the connection name to confirm:
            </p>
            
            <p className="mt-4 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2 font-mono text-sm font-semibold select-all text-center">
              {deleteTarget.name}
            </p>
            
            <div className="mt-4 relative">
              <input
                id="delete-confirm-input"
                type="text"
                value={deleteConfirmName}
                onChange={e => setDeleteConfirmName(e.target.value)}
                autoComplete="off"
                placeholder={deleteTarget.name}
                disabled={deleteBusy}
                className="w-full rounded-lg border border-[var(--border)] bg-[var(--bg)] px-3 py-2.5 text-sm outline-none focus:border-red-500/50 focus:ring-1 focus:ring-red-500/50 disabled:opacity-50 transition-all font-mono"
              />
            </div>
            
            <div className="mt-6 flex flex-wrap justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={closeDeleteDialog}
                disabled={deleteBusy}
                className="rounded-lg bg-[var(--bg)] border border-[var(--border)] px-4 py-2 text-sm font-medium text-[var(--text)] transition-colors hover:bg-[var(--bg-hover)] disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => void confirmDeleteConnection()}
                disabled={deleteBusy || deleteConfirmName !== deleteTarget.name}
                className="rounded-lg bg-red-500 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-red-600 disabled:opacity-40 disabled:cursor-not-allowed shadow-md shadow-red-500/20"
              >
                {deleteBusy ? <div className="flex items-center gap-2"><Loader2 size={16} className="animate-spin" /> Deleting…</div> : 'Delete Connection'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
"""

text = text[:text.find(start_marker)] + new_code

with open(filepath, 'w', encoding='utf-8') as f:
    f.write(text)
print("done")
