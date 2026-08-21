import { useCallback, useEffect, useRef, useState } from 'react';
import { ExternalLink, GitBranch, Globe, Play, Search, Shield, Upload } from 'lucide-react';
import { api } from '../../../api';
import { DeviceCodePanel } from '../DeviceCodePanel';
import type { DeviceCodeState, ImportMode, SocialLoginState } from '../helpers';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { AuthMethodSelector } from './auth-method-selector';
import { FileDropzone } from './file-dropzone';
import { errorMessage } from './helpers';
import { OAuthWaitingPanel } from './oauth-waiting-panel';
import { SetupIntro } from './setup-intro';

type Props = {
    onSuccess: (message: string) => void;
    onError: (message: string) => void;
    onBusyChange: (busy: boolean) => void;
    initialMethod?: string;
    onMethodChange?: (method: string) => void;
};

const PRIMARY_MODES = [
    { id: 'detect' as const, label: 'Auto Detect', description: 'Scan local token', icon: <Search size={13} />, recommended: true },
    { id: 'builder-id' as const, label: 'Builder ID', description: 'AWS Builder ID', icon: <ExternalLink size={13} /> },
];

const MORE_MODES = [
    { id: 'social' as const, label: 'Social Login', description: 'Google / GitHub', icon: <Globe size={13} /> },
    { id: 'idc' as const, label: 'IAM IDC', description: 'Enterprise SSO', icon: <Shield size={13} /> },
    { id: 'file' as const, label: 'Import File', description: 'Upload JSON file', icon: <Upload size={13} /> },
    { id: 'manual' as const, label: 'Manual', description: 'Paste tokens', icon: <Play size={13} /> },
];

