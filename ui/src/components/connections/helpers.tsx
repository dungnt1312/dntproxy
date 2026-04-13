import { Shield, Check, X, Clock, Lock, AlertTriangle, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { ProviderLogo } from './ProviderLogo';

// ─── Types ────────────────────────────────────────────────────────────────────

export const PROVIDERS = [
    { id: 'kiro', name: 'Kiro AI', icon: 'KI' },
    { id: 'openai', name: 'OpenAI', icon: 'OA' },
    { id: 'qwen', name: 'Qwen', icon: 'QW' },
    { id: 'glm', name: 'GLM (Zhipu AI)', icon: 'GL' },
    { id: 'minimax', name: 'MiniMax', icon: 'MM' },
    { id: 'openai-compatible', name: 'OpenAI Compatible', icon: 'OC' },
] as const;

export type ImportMode = 'detect' | 'file' | 'manual' | 'builder-id' | 'idc' | 'social';
export type SortMode = 'name' | 'issues' | 'provider';

export interface DeviceCodeState {
    sessionId: string;
    userCode: string;
    verificationUri: string;
    verificationUriComplete: string;
    expiresIn: number;
    interval: number;
}
export interface SocialLoginState {
    sessionId: string;
    loginUrl: string;
    provider: 'google' | 'github';
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

export function secsToHuman(secs: number): string {
    if (secs <= 0) return '0s';
    if (secs < 60) return `${secs}s`;
    if (secs < 3600) return `${Math.floor(secs / 60)}m`;
    return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`;
}

export function getProviderMeta(provider: string) {
    return PROVIDERS.find((x) => x.id === provider);
}

export const getProviderLabel = (p: string) => getProviderMeta(p)?.name ?? p;

export function connectionAttentionRank(c: {
    isActive?: boolean;
    rateLimitedUntil?: string;
    expiresAt?: string;
    backoffLevel?: number;
    lastError?: string;
}) {
    if (!c.isActive) return 2;
    const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date();
    const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date();
    const hasIssue = isRL || isExpired || (c.backoffLevel ?? 0) > 0 || !!c.lastError;
    return hasIssue ? 0 : 1;
}

// ─── Provider Logo Wrapper ────────────────────────────────────────────────────
// Wraps ProviderLogo for easy use with consistent styling

export const ProviderLogoIcon = ({
    provider,
    size = 20,
    className = '',
}: {
    provider: string;
    size?: number;
    className?: string;
}) => <ProviderLogo provider={provider} size={size} className={className} />;

// Legacy aliases for backward compatibility
export const AwsLogo = ProviderLogoIcon;
export const OpenAILogo = ProviderLogoIcon;
export const GLMLogo = ProviderLogoIcon;
export const MiniMaxLogo = ProviderLogoIcon;
export const QwenLogo = ProviderLogoIcon;
export const CustomLogo = ProviderLogoIcon;

export function getProviderInfo(provider: string) {
    if (provider === 'kiro')
        return {
            icon: <ProviderLogoIcon provider="kiro" size={32} className="rounded-md" />,
            colorClass: 'bg-[#FF9900]/10 border-[#FF9900]/20',
            dotClass: 'bg-[#FF9900]',
            label: 'AWS / Kiro',
        };
    if (provider === 'openai')
        return {
            icon: <ProviderLogoIcon provider="openai" size={32} className="rounded-md" />,
            colorClass: 'bg-[#10a37f]/10 border-[#10a37f]/20',
            dotClass: 'bg-[#10a37f]',
            label: 'OpenAI',
        };
    if (provider === 'glm')
        return {
            icon: <ProviderLogoIcon provider="glm" size={32} className="rounded-md" />,
            colorClass: 'bg-[#0066FF]/10 border-[#0066FF]/20',
            dotClass: 'bg-[#0066FF]',
            label: 'GLM (Zhipu AI)',
        };
    if (provider === 'minimax')
        return {
            icon: <ProviderLogoIcon provider="minimax" size={32} className="rounded-md" />,
            colorClass: 'bg-[#FF6B35]/10 border-[#FF6B35]/20',
            dotClass: 'bg-[#FF6B35]',
            label: 'MiniMax',
        };
    if (provider === 'qwen')
        return {
            icon: <ProviderLogoIcon provider="qwen" size={32} className="rounded-md" />,
            colorClass: 'bg-[#6366F1]/10 border-[#6366F1]/20',
            dotClass: 'bg-[#6366F1]',
            label: 'Qwen',
        };
    return {
        icon: <ProviderLogoIcon provider={provider} size={32} className="rounded-md" />,
        colorClass: 'bg-primary/10 border-primary/20',
        dotClass: 'bg-primary',
        label: provider || 'Custom API',
    };
}

// ─── Small UI Components ──────────────────────────────────────────────────────

export function TokenBar({ conn }: { conn: any }) {
    if (!conn.expiresAt && !conn.hasApiKey) return null;

    if (conn.hasApiKey) {
        return (
            <Badge
                variant="outline"
                className="gap-1 text-emerald-600 border-emerald-500/30 bg-emerald-500/10 text-[10px] py-0 h-5"
            >
                <Shield size={9} /> API Key
            </Badge>
        );
    }

    const expMs = new Date(conn.expiresAt).getTime();
    const secsLeft = Math.ceil((expMs - Date.now()) / 1000);
    const expired = secsLeft <= 0;

    if (expired) {
        return (
            <Badge
                variant="outline"
                className="gap-1 text-destructive border-destructive/30 bg-destructive/10 text-[10px] py-0 h-5"
            >
                <X size={9} /> Expired
            </Badge>
        );
    }

    return (
        <Badge
            variant="outline"
            className="gap-1 text-emerald-600 border-emerald-500/30 bg-emerald-500/10 text-[10px] py-0 h-5"
        >
            <Check size={9} /> {secsLeft > 3600 * 24 ? `${Math.floor(secsLeft / (3600 * 24))}d` : secsToHuman(secsLeft)}
        </Badge>
    );
}

export function StatusRow({ conn }: { conn: any }) {
    const isRL = conn.rateLimitedUntil && new Date(conn.rateLimitedUntil) > new Date();
    const rlSecs = isRL ? Math.ceil((new Date(conn.rateLimitedUntil).getTime() - Date.now()) / 1000) : 0;
    const lockCount = conn.modelLocks
        ? Object.values(conn.modelLocks).filter((e: any) => new Date(e) > new Date()).length
        : 0;

    return (
        <div className="flex items-center gap-1.5 flex-wrap mt-1">
            {isRL && (
                <Badge
                    variant="outline"
                    className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5"
                >
                    <Clock size={9} /> RL: {secsToHuman(rlSecs)}
                </Badge>
            )}
            {conn.backoffLevel > 0 && (
                <Badge
                    variant="outline"
                    className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5"
                >
                    <RefreshCw size={9} /> Backoff: {conn.backoffLevel}/7
                </Badge>
            )}
            {lockCount > 0 && (
                <Badge
                    variant="outline"
                    className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5"
                >
                    <Lock size={9} /> {lockCount} locked
                </Badge>
            )}
            {conn.lastError && (
                <Badge
                    variant="outline"
                    className="gap-1 text-destructive border-destructive/30 bg-destructive/10 text-[10px] py-0 h-5 max-w-[200px] truncate"
                    title={conn.lastError}
                >
                    <AlertTriangle size={9} /> {conn.lastError.slice(0, 40)}
                    {conn.lastError.length > 40 ? '…' : ''}
                </Badge>
            )}
        </div>
    );
}

// EOF

