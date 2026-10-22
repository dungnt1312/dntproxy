import { ProviderLogo } from './ProviderLogo';

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

// ─── Provider Logo Wrapper ────────────────────────────────────────────────────

export const ProviderLogoIcon = ({
    provider,
    size = 20,
    className = '',
}: {
    provider: string;
    size?: number;
    className?: string;
}) => <ProviderLogo provider={provider} size={size} className={className} />;
