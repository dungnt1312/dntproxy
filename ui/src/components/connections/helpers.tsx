import { Shield, Check, X, Clock, Lock, AlertTriangle, RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'

// ─── Types ────────────────────────────────────────────────────────────────────

export const PROVIDERS = [
  { id: 'kiro', name: 'Kiro AI', icon: 'KI' },
  { id: 'openai', name: 'OpenAI', icon: 'OA' },
  { id: 'qwen', name: 'Qwen', icon: 'QW' },
  { id: 'glm', name: 'GLM (Zhipu AI)', icon: 'GL' },
  { id: 'minimax', name: 'MiniMax', icon: 'MM' },
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
  <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
    <path d="M14.075 12.046c-.235.394-.492.748-.795 1.042-.303.294-.746.541-1.32.748l-.481-1.442c.328-.152.607-.336.853-.559.245-.224.453-.483.62-.773h-1.921c-.482 0-.853-.131-1.11-.383-.245-.252-.361-.595-.361-1.002 0-.585.195-1.026.59-1.325.394-.309.917-.461 1.579-.461h.971v-1.155h-.475c-.328 0-.616.059-.854.184s-.418.303-.526.541h-2.138c.118-.538.4-1.002.836-1.393.438-.381.996-.581 1.66-.581h2.518c.884 0 1.52.196 1.905.57.385.385.579 1.024.579 1.916v3.294c0 .36.035.632.095.819h-1.226zm-2.029-.2c.28-.01.53-.08.73-.207.21-.132.378-.328.514-.602v-.86h-.896c-.309 0-.518.068-.616.211-.101.127-.156.289-.156.495 0 .341.139.601.411.758-.02.049.006.211.006.211zm8.397-2.186c-.035-.303-.139-.589-.328-.854-.188-.266-.453-.496-.789-.661h-2.128c.438-.501 1.02-.758 1.748-.758.461 0 .86.096 1.206.294.341.196.559.458.647.795h1.968c-.14-.73-.501-1.314-1.102-1.742-.589-.438-1.314-.647-2.174-.647-.942 0-1.749.278-2.395.826-.647.558-.971 1.365-.971 2.394 0 1.042.336 1.838 1.011 2.393.682.559 1.527.83 2.535.83.67 0 1.272-.116 1.803-.35.541-.244.972-.57 1.303-.984h-1.921c-.2.257-.453.454-.758.595-.296.143-.632.222-1.002.222-.857 0-1.464-.23-1.803-.681-.244-.316-.39-.7-.411-1.126h3.693c0 .034.02.13.02.261l-.143-.78zm-3.842-.71h1.564c-.11-.341-.318-.601-.58-.781-.261-.168-.58-.266-.993-.266-.445 0-.79.088-1.05.266-.251.18-.466.44-.526.78h1.583v.001zm-3.52-5.748c-4.484 0-8.623 1.583-11.96 4.293l1.134.982c3.08-2.485 6.844-3.921 10.9-3.921 4.542 0 8.718 1.768 11.929 4.82h-.037l1.144-.972c-3.411-3.238-7.85-5.201-12.723-5.201h-.385v-.001zm-9.043 8.361l1.792-6.19h2.24l1.325 4.839 1.336-4.839h2.203l1.737 6.19h-1.927l-.678-2.906c-.053-.257-.101-.56-.16-.893h-.037c-.074.257-.123.543-.163.854l-.746 2.946h-1.892l-.789-3.085c-.052-.257-.105-.53-.152-.816h-.062c-.017.118-.088.423-.174.88l-.702 3.02h-2.181l-.014.001z" />
  </svg>
)

export const OpenAILogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
    <path d="M22.28 9.82a6 6 0 0 0-1.56-3.27 6 6 0 0 0-3.33-1.54 6 6 0 0 0-5.39 1.55 6 6 0 0 0-3.62-1.16 6 6 0 0 0-2.32 3.01 6 6 0 0 0 1.56 3.27 6 6 0 0 0 3.33 1.54 6 6 0 0 0 5.39-1.55 6 6 0 0 0 3.62 1.16 6 6 0 0 0 2.32-3.01ZM8.57 16.49a4.4 4.4 0 0 1-2.47-1.13 4.4 4.4 0 0 1-1.16-2.43 4.36 4.36 0 0 1 .43-2.61c.2-.42.47-.81.82-1.12.34-.31.74-.55 1.17-.7.43-.15.89-.2 1.34-.14a4.4 4.4 0 0 1 2.33 1.08 4.42 4.42 0 0 1 1.26 2.39 4.38 4.38 0 0 1-.3 2.63c-.2-.42-.48.8-.82 1.11-.34.31-.74.55-1.17.7a4.4 4.4 0 0 1-1.34.14h-.06Zm8.86-6.67a4.8 4.8 0 0 1-.5 1.27c-.22.39-.51.72-.86.97-.35.26-.75.43-1.17.5-.42.08-.85.05-1.27-.06-.42-.12-.81-.33-1.15-.62-.34-.3-.61-.66-.8-1.08a4.4 4.4 0 0 1-.22-1.29 4.4 4.4 0 0 1 .41-1.24 4.4 4.4 0 0 1 .95-1.1c.36-.29.78-.48 1.23-.56a4.8 4.8 0 0 1 1.27.06c.42.12.81.33 1.15.62.34.3.61.66.8 1.08.19.42.32.87.38 1.33.06.46.05.93-.02 1.39-.08.45-.22.9-.43 1.31ZM12 4.48a4.4 4.4 0 0 1 1.34-.14c.49.03.96.19 1.38.46.42.27.78.64 1.04 1.08.26.44.42.94.47 1.46.05.52-.01 1.04-.17 1.53a4.4 4.4 0 0 1-1.08 1.53 4.42 4.42 0 0 1-1.46 1.08 4.38 4.38 0 0 1-1.53.47 4.4 4.4 0 0 1-1.53-.47 4.4 4.4 0 0 1-1.08-1.53 4.42 4.42 0 0 1-.47-1.46c-.05-.52.01-1.04.17-1.53a4.4 4.4 0 0 1 1.08-1.53c.26-.44.62-.81 1.04-1.08.42-.26.89-.42 1.38-.45v.05Zm-3.43 5.35c.22-.39.51-.72.86-.97.35-.26.75-.43 1.17-.5.42-.08.85-.05 1.27.06.42.12.81.33 1.15.62.34.3.61.66.8 1.08a4.4 4.4 0 0 1 .22 1.29 4.4 4.4 0 0 1-.41 1.24 4.4 4.4 0 0 1-.95 1.1c-.36.29-.78.48-1.23.56a4.8 4.8 0 0 1-1.27-.06c-.42-.12-.81-.33-1.15-.62-.34-.3-.61-.66-.8-1.08-.19-.42-.32-.87-.38-1.33-.06-.46-.05-.93.02-1.39.08-.46.22-.9.43-1.31l.26.51Zm3.43 9.69a4.4 4.4 0 0 1-1.34.14c-.49-.03-.96-.19-1.38-.46-.42-.27-.78-.64-1.04-1.08-.26-.44-.42-.94-.47-1.46-.05-.52.01-1.04.17-1.53a4.4 4.4 0 0 1 1.08-1.53 4.42 4.42 0 0 1 1.46-1.08 4.38 4.38 0 0 1 1.53-.47 4.4 4.4 0 0 1 1.53.47c.44.26.81.62 1.08 1.04.27.42.43.89.47 1.38.04.49-.02 1.02-.17 1.51a4.42 4.42 0 0 1-1.08 1.53c-.42.26-.9.42-1.39.47-.49.05-1.02-.01-1.5-.17l.07-.07ZM22.28 14.18a4.4 4.4 0 0 1 2.47 1.13 4.4 4.4 0 0 1 1.16 2.43 4.36 4.36 0 0 1-.43 2.61c-.2.42-.47.81-.82 1.12-.34.31-.74.55-1.17.7-.43.15-.89.2-1.34.14a4.4 4.4 0 0 1-2.33-1.08 4.42 4.42 0 0 1-1.26-2.39 4.38 4.38 0 0 1 .3-2.63c.2-.42.48-.8.82-1.11.34-.31.74-.55 1.17-.7.43-.15.89-.2 1.34-.14h-.06v.02Z"/>
  </svg>
)

