import { Shield, Check, X, Clock, Lock, AlertTriangle, RefreshCw } from 'lucide-react'

// ─── Types ────────────────────────────────────────────────────────────────────

export const PROVIDERS = [
  { id: 'kiro', name: 'Kiro AI', icon: 'KI' },
  { id: 'openai', name: 'OpenAI', icon: 'OA' },
  { id: 'openai-compatible', name: 'OpenAI Compatible', icon: 'OC' },
] as const

export type ImportMode = 'detect' | 'file' | 'manual' | 'builder-id' | 'idc' | 'social'
export type SortMode = 'name' | 'issues' | 'provider'

export interface DeviceCodeState {
  sessionId: string; userCode: string; verificationUri: string;
  verificationUriComplete: string; expiresIn: number; interval: number
}
export interface SocialLoginState {
  sessionId: string; loginUrl: string; provider: 'google' | 'github'
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

export function secsToHuman(secs: number): string {
  if (secs <= 0) return '0s'
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.floor(secs / 60)}m`
  return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`
}

export function getProviderMeta(provider: string) {
  return PROVIDERS.find(x => x.id === provider)
}

export const getProviderLabel = (p: string) => getProviderMeta(p)?.name ?? p

export function connectionAttentionRank(c: { isActive?: boolean; rateLimitedUntil?: string; expiresAt?: string; backoffLevel?: number; lastError?: string }) {
  if (!c.isActive) return 2
  const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
  const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date()
  const hasIssue = isRL || isExpired || (c.backoffLevel ?? 0) > 0 || !!c.lastError
  return hasIssue ? 0 : 1
}

// ─── Provider Logos ───────────────────────────────────────────────────────────

export const AwsLogo = ({ size = 24, className = '' }: any) => (
  <img 
    src="https://img.icons8.com/color/48/amazon-web-services.png" 
    width={size} 
    height={size} 
    className={`${className} object-contain`} 
    alt="AWS"
  />
)

export const OpenAILogo = ({ size = 24, className = '' }: any) => (
  <img 
    src="https://img.icons8.com/color/48/openai.png" 
    width={size} 
    height={size} 
    className={`${className} object-contain`} 
    alt="OpenAI"
  />
)

export const CustomLogo = ({ size = 24, className = '' }: any) => (
  <img 
    src="https://img.icons8.com/fluency/48/server.png" 
    width={size} 
    height={size} 
    className={`${className} object-contain opacity-80`} 
    alt="Custom"
  />
)

export function getProviderInfo(provider: string) {
  if (provider === 'kiro') return { 
    icon: <AwsLogo size={20} />, 
    bg: 'rgba(255, 153, 0, 0.12)',
    border: 'rgba(255, 153, 0, 0.25)',
    color: '#FF9900',
  }
  if (provider === 'openai') return { 
    icon: <OpenAILogo size={20} />, 
    bg: 'rgba(16, 163, 127, 0.12)',
    border: 'rgba(16, 163, 127, 0.25)',
    color: '#10a37f',
  }
  return { 
    icon: <CustomLogo size={20} />, 
    bg: 'rgba(168, 85, 247, 0.12)',
    border: 'rgba(107, 33, 168, 0.25)',
    color: '#a855f7',
  }
}

// ─── Small UI Components ──────────────────────────────────────────────────────

export function TokenBar({ conn }: { conn: any }) {
  if (!conn.expiresAt && !conn.hasApiKey) return null

  if (conn.hasApiKey) {
    return (
      <span className="chip chip-success">
        <Shield size={10} /> API Key
      </span>
    )
  }

  const expMs = new Date(conn.expiresAt).getTime()
  const secsLeft = Math.ceil((expMs - Date.now()) / 1000)
  const expired = secsLeft <= 0

  if (expired) {
    return (
      <span className="chip chip-danger">
        <X size={10} /> Token expired
      </span>
    )
  }

  return (
    <span className="chip chip-success">
      <Check size={10} /> Token valid ({secsLeft > 3600*24 ? `${Math.floor(secsLeft/(3600*24))}d left` : secsToHuman(secsLeft)})
    </span>
  )
}

