import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { ExternalLink, Globe, KeyRound, Loader2, Play, Search, Shield, Upload } from 'lucide-react';
import { api } from '../../../api';
import { DeviceCodePanel } from '../DeviceCodePanel';
import type { DeviceCodeState, ImportMode, SocialLoginState } from '../helpers';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { SecretInput } from '@/components/ui/secret-input';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { FileDropzone } from './file-dropzone';
import {
    DEVICE_CODE_FALLBACK_SECS,
    errorMessage,
    pickResultId,
    type CreateResult,
    type OnSuccess,
} from './helpers';
import { OAuthWaitingPanel } from './oauth-waiting-panel';
import { SetupIntro } from './setup-intro';

type Props = {
    onSuccess: OnSuccess;
    onError: (message: string) => void;
    onBusyChange: (busy: boolean) => void;
    /** Current method — owned by the URL / setup-card select, not by this form. */
    initialMethod?: string;
};

const VALID_MODES: ImportMode[] = ['import', 'builder-id', 'social', 'idc', 'apikey'];

/**
 * Flow ids come from the backend catalog and are coarser than the panels this
 * form renders, so they need an explicit mapping — an unmapped id used to fall
 * through to the token scanner, which is why the API-key chip showed the wrong
 * body. The three ways to reuse an existing credential (scan, file, paste) are
 * one panel rather than three chips: they answer the same question.
 */
const FLOW_TO_MODE: Record<string, ImportMode> = {
    oauth: 'builder-id',
    'oauth-device': 'builder-id',
    detect: 'import',
    file: 'import',
    manual: 'import',
};

function resolveMode(method?: string): ImportMode {
    if (!method) return 'builder-id';
    if (VALID_MODES.includes(method as ImportMode)) return method as ImportMode;
    return FLOW_TO_MODE[method] ?? 'import';
}

/** Which of the three reuse-an-existing-credential paths the Import panel shows. */
type ImportWay = 'detect' | 'file' | 'manual';

const IMPORT_WAYS: { id: ImportWay; label: string; icon: ReactNode }[] = [
    { id: 'detect', label: 'Scan', icon: <Search size={13} /> },
    { id: 'file', label: 'File', icon: <Upload size={13} /> },
    { id: 'manual', label: 'Paste', icon: <Play size={13} /> },
];