export const GLMLogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 15.5v-2.14c-1.72-.45-3-2-3-3.86 0-2.21 1.79-4 4-4s4 1.79 4 4c0 1.86-1.28 3.41-3 3.86v2.14h-2zm1-4.5c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2z"/>
    <text x="12" y="16" textAnchor="middle" fontSize="8" fontWeight="bold" fill="currentColor">G</text>
  </svg>
)

export const MiniMaxLogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
    <path d="M3 3h18v18H3V3zm2 2v14h14V5H5zm2.5 3h2l2.5 4 2.5-4h2v8h-2v-5l-2.5 3.5L9.5 11v5h-2V8z"/>
  </svg>
)

export const QwenLogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-1-13h2v6h-2zm0 8h2v2h-2z"/>
    <circle cx="12" cy="12" r="3" opacity="0.3"/>
    <path d="M12 6l2.5 4.5L12 15l-2.5-4.5z"/>
  </svg>
)

export const CustomLogo = ({ size = 24, className = '' }: { size?: number; className?: string }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={className}>
    <rect width="20" height="8" x="2" y="2" rx="2" ry="2"/>
    <rect width="20" height="8" x="2" y="14" rx="2" ry="2"/>
    <line x1="6" x2="6.01" y1="6" y2="6"/>
    <line x1="6" x2="6.01" y1="18" y2="18"/>
  </svg>
)