export function StatusRow({ conn }: { conn: any }) {
  const isRL = conn.rateLimitedUntil && new Date(conn.rateLimitedUntil) > new Date()
  const rlSecs = isRL ? Math.ceil((new Date(conn.rateLimitedUntil).getTime() - Date.now()) / 1000) : 0
  const lockCount = conn.modelLocks
    ? Object.values(conn.modelLocks).filter((e: any) => new Date(e) > new Date()).length
    : 0

  return (
    <div className="flex items-center gap-1.5 flex-wrap mt-1">
      {isRL && (
        <span className="chip chip-warning">
          <Clock size={10} /> RL: {secsToHuman(rlSecs)}
        </span>
      )}
      {conn.backoffLevel > 0 && (
        <span className="chip chip-warning">
          <RefreshCw size={10} /> Backoff: {conn.backoffLevel}/7
        </span>
      )}
      {lockCount > 0 && (
        <span className="chip chip-warning">
          <Lock size={10} /> {lockCount} models
        </span>
      )}
      {conn.lastError && (
        <span className="chip chip-danger truncate max-w-[180px]" title={conn.lastError}>
          <AlertTriangle size={10} /> {conn.lastError.slice(0, 40)}{conn.lastError.length > 40 ? '…' : ''}
        </span>
      )}
    </div>
  )
}

interface QuotaBucket {
  label: string
  used: number
  limit: number
  pct: number
  checkedAt?: string
  expiresAt?: string
}

export function QuotaPanel({ data, loading }: { data: any; loading: boolean }) {
  if (loading) {
    return (
      <div className="flex items-center gap-2 text-xs text-[var(--text-muted)] py-2">
        <RefreshCw size={12} className="animate-spin text-[var(--accent)]" />
        Checking quota…
      </div>
    )
  }
  if (!data) return null

  // ── Codex / ChatGPT OAuth ─ map to progress bar format ──────────────────
  if (data.quotaSupported && (data.usageLimitReached !== undefined || data.quotaAvailable !== undefined)) {
    const exhausted = !!data.usageLimitReached
    const secsLeft = data.resetsInSeconds ?? 0
    const planType = data.planType ?? ''
    const planLabel = planType
      ? planType.charAt(0).toUpperCase() + planType.slice(1) + ' plan quota'
      : 'ChatGPT quota'

    const barColor = exhausted ? '#f87171' : '#4ade80'
    const barWidth = 100
    const statusText = exhausted ? 'Exhausted' : 'Available'
    const statusColor = exhausted ? '#f87171' : '#4ade80'

    const subLabel = exhausted && data.resetsAtHuman
      ? `Resets ${data.resetsAtHuman}${secsLeft > 0 ? ` (in ${secsToHuman(secsLeft)})` : ''}`
      : data.note || 'Quota is available for this session'

    return (
      <div className="space-y-2.5 pt-0.5">
        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-xs">
            <span className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ backgroundColor: barColor }} />
              <span className="text-[var(--text-muted)]">{planLabel}</span>
            </span>
            <span className="font-medium text-[10px] px-2 py-0.5 rounded-full" style={{ color: statusColor, backgroundColor: `${statusColor}18` }}>
              {statusText}
            </span>
          </div>
          <div className="progress-bar">
            <div
              className="progress-fill"
              style={{ width: `${barWidth}%`, backgroundColor: barColor }}
            />
          </div>
          <p className="text-[10px] text-[var(--text-dim)] mt-0.5">{subLabel}</p>
        </div>
      </div>
    )
  }

  // ── Standard quota buckets (Kiro, OpenAI API key) ───────────────────────
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
    <div className="space-y-2.5 pt-0.5">
      {buckets.length === 0 && (!data.overageCharges || data.overageCharges <= 0) && (
        <p className="text-xs text-[var(--text-dim)] italic">
          {data.note || 'No quota information available for this connection.'}
        </p>
      )}
      {data.overageCharges > 0 && (
        <div className="chip chip-warning">
          <AlertTriangle size={10} /> Overage: ${data.overageCharges.toLocaleString()}
        </div>
      )}
      {buckets.map((b) => {
        const usedPct = Math.max(0, Math.min(b.pct, 100))
        const remainingPct = Math.max(0, 100 - usedPct)
        const c = color(usedPct)
        return (
          <div key={b.label} className="space-y-1.5">
            <div className="flex items-center justify-between text-xs">
              <span className="flex items-center gap-1.5">
                <span
                  className="w-2 h-2 rounded-full flex-shrink-0"
                  style={{ backgroundColor: c }}
                />
                <span className="text-[var(--text-muted)]">{b.label}</span>
              </span>
              <div className="flex items-center gap-2 text-[var(--text-muted)]">
                <span className="font-mono text-[11px]">
                  {fmt(b.used)}&nbsp;/&nbsp;{fmt(b.limit)}
                </span>
              </div>
            </div>
            <div className="progress-bar">
              <div
                className="progress-fill"
                style={{ width: `${remainingPct}%`, backgroundColor: c }}
              />
            </div>
          </div>
        )
      })}
      {data.statusCode != null && data.statusCode !== 200 && (
        <p className="text-xs text-[var(--danger)]">HTTP {data.statusCode} from upstream</p>
      )}
    </div>
  )
}
