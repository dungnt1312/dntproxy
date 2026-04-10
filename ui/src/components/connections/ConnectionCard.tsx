import { useState, useEffect } from 'react'
import {
  TestTube, Settings2, MoreHorizontal, RefreshCw,
  Trash2, BarChart2, Loader2, Play, ChevronRight, AlertTriangle, Clock, Lock
} from 'lucide-react'
import { api } from '../../api'
import { getModelName } from '../../models-config'
import InlineName from './InlineName'
import { TokenBar, QuotaPanel, getProviderInfo, secsToHuman } from './helpers'

interface ConnectionRowProps {
  conn: any
  initialQuotaResult?: any
  onReload: () => void
  onDelete: (id: string, name: string) => void
  onEditModels: (conn: any) => void
}

export default function ConnectionCard({
  conn: c,
  initialQuotaResult,
  onReload,
  onDelete,
  onEditModels,
}: ConnectionRowProps) {
  const [menuOpen, setMenuOpen] = useState(false)
  const [testResult, setTestResult] = useState<any>(null)
  const [quotaResult, setQuotaResult] = useState<any>(initialQuotaResult ?? null)
  const [quotaLoading, setQuotaLoading] = useState(false)
  const [modelTestResults, setModelTestResults] = useState<Record<string, { status: 'ok' | 'error' | 'loading'; message?: string }>>({})

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuOpen && !(e.target as Element).closest('[data-connection-menu-root]')) {
        setMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [menuOpen])

  useEffect(() => {
    if (initialQuotaResult !== undefined) {
      setQuotaResult(initialQuotaResult)
    }
  }, [initialQuotaResult])

  const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
  const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date()
  const hasIssue = isRL || isExpired || c.backoffLevel > 0 || c.lastError
  const statusColor = !c.isActive ? '#64748b'
    : isRL ? '#fbbf24'
    : hasIssue ? '#ef4444'
    : '#22c55e'

  const providerInfo = getProviderInfo(c.provider)
  const lockCount = c.modelLocks
    ? Object.values(c.modelLocks).filter((e: any) => new Date(e) > new Date()).length
    : 0

  const handleTest = async () => {
    setTestResult({ loading: true })
    try {
      const res = await api.testConnection(c.id)
      setTestResult(res)
    } catch (e: any) {
      setTestResult({ status: 'error', message: e.message })
    }
  }

  const handleTestModel = async (model: string) => {
    setModelTestResults(prev => ({ ...prev, [model]: { status: 'loading' } }))
    try {
      const res = await api.testModel(c.id, model)
      setModelTestResults(prev => ({ ...prev, [model]: { status: res.status === 'ok' ? 'ok' : 'error', message: res.message } }))
    } catch (e: any) {
      setModelTestResults(prev => ({ ...prev, [model]: { status: 'error', message: e.message } }))
    }
  }

  const handleCheckQuota = async () => {
    setQuotaLoading(true)
    try {
      const res = await api.checkQuota(c.id)
      setQuotaResult(res)
    } catch (e: any) {
      setQuotaResult({ error: e.message })
    } finally {
      setQuotaLoading(false)
    }
  }

  const handleToggle = async () => {
    await api.updateConnection(c.id, { isActive: !c.isActive })
    onReload()
  }

  const handleRename = async (id: string, name: string) => {
    await api.updateConnection(id, { name })
    onReload()
  }

  const handleResetCooldown = async () => {
    try { await api.resetCooldown(c.id); onReload() } catch (e: any) { console.error(e.message) }
  }

  const rlSecs = isRL ? Math.ceil((new Date(c.rateLimitedUntil).getTime() - Date.now()) / 1000) : 0

  let quotaText = ''
  if (quotaResult && !quotaResult.error && quotaResult.totalTokens > 0) {
    const used = quotaResult.totalTokens - quotaResult.remainingTokens
    quotaText = `${(used / 1000).toFixed(1)}k / ${(quotaResult.totalTokens / 1000).toFixed(1)}k`
  }

  return (
    <div
      className={`glass-sm transition-all cursor-default group ${
        !c.isActive ? 'opacity-60' :
        isRL ? 'border-amber-500/20' :
        hasIssue ? 'border-red-500/20' :
        'border-[var(--border)] hover:border-[var(--border-hover)]'
      }`}
      style={{ padding: '16px', display: 'flex', flexDirection: 'column', gap: '14px' }}
    >
      {/* ── Header: Identity + Status + Actions ── */}
      <div style={{ display: 'flex', gap: '14px', alignItems: 'flex-start' }}>
        {/* Provider Icon */}
        <div className="relative" style={{ flexShrink: 0 }}>
          <div
            className="flex items-center justify-center shadow-sm"
            style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              backgroundColor: providerInfo.bg,
              border: `1px solid ${providerInfo.border}`,
            }}
          >
            {providerInfo.icon}
          </div>
          <span
            className="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full border-2"
            style={{
              backgroundColor: statusColor,
              borderColor: 'var(--bg-card-solid)',
            }}
          />
        </div>

        {/* Identity + Status */}
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: '6px' }}>
          {/* Name row */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <InlineName conn={c} onRename={handleRename} />
            <span style={{ fontSize: '11px', color: 'var(--text-dim)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {c.email || c.baseUrl?.replace('https://', '') || c.authMethod || 'API Key'}
            </span>
          </div>

          {/* Status row */}
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', alignItems: 'center' }}>
            <TokenBar conn={c} />

            {isRL && (
              <span className="chip chip-warning" style={{ padding: '2px 8px', fontSize: '10px' }}>
                <Clock size={10} /> RL: {secsToHuman(rlSecs)}
              </span>
            )}
            {c.backoffLevel > 0 && (
              <span className="chip chip-warning" style={{ padding: '2px 8px', fontSize: '10px' }}>
                <RefreshCw size={10} /> Backoff: {c.backoffLevel}/7
              </span>
            )}
            {lockCount > 0 && (
              <span className="chip chip-warning" style={{ padding: '2px 8px', fontSize: '10px' }}>
                <Lock size={10} /> {lockCount} locked
              </span>
            )}
            {c.lastError && (
              <span className="chip chip-danger truncate" style={{ maxWidth: '180px', fontSize: '10px' }} title={c.lastError}>
                <AlertTriangle size={10} /> {c.lastError.slice(0, 30)}{c.lastError.length > 30 ? '…' : ''}
              </span>
            )}

            {testResult && (
              <span style={{
                fontSize: '10px',
                fontWeight: 500,
                marginLeft: '4px',
                color: testResult.status === 'ok' ? 'var(--success)' : testResult.loading ? 'var(--text-dim)' : 'var(--danger)',
              }}>
                {testResult.loading ? 'Testing…' : testResult.status === 'ok' ? '✓ OK' : '✗ Failed'}
              </span>
            )}

            {quotaText && (
              <button
                onClick={handleCheckQuota}
                className="flex items-center gap-1"
                style={{
                  fontSize: '11px',
                  fontFamily: 'monospace',
                  color: 'var(--text-muted)',
                  background: 'rgba(255,255,255,0.04)',
                  padding: '2px 8px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  cursor: 'pointer',
                  transition: 'all 150ms ease',
                }}
              >
                {quotaText}
                <RefreshCw size={10} className={quotaLoading ? 'animation: spin 1s linear infinite' : ''} style={{ opacity: quotaLoading ? 1 : 0.5 }} />
              </button>
            )}
          </div>
        </div>

        {/* Actions */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
          {/* Toggle */}
          <button
            onClick={handleToggle}
            style={{
              position: 'relative',
              width: '36px',
              height: '20px',
              borderRadius: '10px',
              backgroundColor: c.isActive ? 'var(--accent)' : 'var(--border)',
              border: 'none',
              cursor: 'pointer',
              transition: 'background-color 200ms ease',
              flexShrink: 0,
            }}
          >
            <span style={{
              position: 'absolute',
              top: '3px',
              left: c.isActive ? '18px' : '3px',
              width: '14px',
              height: '14px',
              borderRadius: '50%',
              backgroundColor: 'white',
              boxShadow: '0 1px 3px rgba(0,0,0,0.3)',
              transition: 'left 200ms ease',
            }} />
          </button>

          <button
            onClick={handleTest}
            className="btn-ghost"
            style={{ padding: '6px 10px', fontSize: '12px', gap: '4px' }}
          >
            <TestTube size={13} />
          </button>
          <button
            onClick={() => onEditModels(c)}
            className="btn-ghost"
            style={{ padding: '6px 10px', fontSize: '12px', gap: '4px' }}
          >
            <Settings2 size={13} />
          </button>

          {/* More menu */}
          <div className="relative" data-connection-menu-root>
            <button
              onClick={() => setMenuOpen(!menuOpen)}
              className="btn-icon"
              style={{ padding: '6px' }}
            >
              <MoreHorizontal size={16} />
            </button>

            {menuOpen && (
              <div
                className="glass-sm shadow-xl"
                style={{
                  position: 'absolute',
                  top: '100%',
                  right: 0,
                  marginTop: '4px',
                  width: '160px',
                  padding: '6px',
                  zIndex: 50,
                  animation: 'fadeIn 150ms ease',
                }}
              >
                {(isRL || c.backoffLevel > 0) && (
                  <button
                    onClick={() => { handleResetCooldown(); setMenuOpen(false) }}
                    className="flex items-center gap-2"
                    style={{
                      width: '100%',
                      padding: '8px 10px',
                      fontSize: '12px',
                      fontWeight: 500,
                      color: 'var(--warning)',
                      background: 'transparent',
                      border: 'none',
                      borderRadius: '8px',
                      cursor: 'pointer',
                      textAlign: 'left',
                    }}
                  >
                    <RefreshCw size={13} /> Reset Cooldown
                  </button>
                )}
                <button
                  onClick={() => { handleCheckQuota(); setMenuOpen(false) }}
                  className="flex items-center gap-2"
                  style={{
                    width: '100%',
                    padding: '8px 10px',
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--text)',
                    background: 'transparent',
                    border: 'none',
                    borderRadius: '8px',
                    cursor: 'pointer',
                    textAlign: 'left',
                  }}
                >
                  <BarChart2 size={13} /> Check Quota
                </button>
                <button
                  onClick={() => { onDelete(c.id, c.name); setMenuOpen(false) }}
                  className="flex items-center gap-2"
                  style={{
                    width: '100%',
                    padding: '8px 10px',
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--danger)',
                    background: 'transparent',
                    border: 'none',
                    borderRadius: '8px',
                    cursor: 'pointer',
                    textAlign: 'left',
                    borderTop: '1px solid var(--border)',
                    marginTop: '4px',
                    paddingTop: '12px',
                  }}
                >
                  <Trash2 size={13} /> Remove
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* ── Divider ── */}
      <div style={{ height: '1px', backgroundColor: 'var(--border)' }} />

      {/* ── Models + Quota ── */}
      <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-start' }}>
        {/* Models */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{
            fontSize: '10px',
            fontWeight: 700,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            color: 'var(--text-dim)',
            marginBottom: '8px',
            fontFamily: 'var(--font-heading)',
          }}>
            Supported Models
          </div>
          {c.supportedModels?.length ? (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
              {c.supportedModels.slice(0, 6).map((m: string, i: number) => {
                const mTest = modelTestResults[m]
                return (
                  <div key={i} className="group/model relative">
                    <div
                      style={{
                        padding: '4px 10px',
                        borderRadius: '8px',
                        fontSize: '11px',
                        fontFamily: 'monospace',
                        border: '1px solid',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '4px',
                        cursor: 'default',
                        transition: 'all 150ms ease',
                        ...(mTest?.status === 'ok'
                          ? { backgroundColor: 'var(--success-glow)', color: 'var(--success)', borderColor: 'rgba(34,197,94,0.25)' }
                          : mTest?.status === 'error'
                          ? { backgroundColor: 'var(--danger-glow)', color: 'var(--danger)', borderColor: 'rgba(239,68,68,0.25)' }
                          : { backgroundColor: 'rgba(255,255,255,0.03)', color: 'var(--text-muted)', borderColor: 'var(--border)' }),
                      }}
                    >
                      {mTest?.status === 'loading' && <Loader2 size={10} className="animate-spin" />}
                      {getModelName(m)}
                    </div>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleTestModel(m) }}
                      className="absolute"
                      style={{
                        top: '50%',
                        left: '50%',
                        transform: 'translate(-50%, -50%)',
                        opacity: 0,
                        padding: '4px',
                        borderRadius: '6px',
                        backgroundColor: 'var(--accent)',
                        color: 'white',
                        border: 'none',
                        cursor: 'pointer',
                        zIndex: 10,
                      }}
                    >
                      <Play size={10} fill="currentColor" />
                    </button>
                  </div>
                )
              })}
              {c.supportedModels.length > 6 && (
                <button
                  onClick={() => onEditModels(c)}
                  style={{
                    padding: '4px 10px',
                    borderRadius: '8px',
                    fontSize: '11px',
                    fontWeight: 500,
                    color: 'var(--text-dim)',
                    backgroundColor: 'rgba(255,255,255,0.03)',
                    border: '1px solid var(--border)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                  }}
                >
                  +{c.supportedModels.length - 6} <ChevronRight size={10} />
                </button>
              )}
            </div>
          ) : (
            <div style={{ fontSize: '11px', color: 'var(--text-dim)' }}>No models assigned</div>
          )}
        </div>

        {/* Quota Panel */}
        {(quotaResult || quotaLoading) && (
          <div
            style={{
              flexShrink: 0,
              width: '200px',
              backgroundColor: 'rgba(255,255,255,0.02)',
              border: '1px solid var(--border)',
              borderRadius: '10px',
              padding: '12px',
            }}
          >
            <QuotaPanel data={quotaResult} loading={quotaLoading} />
          </div>
        )}
      </div>
    </div>
  )
}
