import { Shield, Check, X, Clock, Lock, AlertTriangle, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { ProviderLogo } from './ProviderLogo';
import { getProviderLabel, getProviderMeta as getRegistryProviderMeta } from '@/lib/provider-registry';
import {
  isRateLimited,
  rateLimitSecondsLeft,
  lockedModelCount,
  secsToHuman,
  tokenSecondsLeft,
} from '@/lib/connection-status';
import type { Connection } from '@/types/connections';

// ─── Types ────────────────────────────────────────────────────────────────────

export type ImportMode = 'import' | 'builder-id' | 'idc' | 'social' | 'apikey';

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

export { getProviderLabel };

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

export function getProviderInfo(provider: string) {
    const meta = getRegistryProviderMeta(provider);
    return {
        icon: <ProviderLogoIcon provider={provider} size={32} className="rounded-md" />,
        colorClass: meta.colorClass,
        dotClass: 'bg-primary',
        label: meta.label,
    };
}

// ─── Small UI Components ──────────────────────────────────────────────────────

export function TokenBar({ conn }: { conn: Connection }) {
    if (!conn.expiresAt && !conn.apiKey) return null;

    if (conn.apiKey) {
        return (
            <Badge
                variant="outline"
                className="gap-1 text-emerald-600 border-emerald-500/30 bg-emerald-500/10 text-[10px] py-0 h-5"
            >
                <Shield size={9} /> API Key
            </Badge>
        );
    }

    if (!conn.expiresAt) return null;

    const secsLeft = tokenSecondsLeft(conn) ?? 0;
    if (secsLeft <= 0) {
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
            <Check size={9} /> {secsToHuman(secsLeft)}
        </Badge>
    );
}

export function StatusRow({ conn }: { conn: Connection }) {
    const isRL = isRateLimited(conn);
    const rlSecs = isRL ? rateLimitSecondsLeft(conn) : 0;
    const lockCount = lockedModelCount(conn);

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
            {(conn.backoffLevel ?? 0) > 0 && (
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