export function getProviderInfo(provider: string) {
  if (provider === 'kiro') return {
    icon: <AwsLogo size={20} className="text-[#FF9900]" />,
    colorClass: 'bg-[#FF9900]/10 border-[#FF9900]/20',
    dotClass: 'bg-[#FF9900]',
    label: 'AWS / Kiro',
  }
  if (provider === 'openai') return {
    icon: <OpenAILogo size={20} className="text-[#10a37f]" />,
    colorClass: 'bg-[#10a37f]/10 border-[#10a37f]/20',
    dotClass: 'bg-[#10a37f]',
    label: 'OpenAI',
  }
  if (provider === 'glm') return {
    icon: <GLMLogo size={20} className="text-[#0066FF]" />,
    colorClass: 'bg-[#0066FF]/10 border-[#0066FF]/20',
    dotClass: 'bg-[#0066FF]',
    label: 'GLM (Zhipu AI)',
  }
  if (provider === 'minimax') return {
    icon: <MiniMaxLogo size={20} className="text-[#FF6B35]" />,
    colorClass: 'bg-[#FF6B35]/10 border-[#FF6B35]/20',
    dotClass: 'bg-[#FF6B35]',
    label: 'MiniMax',
  }
  if (provider === 'qwen') return {
    icon: <QwenLogo size={20} className="text-[#6366F1]" />,
    colorClass: 'bg-[#6366F1]/10 border-[#6366F1]/20',
    dotClass: 'bg-[#6366F1]',
    label: 'Qwen',
  }
  return {
    icon: <CustomLogo size={20} className="text-purple-500" />,
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

// ─── Quota bucket type (mirrors backend UsageResponse) ────────────────────

interface QuotaBucket {
  key: string
  label: string
  used: number
  total: number
  remaining: number
  pct: number       // 0-100, percent USED (from backend)
  resetAt?: string
  unlimited: boolean
}

interface UsageData {
  provider?: string
  plan?: string
  limitReached?: boolean
  message?: string
  quotas?: QuotaBucket[]
  error?: string
}

// ─── Time helpers ──────────────────────────────────────────────────────────

function timeUntil(dateStr: string): string {
  const diff = new Date(dateStr).getTime() - Date.now()
  if (diff <= 0) return 'now'
  const mins = Math.floor(diff / 60000)
  const hours = Math.floor(mins / 60)
  const days = Math.floor(hours / 24)
  if (days > 0) return `in ${days}d ${hours % 24}h`
  if (hours > 0) return `in ${hours}h ${mins % 60}m`
  return `in ${mins}m`
}

// ─── QuotaPanel ────────────────────────────────────────────────────────────

export function QuotaPanel({ data, loading }: { data: UsageData | null; loading: boolean }) {
  if (loading) {
    return (
      <div className="flex items-center gap-2 text-xs text-muted-foreground py-1">
        <RefreshCw size={12} className="animate-spin text-primary" />
        Checking quota…
      </div>
    )
  }
  if (!data) return null

  // Error from API call itself
  if (data.error) {
    return <p className="text-[11px] text-destructive">{data.error}</p>
  }

  // Message with no quota data (e.g. API key connections, auth expired)
  const quotas = data.quotas ?? []
  if (data.message && quotas.length === 0) {
    return <p className="text-[11px] text-muted-foreground italic">{data.message}</p>
  }

  if (quotas.length === 0) {
    return <p className="text-[11px] text-muted-foreground italic">No quota info available.</p>
  }

  const barColor = (pct: number) =>
    pct >= 90 ? 'bg-red-500' : pct >= 70 ? 'bg-amber-500' : 'bg-emerald-500'

  return (
    <div className="space-y-2.5">
      {/* Plan header */}
      {(data.plan || data.limitReached) && (
        <div className="flex items-center justify-between">
          {data.plan && (
            <span className="text-[10px] text-muted-foreground font-medium uppercase tracking-wide">
              {data.plan}
            </span>
          )}
          {data.limitReached && (
            <Badge variant="outline" className="text-destructive border-destructive/30 bg-destructive/10 text-[10px] py-0">
              Limit Reached
            </Badge>
          )}
        </div>
      )}

      {/* Quota rows — bar width = REMAINING (full = lots of quota left) */}
      {quotas.map((b) => (
        <div key={b.key} className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground capitalize">{b.label}</span>
            <div className="flex items-center gap-1.5">
              <span className="font-mono text-[10px] text-muted-foreground">
                {b.used} / {b.total}
              </span>
              {b.resetAt && (
                <span className="text-[10px] text-muted-foreground/50">
                  · {timeUntil(b.resetAt)}
                </span>
              )}
            </div>
          </div>
          <div className="relative h-1.5 rounded-full bg-muted overflow-hidden">
            <div
              className={`absolute inset-y-0 left-0 rounded-full transition-all ${barColor(b.pct)}`}
              style={{ width: `${Math.max(0, 100 - b.pct)}%` }}
            />
          </div>
        </div>
      ))}
    </div>
  )
}

