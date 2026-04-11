import { Shield, Check, X, Clock, Lock, AlertTriangle, RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'

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

export const AwsLogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <img src="https://img.icons8.com/color/48/amazon-web-services.png" width={size} height={size} className={`${className} object-contain`} alt="AWS" />
)

export const OpenAILogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <img src="https://img.icons8.com/color/48/openai.png" width={size} height={size} className={`${className} object-contain`} alt="OpenAI" />
)

export const CustomLogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <img src="https://img.icons8.com/fluency/48/server.png" width={size} height={size} className={`${className} object-contain opacity-80`} alt="Custom" />
)

export function getProviderInfo(provider: string) {
  if (provider === 'kiro') return {
    icon: <AwsLogo size={20} />,
    colorClass: 'bg-orange-500/10 border-orange-500/20',
    dotClass: 'bg-orange-500',
    label: 'AWS / Kiro',
  }
  if (provider === 'openai') return {
    icon: <OpenAILogo size={20} />,
    colorClass: 'bg-emerald-500/10 border-emerald-500/20',
    dotClass: 'bg-emerald-500',
    label: 'OpenAI',
  }
  return {
    icon: <CustomLogo size={20} />,
    colorClass: 'bg-purple-500/10 border-purple-500/20',
    dotClass: 'bg-purple-500',
    label: 'Custom API',
  }
}

// ─── Small UI Components ──────────────────────────────────────────────────────

export function TokenBar({ conn }: { conn: any }) {
  if (!conn.expiresAt && !conn.hasApiKey) return null

  if (conn.hasApiKey) {
    return (
      <Badge variant="outline" className="gap-1 text-emerald-600 border-emerald-500/30 bg-emerald-500/10 text-[10px] py-0 h-5">
        <Shield size={9} /> API Key
      </Badge>
    )
  }

  const expMs = new Date(conn.expiresAt).getTime()
  const secsLeft = Math.ceil((expMs - Date.now()) / 1000)
  const expired = secsLeft <= 0

  if (expired) {
    return (
      <Badge variant="outline" className="gap-1 text-destructive border-destructive/30 bg-destructive/10 text-[10px] py-0 h-5">
        <X size={9} /> Expired
      </Badge>
    )
  }

  return (
    <Badge variant="outline" className="gap-1 text-emerald-600 border-emerald-500/30 bg-emerald-500/10 text-[10px] py-0 h-5">
      <Check size={9} /> {secsLeft > 3600 * 24 ? `${Math.floor(secsLeft / (3600 * 24))}d` : secsToHuman(secsLeft)}
    </Badge>
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
        <Badge variant="outline" className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5">
          <Clock size={9} /> RL: {secsToHuman(rlSecs)}
        </Badge>
      )}
      {conn.backoffLevel > 0 && (
        <Badge variant="outline" className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5">
          <RefreshCw size={9} /> Backoff: {conn.backoffLevel}/7
        </Badge>
      )}
      {lockCount > 0 && (
        <Badge variant="outline" className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5">
          <Lock size={9} /> {lockCount} locked
        </Badge>
      )}
      {conn.lastError && (
        <Badge variant="outline" className="gap-1 text-destructive border-destructive/30 bg-destructive/10 text-[10px] py-0 h-5 max-w-[200px] truncate" title={conn.lastError}>
          <AlertTriangle size={9} /> {conn.lastError.slice(0, 40)}{conn.lastError.length > 40 ? '…' : ''}
        </Badge>
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
      <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
        <RefreshCw size={12} className="animate-spin text-primary" />
        Checking quota…
      </div>
    )
  }
  if (!data) return null

  if (data.quotaSupported && (data.usageLimitReached !== undefined || data.quotaAvailable !== undefined)) {
    const exhausted = !!data.usageLimitReached
    const secsLeft = data.resetsInSeconds ?? 0
    const planType = data.planType ?? ''
    const planLabel = planType ? planType.charAt(0).toUpperCase() + planType.slice(1) + ' plan quota' : 'ChatGPT quota'
    const subLabel = exhausted && data.resetsAtHuman
      ? `Resets ${data.resetsAtHuman}${secsLeft > 0 ? ` (in ${secsToHuman(secsLeft)})` : ''}`
      : data.note || 'Quota available'

    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">{planLabel}</span>
          <Badge variant="outline" className={exhausted ? 'text-destructive border-destructive/30 bg-destructive/10 text-[10px]' : 'text-emerald-600 border-emerald-500/30 bg-emerald-500/10 text-[10px]'}>
            {exhausted ? 'Exhausted' : 'Available'}
          </Badge>
        </div>
        <Progress value={100} className={exhausted ? '[&>div]:bg-destructive' : '[&>div]:bg-emerald-500'} />
        <p className="text-[10px] text-muted-foreground">{subLabel}</p>
      </div>
    )
  }

  const buckets: QuotaBucket[] = []
  if (data.quotaSupported !== false) {
    if (data.requestsLimit != null && data.requestsLimit >= 0) {
      buckets.push({ label: 'requests', used: data.requestsLimit - (data.requestsRemaining ?? 0), limit: data.requestsLimit, pct: data.requestsPct ?? 0, checkedAt: data.resetRequests })
    }
    if (data.freeTrialLimit != null && data.freeTrialLimit >= 0) {
      buckets.push({ label: 'free trial', used: data.freeTrialLimit - (data.freeTrialRemaining ?? 0), limit: data.freeTrialLimit, pct: data.freeTrialPct ?? 0, expiresAt: data.freeTrialExpiresAt })
    }
    if (data.tokensLimit != null && data.tokensLimit >= 0) {
      buckets.push({ label: 'tokens', used: data.tokensLimit - (data.tokensRemaining ?? 0), limit: data.tokensLimit, pct: data.tokensPct ?? 0, checkedAt: data.resetTokens })
    }
  }

  const getProgressClass = (pct: number) =>
    pct >= 90 ? '[&>div]:bg-destructive' : pct >= 70 ? '[&>div]:bg-amber-500' : '[&>div]:bg-emerald-500'

  const fmt = (n: number) => n.toLocaleString()

  return (
    <div className="space-y-2.5">
      {buckets.length === 0 && (!data.overageCharges || data.overageCharges <= 0) && (
        <p className="text-xs text-muted-foreground italic">{data.note || 'No quota info available.'}</p>
      )}
      {data.overageCharges > 0 && (
        <Badge variant="outline" className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px]">
          <AlertTriangle size={9} /> Overage: ${data.overageCharges.toLocaleString()}
        </Badge>
      )}
      {buckets.map((b) => {
        const usedPct = Math.max(0, Math.min(b.pct, 100))
        return (
          <div key={b.label} className="space-y-1">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground capitalize">{b.label}</span>
              <span className="font-mono text-[11px] text-muted-foreground">{fmt(b.used)} / {fmt(b.limit)}</span>
            </div>
            <Progress value={usedPct} className={`h-1.5 ${getProgressClass(usedPct)}`} />
          </div>
        )
      })}
      {data.statusCode != null && data.statusCode !== 200 && (
        <p className="text-xs text-destructive">HTTP {data.statusCode} from upstream</p>
      )}
    </div>
  )
}
