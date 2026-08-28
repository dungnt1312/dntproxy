import type { ReactNode } from 'react';
import { ExternalLink, Globe, KeyRound, Play, Search, Shield, Upload } from 'lucide-react';

/**
 * Human-readable labels for connection auth methods. One source of truth for
 * the provider-picker chips, the setup-card method switch, and result copy.
 */

export interface AuthFlowInfo {
    id: string;
    label: string;
    /** Short chip text, e.g. inside provider cards. */
    short: string;
    description: string;
    icon: ReactNode;
}

const FLOWS: Record<string, Omit<AuthFlowInfo, 'id'>> = {
    oauth: {
        label: 'OAuth sign-in',
        short: 'OAuth',
        description: 'Connect securely in your browser — no key pasting.',
        icon: <ExternalLink size={12} />,
    },
    apikey: {
        label: 'API key',
        short: 'API key',
        description: 'Paste a key from the provider console.',
        icon: <KeyRound size={12} />,
    },
    'builder-id': {
        label: 'AWS Builder ID',
        short: 'Builder ID',
        description: 'Device-code sign-in for AWS Builder ID.',
        icon: <ExternalLink size={12} />,
    },
    'oauth-device': {
        label: 'Device code',
        short: 'Device',
        description: 'Sign in by entering a short code on the provider site.',
        icon: <ExternalLink size={12} />,
    },
    idc: {
        label: 'IAM Identity Center',
        short: 'IAM IDC',
        description: 'Enterprise SSO with a custom start URL.',
        icon: <Shield size={12} />,
    },
    social: {
        label: 'Social login',
        short: 'Social',
        description: 'Google or GitHub via Kiro Identity.',
        icon: <Globe size={12} />,
    },
    detect: {
        label: 'Auto detect',
        short: 'Detect',
        description: 'Scan for an existing token on this machine.',
        icon: <Search size={12} />,
    },
    file: {
        label: 'Import file',
        short: 'File',
        description: 'Upload an existing credential JSON.',
        icon: <Upload size={12} />,
    },
    import: {
        label: 'Import credentials',
        short: 'Import',
        description: 'Import existing credentials.',
        icon: <Upload size={12} />,
    },
    manual: {
        label: 'Manual entry',
        short: 'Manual',
        description: 'Paste refresh token / config manually.',
        icon: <Play size={12} />,
    },
};

function fallback(id: string): Omit<AuthFlowInfo, 'id'> {
    const human = id.replace(/[-_]/g, ' ');
    return {
        label: human.charAt(0).toUpperCase() + human.slice(1),
        short: human,
        description: `Connect using ${human}.`,
        icon: <KeyRound size={12} />,
    };
}

export function authFlowInfo(id: string): AuthFlowInfo {
    return { id, ...(FLOWS[id] ?? fallback(id)) };
}

/** Provider's auth flows mapped to display info, preserving backend order. */
export function authFlowOptions(authFlows: readonly string[]): AuthFlowInfo[] {
    return authFlows.map(authFlowInfo);
}