export function KiroConnectionForm({ onSuccess, onError, onBusyChange, initialMethod, onMethodChange }: Props) {
    const [mode, setMode] = useState<ImportMode>(() =>
        initialMethod && [...PRIMARY_MODES, ...MORE_MODES].some(({ id }) => id === initialMethod)
            ? initialMethod as ImportMode
            : 'detect',
    );
    const [loading, setLoading] = useState(false);
    const [form, setForm] = useState({
        refreshToken: '',
        clientId: '',
        clientSecret: '',
        region: '',
        authMethod: 'builder-id',
    });
    const [idcForm, setIdcForm] = useState({ startUrl: '', region: '' });
    const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null);
    const [polling, setPolling] = useState(false);
    const [socialLogin, setSocialLogin] = useState<SocialLoginState | null>(null);
    const [socialCallbackUrl, setSocialCallbackUrl] = useState('');
    const [socialProvider, setSocialProvider] = useState<'google' | 'github'>('google');
    const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    const clearPolling = useCallback(() => {
        if (pollTimerRef.current) {
            clearTimeout(pollTimerRef.current);
            pollTimerRef.current = null;
        }
        setDeviceCode(null);
        setPolling(false);
    }, []);

    useEffect(() => () => {
        if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
    }, []);

    useEffect(() => {
        if (initialMethod && [...PRIMARY_MODES, ...MORE_MODES].some(({ id }) => id === initialMethod)) {
            setMode(initialMethod as ImportMode);
        }
    }, [initialMethod]);

    const formDirty =
        Boolean(form.refreshToken.trim()) ||
        Boolean(idcForm.startUrl.trim()) ||
        Boolean(socialCallbackUrl.trim());
    const busy = loading || polling || !!deviceCode || !!socialLogin || formDirty;

    useEffect(() => {
        onBusyChange(busy);
        return () => onBusyChange(false);
    }, [busy, onBusyChange]);

    const startPolling = useCallback((sessionId: string, interval: number) => {
        const ms = Math.max(interval, 3) * 1000;
        const poll = async () => {
            try {
                const res = await api.pollAuth(sessionId);
                if (res.status === 'pending') {
                    pollTimerRef.current = setTimeout(poll, ms);
                    return;
                }
                if (res.status === 'success') {
                    onSuccess(`Connected! ${res.email ? `(${res.email})` : res.name ?? ''}`);
                    return;
                }
                onError(res.errorDescription || res.error || 'Authorization failed');
                setDeviceCode(null);
                setPolling(false);
            } catch (error: unknown) {
                onError(errorMessage(error, 'Authorization failed'));
                setDeviceCode(null);
                setPolling(false);
            }
        };
        pollTimerRef.current = setTimeout(poll, ms);
    }, [onError, onSuccess]);

    const run = async (fn: () => Promise<void>) => {
        setLoading(true);
        onError('');
        try {
            await fn();
        } catch (error: unknown) {
            onError(errorMessage(error, 'Kiro setup failed'));
        } finally {
            setLoading(false);
        }
    };

    const importToken = async (
        data: {
            refreshToken: string;
            clientId?: string;
            clientSecret?: string;
            region?: string;
            authMethod?: string;
        },
        message = 'Imported!',
    ) => {
        await api.importConnection(data);
        onSuccess(message);
    };

    return (
        <div className="space-y-5">
            <AuthMethodSelector
                value={mode}
                onChange={(next) => {
                    clearPolling();
                    setSocialLogin(null);
                    setSocialCallbackUrl('');
                    setMode(next);
                    onMethodChange?.(next);
                    onError('');
                }}
                primary={PRIMARY_MODES}
                more={MORE_MODES}
            />

            <div className="border-t pt-5">
                {mode === 'detect' && (
                    <div className="mx-auto max-w-sm space-y-4 text-center">
                        <SetupIntro
                            icon={<Search size={20} className="text-primary" />}
                            title="Auto Detect & Import"
                            description={
                                <>
                                    Scan for Kiro credentials from{' '}
                                    <code className="rounded bg-muted px-1.5 py-0.5 text-[11px]">kiro-auth-token.json</code> on
                                    your machine.
                                </>
                            }
                        />
                        <Button
                            onClick={() =>
                                void run(async () => {
                                    const res = await api.detectKiroToken();
                                    if (!res.found) {
                                        onError(res.error || 'No Kiro token found.');
                                        return;
                                    }
                                    await importToken(
                                        {
                                            refreshToken: res.refreshToken,
                                            clientId: res.clientId || '',
                                            clientSecret: res.clientSecret || '',
                                            region: res.region || '',
                                            authMethod: res.authMethod || 'builder-id',
                                        },
                                        'Connection imported!',
                                    );
                                })
                            }
                            disabled={loading}
                            size="sm"
                            className="gap-2"
                        >
                            <Search size={13} />
                            {loading ? 'Detecting…' : 'Scan & Import'}
                        </Button>
                    </div>
                )}

                {mode === 'builder-id' && (
                    <div className="mx-auto max-w-sm space-y-4 text-center">
                        <SetupIntro
                            icon={<ExternalLink size={20} className="text-orange-500" />}
                            title="AWS Builder ID"
                            description="Authenticate via AWS Builder ID using Device Code Flow."
                        />
                        {!deviceCode && (
                            <Button
                                onClick={() =>
                                    void run(async () => {
                                        const res = await api.startBuilderID();
                                        setDeviceCode(res);
                                        setPolling(true);
                                        startPolling(res.sessionId, res.interval);
                                    })
                                }
                                disabled={loading}
                                size="sm"
                                className="gap-2"
                            >
                                <ExternalLink size={13} />
                                Start Login
                            </Button>
                        )}
                        {deviceCode && (
                            <DeviceCodePanel
                                userCode={deviceCode.userCode}
                                verificationUri={deviceCode.verificationUri}
                                verificationUriComplete={deviceCode.verificationUriComplete}
                                intervalSec={deviceCode.interval}
                                waiting={polling}
                                onCancel={clearPolling}
                            />
                        )}
                    </div>
                )}

                {mode === 'idc' && (
                    <div className="mx-auto max-w-sm space-y-4">
                        <SetupIntro
                            icon={<Shield size={20} className="text-blue-500" />}
                            title="IAM Identity Center"
                            description="Enterprise SSO with custom start URL."
                        />
                        {!deviceCode && (
                            <div className="space-y-3">
                                <div className="space-y-1 text-left">
                                    <label htmlFor="idc-start-url" className="text-xs font-medium">
                                        Start URL <span className="text-destructive">*</span>
                                    </label>
                                    <Input
                                        id="idc-start-url"
                                        autoComplete="off"
                                        value={idcForm.startUrl}
                                        onChange={(event) => setIdcForm({ ...idcForm, startUrl: event.target.value })}
                                        placeholder="https://mycompany.awsapps.com/start"
                                        className="text-xs"
                                    />
                                </div>
                                <div className="space-y-1 text-left">
                                    <label htmlFor="idc-region" className="text-xs font-medium">
                                        Region <span className="font-normal text-muted-foreground">(optional)</span>
                                    </label>
                                    <Input
                                        id="idc-region"
                                        autoComplete="off"
                                        value={idcForm.region}
                                        onChange={(event) => setIdcForm({ ...idcForm, region: event.target.value })}
                                        placeholder="us-east-1"
                                        className="text-xs"
                                    />
                                </div>
                                <Button
                                    onClick={() =>
                                        void run(async () => {
                                            if (!idcForm.startUrl) {
                                                onError('Start URL is required');
                                                return;
                                            }
                                            const res = await api.startIDC({
                                                startUrl: idcForm.startUrl,
                                                region: idcForm.region || undefined,
                                            });
                                            setDeviceCode(res);
                                            setPolling(true);
                                            startPolling(res.sessionId, res.interval);
                                        })
                                    }
                                    disabled={loading || !idcForm.startUrl}
                                    size="sm"
                                    className="w-full gap-2"
                                >
                                    <ExternalLink size={13} />
                                    Start Login
                                </Button>
                            </div>
                        )}
                        {deviceCode && (
                            <DeviceCodePanel
                                userCode={deviceCode.userCode}
                                verificationUri={deviceCode.verificationUri}
                                verificationUriComplete={deviceCode.verificationUriComplete}
                                intervalSec={deviceCode.interval}
                                waiting={polling}
                                onCancel={clearPolling}
                            />
                        )}
                    </div>
                )}

                {mode === 'social' && (
                    <div className="mx-auto max-w-sm space-y-4 text-center">
                        <SetupIntro
                            icon={<Globe size={20} className="text-green-500" />}
                            title="Social Login"
                            description="Authenticate with Google or GitHub via Kiro Identity."
                        />
                        {!socialLogin && (
                            <>
                                <div className="flex justify-center gap-2">
                                    {(['google', 'github'] as const).map((provider) => (
                                        <button
                                            key={provider}
                                            type="button"
                                            aria-pressed={socialProvider === provider}
                                            onClick={() => setSocialProvider(provider)}
                                            className={cn(
                                                'flex cursor-pointer items-center gap-2 rounded-lg border px-4 py-2.5 text-xs font-medium transition',
                                                socialProvider === provider
                                                    ? 'border-primary/30 bg-primary/5 text-primary'
                                                    : 'border-border bg-transparent text-muted-foreground hover:bg-muted',
                                            )}
                                        >
                                            {provider === 'google' ? <Globe size={14} /> : <GitBranch size={14} />}
                                            {provider === 'google' ? 'Google' : 'GitHub'}
                                        </button>
                                    ))}
                                </div>
                                <Button
                                    onClick={() =>
                                        void run(async () => {
                                            const res = await api.startSocialLogin(socialProvider);
                                            setSocialLogin({ ...res, provider: socialProvider });
                                            window.open(res.loginUrl, '_blank');
                                        })
                                    }
                                    disabled={loading}
                                    size="sm"
                                    className="gap-2"
                                >
                                    <Globe size={13} />
                                    Start Login
                                </Button>
                            </>
                        )}
                        {socialLogin && (
                            <OAuthWaitingPanel
                                title="Finish social login"
                                authUrl={socialLogin.loginUrl}
                                callbackValue={socialCallbackUrl}
                                onCallbackChange={setSocialCallbackUrl}
                                callbackPlaceholder="kiro://kiro.kiroAgent/authenticate-success?..."
                                onSubmit={() =>
                                    void run(async () => {
                                        await api.exchangeSocialCode({
                                            sessionId: socialLogin.sessionId,
                                            callbackUrl: socialCallbackUrl,
                                        });
                                        onSuccess('Social login connected!');
                                    })
                                }
                                onCancel={() => {
                                    setSocialLogin(null);
                                    setSocialCallbackUrl('');
                                    onError('');
                                }}
                                loading={loading}
                            >
                                <ol className="list-decimal space-y-1 pl-4 text-left text-xs text-muted-foreground">
                                    <li>Login page opened in the browser.</li>
                                    <li>
                                        After login, copy the{' '}
                                        <code className="rounded bg-muted px-1 text-[10px]">kiro://</code> callback URL.
                                    </li>
                                    <li>Paste it below to complete.</li>
                                </ol>
                            </OAuthWaitingPanel>
                        )}
                    </div>
                )}

                {mode === 'file' && (
                    <div className="mx-auto max-w-sm space-y-4 text-center">
                        <SetupIntro
                            icon={<Upload size={20} className="text-primary" />}
                            title="Import from File"
                            description={
                                <>
                                    Upload your{' '}
                                    <code className="rounded bg-muted px-1.5 py-0.5 text-[11px]">kiro-auth-token.json</code>{' '}
                                    file.
                                </>
                            }
                        />
                        <FileDropzone
                            disabled={loading}
                            title={loading ? 'Processing…' : 'Click to select JSON file'}
                            hint="Only kiro-auth-token.json supported"
                            ariaLabel="Select kiro-auth-token.json file"
                            onFiles={(files) =>
                                void run(async () => {
                                    const file = files[0];
                                    if (!file) return;
                                    const data = JSON.parse(await file.text());
                                    if (!data.refreshToken) {
                                        onError('Invalid file: missing refreshToken');
                                        return;
                                    }
                                    await importToken({
                                        refreshToken: data.refreshToken,
                                        clientId: data.clientId || '',
                                        clientSecret: data.clientSecret || '',
                                        region: data.region || '',
                                        authMethod: data.authMethod?.toLowerCase() || 'builder-id',
                                    });
                                })
                            }
                        />
                    </div>
                )}

                {mode === 'manual' && (
                    <div className="mx-auto max-w-md space-y-4">
                        <SetupIntro
                            icon={<Play size={20} className="text-amber-500" />}
                            title="Manual Import"
                            description="Paste your refresh token and configuration manually."
                        />
                        <div className="space-y-3">
                            <div className="space-y-1">
                                <label htmlFor="kiro-refresh-token" className="text-xs font-medium">
                                    Refresh Token <span className="text-destructive">*</span>
                                </label>
                                <Textarea
                                    id="kiro-refresh-token"
                                    autoComplete="off"
                                    spellCheck={false}
                                    value={form.refreshToken}
                                    onChange={(event) => setForm({ ...form, refreshToken: event.target.value })}
                                    placeholder="Paste the refresh token from kiro-auth-token.json…"
                                    rows={3}
                                    className="font-mono text-xs"
                                />
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <div className="space-y-1">
                                    <label htmlFor="kiro-auth-method" className="text-xs font-medium">
                                        Auth Method
                                    </label>
                                    <select
                                        id="kiro-auth-method"
                                        value={form.authMethod}
                                        onChange={(event) => setForm({ ...form, authMethod: event.target.value })}
                                        className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-xs shadow-sm"
                                    >
                                        <option value="builder-id">AWS Builder ID</option>
                                        <option value="idc">IAM Identity Center</option>
                                        <option value="social">Social Login</option>
                                    </select>
                                </div>
                                <div className="space-y-1">
                                    <label htmlFor="kiro-region" className="text-xs font-medium">
                                        Region <span className="font-normal text-muted-foreground">(optional)</span>
                                    </label>
                                    <Input
                                        id="kiro-region"
                                        autoComplete="off"
                                        value={form.region}
                                        onChange={(event) => setForm({ ...form, region: event.target.value })}
                                        placeholder="us-east-1"
                                        className="text-xs"
                                    />
                                </div>
                            </div>
                            <Button
                                onClick={() => void run(async () => importToken(form))}
                                disabled={loading || !form.refreshToken}
                                size="sm"
                                className="w-full"
                            >
                                {loading ? 'Validating…' : 'Import Configuration'}
                            </Button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