export function KiroConnectionForm({ onSuccess, onError, onBusyChange, initialMethod }: Props) {
    const mode: ImportMode = resolveMode(initialMethod);

    const [loading, setLoading] = useState(false);
    const [importWay, setImportWay] = useState<ImportWay>('detect');
    const [apiKeyForm, setApiKeyForm] = useState({ apiKey: '', region: '' });
    const [form, setForm] = useState({
        refreshToken: '',
        clientId: '',
        clientSecret: '',
        region: '',
        authMethod: 'builder-id',
    });
    const [idcForm, setIdcForm] = useState({ startUrl: '', region: '' });
    const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null);
    const [codeExpired, setCodeExpired] = useState(false);
    const [polling, setPolling] = useState(false);
    const [socialLogin, setSocialLogin] = useState<SocialLoginState | null>(null);
    const [socialCallbackUrl, setSocialCallbackUrl] = useState('');
    const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const [pollExpiresAt, setPollExpiresAt] = useState<number | null>(null);

    const clearPolling = useCallback(() => {
        if (pollTimerRef.current) {
            clearTimeout(pollTimerRef.current);
            pollTimerRef.current = null;
        }
        setPollExpiresAt(null);
        setDeviceCode(null);
        setPolling(false);
        setCodeExpired(false);
    }, []);

    useEffect(() => () => {
        if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
    }, []);

    const formDirty =
        Boolean(form.refreshToken.trim()) ||
        Boolean(idcForm.startUrl.trim()) ||
        Boolean(apiKeyForm.apiKey.trim()) ||
        Boolean(socialCallbackUrl.trim());
    const busy = loading || polling || !!deviceCode || codeExpired || !!socialLogin || formDirty;

    useEffect(() => {
        onBusyChange(busy);
        return () => onBusyChange(false);
    }, [busy, onBusyChange]);

    /** Polls until success/failure/expiry. Expiry stops polling and flips recovery UI. */
    const startPolling = useCallback(
        (sessionId: string, interval: number, expiresInSecs?: number) => {
            const ms = Math.max(interval, 3) * 1000;
            const deadline = Date.now() + Math.max(30, expiresInSecs ?? DEVICE_CODE_FALLBACK_SECS) * 1000;
            setPollExpiresAt(deadline);
            const poll = async () => {
                if (Date.now() >= deadline) {
                    setPolling(false);
                    setCodeExpired(true);
                    return;
                }
                try {
                    const res = await api.pollAuth(sessionId);
                    if (res.status === 'pending') {
                        pollTimerRef.current = setTimeout(poll, ms);
                        return;
                    }
                    if (res.status === 'success') {
                        onSuccess(`Connected! ${res.email ? `(${res.email})` : res.name ?? ''}`, {
                            id: res.id,
                            name: res.name,
                        });
                        clearPolling();
                        return;
                    }
                    onError(res.errorDescription || res.error || 'Authorization failed');
                    clearPolling();
                } catch (error: unknown) {
                    // Network hiccup mid-poll: keep the session alive for manual callback instead of nuking progress.
                    onError(`${errorMessage(error, 'Authorization failed')} — retrying…`);
                    pollTimerRef.current = setTimeout(poll, Math.max(ms * 2, 10_000));
                }
            };
            pollTimerRef.current = setTimeout(poll, ms);
        },
        [onError, onSuccess, clearPolling],
    );

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
        const res = (await api.importConnection(data)) as CreateResult & { email?: string };
        onSuccess(message, { id: res.id, name: res.name });
    };

    const startBuilderIdFlow = () =>
        run(async () => {
            const res: DeviceCodeState = await api.startBuilderID();
            setDeviceCode(res);
            setPolling(true);
            startPolling(res.sessionId, res.interval, res.expiresIn);
        });

    return (
        <div className="space-y-5">
            {/* apikey */}
            {mode === 'apikey' && (
                <div className="space-y-4">
                    <SetupIntro
                        icon={<KeyRound size={20} className="text-primary" />}
                        title="Kiro API key"
                        description="A long-lived key, checked against AWS before saving. It never expires and is not refreshed."
                    />
                    <div className="space-y-3">
                        <div className="space-y-1">
                            <label htmlFor="kiro-api-key" className="text-xs font-medium">
                                API key <span className="text-destructive">*</span>
                            </label>
                            <SecretInput
                                id="kiro-api-key"
                                name="kiro-api-key"
                                revealLabel="API key"
                                value={apiKeyForm.apiKey}
                                onChange={(event) => setApiKeyForm({ ...apiKeyForm, apiKey: event.target.value })}
                                placeholder="ksk_…"
                                className="font-mono text-xs"
                            />
                        </div>
                        <div className="space-y-1">
                            <label htmlFor="kiro-api-key-region" className="text-xs font-medium">
                                Region <span className="font-normal text-muted-foreground">(optional)</span>
                            </label>
                            <Input
                                id="kiro-api-key-region"
                                autoComplete="off"
                                value={apiKeyForm.region}
                                onChange={(event) => setApiKeyForm({ ...apiKeyForm, region: event.target.value })}
                                placeholder="us-east-1"
                                className="text-xs"
                            />
                            <p className="text-[11px] leading-snug text-muted-foreground">
                                Leave empty for <code className="rounded bg-muted px-1 text-[10px]">us-east-1</code>.
                            </p>
                        </div>
                        <Button
                            onClick={() =>
                                void run(async () => {
                                    const res = (await api.addKiroApiKey({
                                        apiKey: apiKeyForm.apiKey.trim(),
                                        region: apiKeyForm.region.trim() || undefined,
                                    })) as CreateResult;
                                    setApiKeyForm({ apiKey: '', region: '' });
                                    onSuccess('API key connected!', pickResultId(res));
                                })
                            }
                            disabled={loading || !apiKeyForm.apiKey.trim()}
                            size="sm"
                            className="w-full gap-2"
                        >
                            {loading && <Loader2 size={13} className="animate-spin" />}
                            {loading ? 'Validating with AWS…' : 'Validate & save'}
                        </Button>
                    </div>
                </div>
            )}

            {/* import — scan, file and paste all answer "reuse an existing credential" */}
            {mode === 'import' && (
                <div className="space-y-4">
                    <SetupIntro
                        icon={<Upload size={20} className="text-primary" />}
                        title="Import existing credentials"
                        description={
                            <>
                                Reuse a Kiro login from{' '}
                                <code className="rounded bg-muted px-1.5 py-0.5 text-[11px]">kiro-auth-token.json</code>.
                            </>
                        }
                    />

                    <div className="flex w-fit gap-1 rounded-lg bg-muted p-1">
                        {IMPORT_WAYS.map((way) => (
                            <button
                                key={way.id}
                                type="button"
                                aria-pressed={importWay === way.id}
                                onClick={() => {
                                    setImportWay(way.id);
                                    onError('');
                                }}
                                className={cn(
                                    'flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition',
                                    importWay === way.id
                                        ? 'bg-background text-foreground shadow-sm'
                                        : 'text-muted-foreground hover:text-foreground',
                                )}
                            >
                                {way.icon}
                                {way.label}
                            </button>
                        ))}
                    </div>

                    {importWay === 'detect' && (
                        <div className="space-y-2">
                            <p className="text-xs leading-snug text-muted-foreground">
                                Looks for the token file in the standard Kiro and AWS SSO cache locations on this machine.
                            </p>
                            <Button
                                onClick={() =>
                                    void run(async () => {
                                        const res = await api.detectKiroToken();
                                        if (!res.found) {
                                            onError(res.error || 'No Kiro token found on this machine.');
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
                                {loading ? <Loader2 size={13} className="animate-spin" /> : <Search size={13} />}
                                {loading ? 'Scanning…' : 'Scan & import'}
                            </Button>
                        </div>
                    )}

                    {importWay === 'file' && (
                        <FileDropzone
                            disabled={loading}
                            title={loading ? 'Processing…' : 'Click to select a JSON file'}
                            hint="Only kiro-auth-token.json is supported"
                            ariaLabel="Select kiro-auth-token.json file"
                            onFiles={(files) =>
                                void run(async () => {
                                    const file = files[0];
                                    if (!file) return;
                                    let data: Record<string, unknown>;
                                    try {
                                        data = JSON.parse(await file.text());
                                    } catch {
                                        onError('The file is not valid JSON.');
                                        return;
                                    }
                                    if (!data.refreshToken) {
                                        onError('Invalid file: missing refreshToken.');
                                        return;
                                    }
                                    await importToken({
                                        refreshToken: String(data.refreshToken),
                                        clientId: String(data.clientId ?? ''),
                                        clientSecret: String(data.clientSecret ?? ''),
                                        region: String(data.region ?? ''),
                                        authMethod:
                                            typeof data.authMethod === 'string' ? data.authMethod.toLowerCase() : 'builder-id',
                                    });
                                })
                            }
                        />
                    )}

                    {importWay === 'manual' && (
                        <div className="space-y-3">
                            <div className="space-y-1">
                                <label htmlFor="kiro-refresh-token" className="text-xs font-medium">
                                    Refresh token <span className="text-destructive">*</span>
                                </label>
                                <Textarea
                                    id="kiro-refresh-token"
                                    autoComplete="off"
                                    spellCheck={false}
                                    value={form.refreshToken}
                                    onChange={(event) => setForm({ ...form, refreshToken: event.target.value })}
                                    placeholder="Paste the refreshToken value…"
                                    rows={3}
                                    className="font-mono text-xs"
                                />
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <div className="space-y-1">
                                    <label htmlFor="kiro-auth-method" className="text-xs font-medium">
                                        Auth method
                                    </label>
                                    <select
                                        id="kiro-auth-method"
                                        value={form.authMethod}
                                        onChange={(event) => setForm({ ...form, authMethod: event.target.value })}
                                        className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-xs shadow-sm"
                                    >
                                        <option value="builder-id">AWS Builder ID</option>
                                        <option value="idc">IAM Identity Center</option>
                                        <option value="social">Social login</option>
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
                                disabled={loading || !form.refreshToken.trim()}
                                size="sm"
                                className="w-full gap-2"
                            >
                                {loading && <Loader2 size={13} className="animate-spin" />}
                                {loading ? 'Validating…' : 'Import configuration'}
                            </Button>
                        </div>
                    )}
                </div>
            )}

            {/* builder-id */}
            {mode === 'builder-id' && (
                <div className="space-y-4">
                    <SetupIntro
                        icon={<ExternalLink size={20} className="text-orange-500" />}
                        title="AWS Builder ID"
                        description="Authenticate via AWS Builder ID using the device-code flow."
                    />
                    {!deviceCode && !codeExpired && (
                        <Button onClick={() => void startBuilderIdFlow()} disabled={loading} size="sm" className="gap-2">
                            <ExternalLink size={13} />
                            Start Login
                        </Button>
                    )}
                    {(deviceCode || codeExpired) && (
                        <DeviceCodePanel
                            userCode={deviceCode?.userCode ?? '—'}
                            verificationUri={deviceCode?.verificationUri}
                            verificationUriComplete={deviceCode?.verificationUriComplete}
                            expiresAtMs={pollExpiresAt ?? undefined}
                            expired={codeExpired}
                            waiting={polling}
                            onCancel={() => {
                                clearPolling();
                                onError('');
                            }}
                            onRestart={() => {
                                clearPolling();
                                void startBuilderIdFlow();
                            }}
                        />
                    )}
                </div>
            )}

            {/* idc */}
            {mode === 'idc' && (
                <div className="space-y-4">
                    <SetupIntro
                        icon={<Shield size={20} className="text-blue-500" />}
                        title="IAM Identity Center"
                        description="Enterprise SSO with a custom start URL."
                    />
                    {!deviceCode && !codeExpired && (
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
                                        const res: DeviceCodeState = await api.startIDC({
                                            startUrl: idcForm.startUrl,
                                            region: idcForm.region || undefined,
                                        });
                                        setDeviceCode(res);
                                        setPolling(true);
                                        startPolling(res.sessionId, res.interval, res.expiresIn);
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
                    {(deviceCode || codeExpired) && (
                        <DeviceCodePanel
                            userCode={deviceCode?.userCode ?? '—'}
                            verificationUri={deviceCode?.verificationUri}
                            verificationUriComplete={deviceCode?.verificationUriComplete}
                            expiresAtMs={pollExpiresAt ?? undefined}
                            expired={codeExpired}
                            waiting={polling}
                            onCancel={() => {
                                clearPolling();
                                onError('');
                            }}
                            onRestart={() =>
                                void run(async () => {
                                    const res: DeviceCodeState = await api.startIDC({
                                        startUrl: idcForm.startUrl,
                                        region: idcForm.region || undefined,
                                    });
                                    setDeviceCode(res);
                                    setPolling(true);
                                    startPolling(res.sessionId, res.interval, res.expiresIn);
                                })
                            }
                        />
                    )}
                </div>
            )}

            {/* social */}
            {mode === 'social' && (
                <div className="space-y-4">
                    {!socialLogin ? (
                        <>
                            <SetupIntro
                                icon={<Globe size={20} className="text-green-500" />}
                                title="Social login"
                                description="Google or GitHub via Kiro Identity."
                            />
                            <SocialProviderChoice
                                disabled={loading}
                                onStart={(provider) =>
                                    void run(async () => {
                                        const res = await api.startSocialLogin(provider);
                                        setSocialLogin({ ...res, provider });
                                        // No auto window.open here — it fires after `await` so
                                        // popup blockers kill it. The panel owns the CTA.
                                    })
                                }
                            />
                        </>
                    ) : (
                        <OAuthWaitingPanel
                            title="Finish social sign-in"
                            authUrl={socialLogin.loginUrl}
                            callbackValue={socialCallbackUrl}
                            onCallbackChange={setSocialCallbackUrl}
                            callbackPlaceholder="kiro://kiro.kiroAgent/authenticate-success?..."
                            onSubmit={() =>
                                void run(async () => {
                                    const res = await api.exchangeSocialCode({
                                        sessionId: socialLogin.sessionId,
                                        callbackUrl: socialCallbackUrl,
                                    });
                                    const picked = pickResultId(res);
                                    onSuccess('Social login connected!', picked);
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
                                <li>Open the sign-in page and authenticate with Google/GitHub.</li>
                                <li>
                                    After signing in, copy the callback URL{' '}
                                    <code className="rounded bg-muted px-1 text-[10px]">kiro://</code>.
                                </li>
                                <li>Paste it below to finish.</li>
                            </ol>
                        </OAuthWaitingPanel>
                    )}
                </div>
            )}

        </div>
    );
}

function SocialProviderChoice({
    disabled,
    onStart,
}: {
    disabled: boolean;
    onStart: (provider: 'google' | 'github') => void;
}) {
    const [choice, setChoice] = useState<'google' | 'github'>('google');
    return (
        <>
            <div className="flex justify-center gap-2">
                {(['google', 'github'] as const).map((provider) => (
                    <button
                        key={provider}
                        type="button"
                        aria-pressed={choice === provider}
                        onClick={() => setChoice(provider)}
                        className={cn(
                            'flex cursor-pointer items-center gap-2 rounded-lg border px-4 py-2.5 text-xs font-medium transition',
                            choice === provider
                                ? 'border-primary/30 bg-primary/5 text-primary'
                                : 'border-border bg-transparent text-muted-foreground hover:bg-muted',
                        )}
                    >
                        <Globe size={14} />
                        {provider === 'google' ? 'Google' : 'GitHub'}
                    </button>
                ))}
            </div>
            <Button disabled={disabled} size="sm" className="gap-2" onClick={() => onStart(choice)}>
                <Globe size={13} />
                Continue with {choice === 'google' ? 'Google' : 'GitHub'}
            </Button>
        </>
    );
}
