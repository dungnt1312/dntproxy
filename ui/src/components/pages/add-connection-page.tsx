import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
    ArrowLeft,
    AlertTriangle,
    CheckCircle2,
    ExternalLink,
    Globe,
    GitBranch,
    KeyRound,
    Loader2,
    Play,
    Search,
    Shield,
    Upload,
} from 'lucide-react';
import { api } from '../../api';
import type { ImportMode, DeviceCodeState, SocialLoginState } from '../connections/helpers';
import { ProviderLogoIcon } from '../connections/helpers';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { getProviderMeta } from '@/lib/provider-registry';
import { useProviderCatalog } from '@/lib/use-provider-catalog';
import { ApiKeyConnectionForm } from '../connections/ApiKeyConnectionForm';
import type { CreateConnectionPayload } from '@/types/provider-metadata';

export default function AddConnectionPage() {
    const navigate = useNavigate();
    const { providers: providerCatalog } = useProviderCatalog();
    const [provider, setProvider] = useState('kiro');
    const [providerSearch, setProviderSearch] = useState('');
    const [importMode, setImportMode] = useState<ImportMode>('detect');

    // Kiro OAuth state
    const [form, setForm] = useState({
        refreshToken: '',
        clientId: '',
        clientSecret: '',
        region: '',
        authMethod: 'builder-id',
    });

    // Qwen OAuth state
    const [qwenMode, setQwenMode] = useState<'oauth' | 'apikey'>('oauth');
    const [qwenDeviceCode, setQwenDeviceCode] = useState<DeviceCodeState | null>(null);
    const [qwenPolling, setQwenPolling] = useState(false);
    const qwenPollRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Common state
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    // Kiro device code state
    const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null);
    const [polling, setPolling] = useState(false);
    const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const [idcForm, setIdcForm] = useState({ startUrl: '', region: '' });
    const [socialLogin, setSocialLogin] = useState<SocialLoginState | null>(null);
    const [socialCallbackUrl, setSocialCallbackUrl] = useState('');
    const [socialProvider, setSocialProvider] = useState<'google' | 'github'>('google');

    // OpenAI OAuth state
    const [openaiMode, setOpenaiMode] = useState<'oauth' | 'apikey'>('oauth');
    const [openaiOAuthSession, setOpenaiOAuthSession] = useState<{
        sessionId: string;
        authUrl: string;
    } | null>(null);
    const [openaiManualCallback, setOpenaiManualCallback] = useState('');
    const openaiPollRef = useRef<ReturnType<typeof setInterval> | null>(null);

    // xAI OAuth state
    const [xaiMode, setXaiMode] = useState<'oauth' | 'file'>('oauth');
    const [xaiOAuthSession, setXaiOAuthSession] = useState<{
        sessionId: string;
        authUrl: string;
        redirectUri?: string;
    } | null>(null);
    const [xaiManualCallback, setXaiManualCallback] = useState('');

    useEffect(() => {
        return () => {
            if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
            if (openaiPollRef.current) clearInterval(openaiPollRef.current);
            if (qwenPollRef.current) clearTimeout(qwenPollRef.current);
        };
    }, []);

    const clearKiroPolling = useCallback(() => {
        if (pollTimerRef.current) {
            clearTimeout(pollTimerRef.current);
            pollTimerRef.current = null;
        }
        setDeviceCode(null);
        setPolling(false);
    }, []);

    const clearOpenAIPolling = useCallback(() => {
        if (openaiPollRef.current) {
            clearInterval(openaiPollRef.current);
            openaiPollRef.current = null;
        }
        setOpenaiOAuthSession(null);
        setOpenaiManualCallback('');
    }, []);

    const clearXAIAuth = useCallback(() => {
        setXaiOAuthSession(null);
        setXaiManualCallback('');
        setXaiMode('oauth');
    }, []);

    const clearQwenPolling = useCallback(() => {
        if (qwenPollRef.current) {
            clearTimeout(qwenPollRef.current);
            qwenPollRef.current = null;
        }
        setQwenDeviceCode(null);
        setQwenPolling(false);
    }, []);

    const resetForm = () => {
        setForm({
            refreshToken: '',
            clientId: '',
            clientSecret: '',
            region: '',
            authMethod: 'builder-id',
        });
        clearQwenPolling();
        setIdcForm({ startUrl: '', region: '' });
        clearKiroPolling();
        setSocialLogin(null);
        setSocialCallbackUrl('');
        clearOpenAIPolling();
        clearXAIAuth();
        setError('');
    };

    const parseSupportedModels = (str: string) =>
        str
            .split('\n')
            .map((s) => s.trim())
            .filter(Boolean);

    const handleSuccess = (msg: string) => {
        resetForm();
        toast.success(msg);
        navigate('/connections');
    };

    const handleCreateConnection = async (payload: CreateConnectionPayload) => {
        setLoading(true);
        setError('');
        try {
            await api.createConnection(payload);
            handleSuccess(`${getProviderMeta(payload.provider).label} connected!`);
        } catch (e: unknown) {
            setError(e instanceof Error ? e.message : 'Failed to add connection');
        } finally {
            setLoading(false);
        }
    };

    const selectedProviderMeta = providerCatalog.find((p) => p.id === provider);

    const handleStartBuilderID = async () => {
        setLoading(true);
        setError('');
        try {
            const res = await api.startBuilderID();
            setDeviceCode(res);
            setPolling(true);
            startPolling(res.sessionId, res.interval);
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleStartIDC = async () => {
        if (!idcForm.startUrl) {
            setError('Start URL is required');
            return;
        }
        setLoading(true);
        setError('');
        try {
            const res = await api.startIDC({
                startUrl: idcForm.startUrl,
                region: idcForm.region || undefined,
            });
            setDeviceCode(res);
            setPolling(true);
            startPolling(res.sessionId, res.interval);
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

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
                    handleSuccess(`Connected! ${res.email ? `(${res.email})` : res.name}`);
                    return;
                }
                setError(res.errorDescription || res.error || 'Authorization failed');
                setDeviceCode(null);
                setPolling(false);
            } catch (e: any) {
                setError(e.message);
                setDeviceCode(null);
                setPolling(false);
            }
        };
        pollTimerRef.current = setTimeout(poll, ms);
    }, []);

    const handleStartSocial = async () => {
        setLoading(true);
        setError('');
        try {
            const res = await api.startSocialLogin(socialProvider);
            setSocialLogin({ ...res, provider: socialProvider });
            window.open(res.loginUrl, '_blank');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleExchangeSocial = async () => {
        if (!socialLogin || !socialCallbackUrl) return;
        setLoading(true);
        setError('');
        try {
            await api.exchangeSocialCode({
                sessionId: socialLogin.sessionId,
                callbackUrl: socialCallbackUrl,
            });
            handleSuccess('Social login connected!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleDetect = async () => {
        setLoading(true);
        setError('');
        try {
            const res = await api.detectKiroToken();
            if (res.found) {
                await api.importConnection({
                    refreshToken: res.refreshToken,
                    clientId: res.clientId || '',
                    clientSecret: res.clientSecret || '',
                    region: res.region || '',
                    authMethod: res.authMethod || 'builder-id',
                });
                handleSuccess('Connection imported!');
            } else setError(res.error || 'No Kiro token found.');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const input = e.currentTarget;
        const file = input.files?.[0];
        if (!file) return;
        setLoading(true);
        setError('');
        try {
            const data = JSON.parse(await file.text());
            if (!data.refreshToken) {
                setError('Invalid file: missing refreshToken');
                return;
            }
            await api.importConnection({
                refreshToken: data.refreshToken,
                clientId: data.clientId || '',
                clientSecret: data.clientSecret || '',
                region: data.region || '',
                authMethod: data.authMethod?.toLowerCase() || 'builder-id',
            });
            handleSuccess('Imported!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            input.value = '';
            setLoading(false);
        }
    };

    const handleManualImport = async () => {
        setLoading(true);
        setError('');
        try {
            await api.importConnection(form);
            handleSuccess('Imported!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleAddOpenAI = () => { void handleCreateConnection; }; // handled by ApiKeyConnectionForm

    const handleStartXAIOAuth = async () => {
        setLoading(true);
        setError('');
        try {
            const res = await api.startXAIOAuth();
            setXaiOAuthSession({
                sessionId: res.sessionId,
                authUrl: res.authUrl,
                redirectUri: res.redirectUri,
            });
            window.open(res.authUrl, '_blank');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleExchangeXAIOAuth = async () => {
        if (!xaiOAuthSession || !xaiManualCallback.trim()) return;
        setLoading(true);
        setError('');
        try {
            const res = await api.exchangeXAIOAuth(xaiOAuthSession.sessionId, xaiManualCallback.trim());
            handleSuccess(`Grok Build connected! ${res.email || res.name || ''}`);
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleXAIFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const input = e.currentTarget;
        const files = input.files;
        if (!files || files.length === 0) return;
        await processXAIFiles(Array.from(files));
        input.value = '';
    };

    const processXAIFile = async (file: File): Promise<{ email: string; duplicate: boolean; error?: string }> => {
        const data = JSON.parse(await file.text());
        const res = await api.importXAIAuthFile(data);
        return { email: res.email || '', duplicate: !!res.duplicate };
    };

    const processXAIFiles = async (files: File[]) => {
        setLoading(true);
        setError('');
        let imported = 0;
        let skipped = 0;
        const errors: string[] = [];
        for (const file of files) {
            try {
                const result = await processXAIFile(file);
                if (result.duplicate) {
                    skipped++;
                } else {
                    imported++;
                }
            } catch (e: any) {
                errors.push(`${file.name}: ${e.message}`);
            }
        }
        setLoading(false);
        if (imported > 0 || skipped > 0) {
            const parts: string[] = [];
            if (imported > 0) parts.push(`${imported} imported`);
            if (skipped > 0) parts.push(`${skipped} skipped (duplicate)`);
            if (errors.length > 0) parts.push(`${errors.length} failed`);
            handleSuccess(`Grok batch import: ${parts.join(', ')}`);
        } else if (errors.length > 0) {
            setError(errors.join('\n'));
        }
    };

    const handleAddCustom = async () => {
        // handled by ApiKeyConnectionForm now, kept for unused reference check
        void handleCreateConnection;
    };

    const handleStartQwenOAuth = async () => {
        setLoading(true);
        setError('');
        try {
            const res = await api.startQwenOAuth();
            setQwenDeviceCode(res);
            setQwenPolling(true);
            startQwenPolling(res.sessionId, res.interval);
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const startQwenPolling = useCallback((sessionId: string, interval: number) => {
        const ms = Math.max(interval, 3) * 1000;
        const poll = async () => {
            try {
                const res = await api.pollQwenOAuth(sessionId);
                if (res.status === 'pending') {
                    qwenPollRef.current = setTimeout(poll, ms);
                    return;
                }
                if (res.status === 'success') {
                    handleSuccess(`Qwen connected! ${res.email ? `(${res.email})` : res.name}`);
                    return;
                }
                setError(res.errorDescription || res.error || 'Authorization failed');
                setQwenDeviceCode(null);
                setQwenPolling(false);
            } catch (e: any) {
                setError(e.message);
                setQwenDeviceCode(null);
                setQwenPolling(false);
            }
        };
        qwenPollRef.current = setTimeout(poll, ms);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    // Removed: handleAddQwenAPIKey, handleAddAnthropic, handleAddGemini, handleAddCline
    // These providers now use ApiKeyConnectionForm + handleCreateConnection

    const DeviceCodePanel = () =>
        deviceCode ? (
            <div className="space-y-3 mt-4 rounded-xl border bg-muted/40 p-5">
                <div className="text-center">
                    <p className="text-xs text-muted-foreground mb-2">Enter this code on the authorization page:</p>
                    <div className="text-3xl font-mono font-bold tracking-[0.25em] text-primary mb-4 select-all">
                        {deviceCode.userCode}
                    </div>
                    <Button asChild size="sm" className="gap-2">
                        <a
                            href={deviceCode.verificationUriComplete || deviceCode.verificationUri}
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            <ExternalLink size={14} /> Open Authorization Page
                        </a>
                    </Button>
                </div>
                {polling && (
                    <div className="flex items-center justify-center gap-2 text-xs text-muted-foreground mt-3 pt-3 border-t">
                        <Loader2 size={12} className="animate-spin" /> Waiting for authorization ({deviceCode.interval}
                        s)…
                    </div>
                )}
            </div>
        ) : null;

    const providerTabs = providerCatalog.map((p) => ({
        id: p.id,
        name: p.name,
        icon: <ProviderLogoIcon provider={p.id} size={24} />,
        description: p.ui.description,
        auth: p.ui.authFlows.join(', '),
        recommended: p.id === 'kiro' || p.id === 'openai',
    }));

    const kiroModes = [
        {
            id: 'detect' as ImportMode,
            label: 'Auto Detect',
            icon: <Search size={13} />,
            desc: 'Scan local token',
        },
        {
            id: 'builder-id' as ImportMode,
            label: 'Builder ID',
            icon: <ExternalLink size={13} />,
            desc: 'AWS Builder ID',
        },
        {
            id: 'social' as ImportMode,
            label: 'Social Login',
            icon: <Globe size={13} />,
            desc: 'Google / GitHub',
        },
        {
            id: 'idc' as ImportMode,
            label: 'IAM IDC',
            icon: <Shield size={13} />,
            desc: 'Enterprise SSO',
        },
        {
            id: 'file' as ImportMode,
            label: 'Import File',
            icon: <Upload size={13} />,
            desc: 'Upload JSON file',
        },
        {
            id: 'manual' as ImportMode,
            label: 'Manual',
            icon: <Play size={13} />,
            desc: 'Paste tokens',
        },
    ];

    const providerQuery = providerSearch.trim().toLowerCase();
    const filteredProviderTabs = providerQuery
        ? providerTabs.filter((p) =>
              [p.name, p.description, p.auth, p.id].join(' ').toLowerCase().includes(providerQuery),
          )
        : providerTabs;
    const selectedProvider = providerTabs.find((p) => p.id === provider);
    const selectedProviderHidden =
        Boolean(providerQuery) && !filteredProviderTabs.some((p) => p.id === provider);

    const handleProviderSelect = (providerId: string) => {
        setProvider(providerId);
        resetForm();
        setProviderSearch('');
    };

    const renderApiKeyProviderForm = ({
        providerId,
        title,
        description,
        form,
        setForm,
        onSubmit,
        docsUrl,
        docsLabel,
    }: {
        providerId: 'anthropic' | 'gemini' | 'cline';
        title: string;
        description: string;
        form: { name: string; apiKey: string; baseUrl: string; supportedModels: string };
        setForm: (value: { name: string; apiKey: string; baseUrl: string; supportedModels: string }) => void;
        onSubmit: () => void;
        docsUrl: string;
        docsLabel: string;
    }) => {
        const meta = getProviderMeta(providerId);

        return (
            <div className="max-w-lg mx-auto space-y-5">
                <div className="text-center">
                    <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-muted mx-auto">
                        <ProviderLogoIcon provider={providerId} size={20} />
                    </div>
                    <p className="font-medium text-sm mt-3 mb-1">{title}</p>
                    <p className="text-xs text-muted-foreground">
                        {description}{' '}
                        <a
                            href={docsUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-primary hover:underline"
                        >
                            {docsLabel}
                        </a>
                        .
                    </p>
                </div>
                <div className="space-y-3">
                    <Input
                        type="password"
                        value={form.apiKey}
                        onChange={(e) => setForm({ ...form, apiKey: e.target.value })}
                        placeholder="API Key *"
                        className="text-xs font-mono"
                    />
                    <div className="grid grid-cols-2 gap-3">
                        <Input
                            value={form.name}
                            onChange={(e) => setForm({ ...form, name: e.target.value })}
                            placeholder="Display Name (optional)"
                            className="text-xs"
                        />
                        <Input
                            value={form.baseUrl}
                            onChange={(e) => setForm({ ...form, baseUrl: e.target.value })}
                            placeholder="Base URL (optional)"
                            className="text-xs font-mono"
                        />
                    </div>
                    <Textarea
                        value={form.supportedModels}
                        onChange={(e) => setForm({ ...form, supportedModels: e.target.value })}
                        placeholder="Supported Models (one per line, auto-populated if empty)"
                        rows={3}
                        className="text-xs font-mono"
                    />
                    <Button
                        onClick={onSubmit}
                        disabled={loading || !form.apiKey}
                        size="sm"
                        className={cn('w-full', meta.accentClass)}
                    >
                        {loading ? 'Adding…' : `Add ${meta.label} Connection`}
                    </Button>
                </div>
            </div>
        );
    };

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center gap-4">
                <Button variant="ghost" size="icon" onClick={() => navigate('/connections')} className="shrink-0">
                    <ArrowLeft className="h-5 w-5" />
                </Button>
                <div className="flex items-center gap-3 min-w-0">
                    <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 shrink-0">
                        {selectedProvider?.icon}
                    </div>
                    <div className="min-w-0">
                        <h1 className="text-2xl font-bold tracking-tight">Add Connection</h1>
                        <p className="text-sm text-muted-foreground truncate">
                            Connect a new AI provider account to your proxy.
                        </p>
                    </div>
                </div>
            </div>

            {/* Provider picker */}
            <div className="space-y-3 rounded-xl border bg-muted/20 p-3">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                        <p className="text-sm font-semibold">Choose provider</p>
                        <p className="text-xs text-muted-foreground">
                            Pick the account type first; the setup form below changes automatically.
                        </p>
                    </div>
                    <div className="relative w-full sm:w-72">
                        <Search
                            size={14}
                            className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none"
                        />
                        <Input
                            type="text"
                            name="provider-picker-filter"
                            value={providerSearch}
                            onChange={(e) => setProviderSearch(e.target.value)}
                            placeholder="Search providers or auth..."
                            className="h-8 pl-9 text-sm"
                            autoComplete="new-password"
                            data-1p-ignore
                        />
                    </div>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2">
                    {filteredProviderTabs.map((p) => {
                        const isSelected = provider === p.id;

                        return (
                            <button
                                type="button"
                                key={p.id}
                                onClick={(event) => {
                                    event.preventDefault();
                                    handleProviderSelect(p.id);
                                }}
                                className={cn(
                                    'group flex min-h-28 items-start gap-3 rounded-lg border p-3 text-left transition-all cursor-pointer',
                                    isSelected
                                        ? 'bg-background border-primary/40 text-foreground shadow-sm ring-1 ring-primary/10'
                                        : 'bg-card border-border text-muted-foreground hover:bg-background hover:text-foreground hover:border-primary/20',
                                )}
                            >
                                <div
                                    className={cn(
                                        'flex h-10 w-10 shrink-0 items-center justify-center rounded-lg transition-colors',
                                        isSelected ? 'bg-primary/10' : 'bg-muted/70 group-hover:bg-muted',
                                    )}
                                >
                                    {p.icon}
                                </div>
                                <div className="min-w-0 flex-1 space-y-1">
                                    <div className="flex items-center gap-2">
                                        <span className="truncate text-sm font-semibold">{p.name}</span>
                                        {p.recommended && (
                                            <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                                Fast
                                            </Badge>
                                        )}
                                    </div>
                                    <p className="text-xs leading-snug text-muted-foreground">{p.description}</p>
                                    <p className="flex items-center gap-1 text-[11px] text-muted-foreground">
                                        <KeyRound size={11} /> {p.auth}
                                    </p>
                                </div>
                            </button>
                        );
                    })}
                </div>
                {filteredProviderTabs.length === 0 && (
                    <div className="rounded-lg border border-dashed bg-background py-8 text-center text-sm text-muted-foreground">
                        No providers match "{providerSearch.trim()}".
                    </div>
                )}
                {selectedProviderHidden && selectedProvider && (
                    <div className="flex flex-col gap-2 rounded-lg border bg-background px-3 py-2 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
                        <span>
                            Current setup is still {selectedProvider.name}, but it is hidden by this search.
                        </span>
                        <Button
                            type="button"
                            variant="link"
                            className="h-auto justify-start p-0 text-xs"
                            onClick={() => setProviderSearch('')}
                        >
                            Show selected provider
                        </Button>
                    </div>
                )}
            </div>

            {/* Content */}
            <div className="rounded-xl border bg-card">
                {selectedProvider && (
                    <div className="flex flex-col gap-3 border-b bg-muted/20 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
                        <div className="flex min-w-0 items-center gap-3">
                            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background">
                                {selectedProvider.icon}
                            </div>
                            <div className="min-w-0">
                                <p className="truncate text-sm font-semibold">{selectedProvider.name} setup</p>
                                <p className="truncate text-xs text-muted-foreground">{selectedProvider.description}</p>
                            </div>
                        </div>
                        <Badge variant="outline" className="w-fit gap-1 text-xs">
                            <KeyRound size={12} /> {selectedProvider.auth}
                        </Badge>
                    </div>
                )}
                <div className="p-6">
                    {/* Kiro */}
                    {provider === 'kiro' && (
                        <div className="space-y-5">
                            {/* Mode selector */}
                            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2">
                                {kiroModes.map((m) => (
                                    <button
                                        key={m.id}
                                        onClick={() => {
                                            clearKiroPolling();
                                            setImportMode(m.id);
                                            setSocialLogin(null);
                                            setError('');
                                        }}
                                        className={cn(
                                            'flex min-h-20 flex-col items-center justify-center gap-1 rounded-lg border p-3 text-center text-xs font-medium transition-all cursor-pointer',
                                            importMode === m.id
                                                ? 'bg-primary/5 border-primary/30 text-primary ring-1 ring-primary/10'
                                                : 'bg-transparent border-transparent text-muted-foreground hover:bg-muted hover:border-border',
                                        )}
                                    >
                                        {m.icon}
                                        <span>{m.label}</span>
                                        <span className="text-[10px] font-normal text-muted-foreground">{m.desc}</span>
                                    </button>
                                ))}
                            </div>

                            <div className="border-t pt-5">
                                {importMode === 'detect' && (
                                    <div className="text-center space-y-4 max-w-sm mx-auto">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 mx-auto">
                                            <Search size={20} className="text-primary" />
                                        </div>
                                        <div>
                                            <p className="font-medium text-sm mb-1">Auto Detect & Import</p>
                                            <p className="text-xs text-muted-foreground">
                                                Scan for Kiro credentials from{' '}
                                                <code className="bg-muted px-1.5 py-0.5 rounded text-[11px]">
                                                    kiro-auth-token.json
                                                </code>{' '}
                                                on your machine.
                                            </p>
                                        </div>
                                        <Button onClick={handleDetect} disabled={loading} size="sm" className="gap-2">
                                            {loading ? (
                                                <Loader2 size={13} className="animate-spin" />
                                            ) : (
                                                <Search size={13} />
                                            )}
                                            {loading ? 'Detecting…' : 'Scan & Import'}
                                        </Button>
                                    </div>
                                )}

                                {importMode === 'builder-id' && (
                                    <div className="text-center space-y-4 max-w-sm mx-auto">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-orange-500/10 mx-auto">
                                            <ExternalLink size={20} className="text-orange-500" />
                                        </div>
                                        <div>
                                            <p className="font-medium text-sm mb-1">AWS Builder ID</p>
                                            <p className="text-xs text-muted-foreground">
                                                Authenticate via AWS Builder ID using Device Code Flow.
                                            </p>
                                        </div>
                                        {!deviceCode && (
                                            <Button
                                                onClick={handleStartBuilderID}
                                                disabled={loading}
                                                size="sm"
                                                className="gap-2"
                                            >
                                                {loading ? (
                                                    <Loader2 size={13} className="animate-spin" />
                                                ) : (
                                                    <ExternalLink size={13} />
                                                )}
                                                Start Login
                                            </Button>
                                        )}
                                        <DeviceCodePanel />
                                    </div>
                                )}

                                {importMode === 'idc' && (
                                    <div className="space-y-4 max-w-sm mx-auto">
                                        <div className="text-center">
                                            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-blue-500/10 mx-auto">
                                                <Shield size={20} className="text-blue-500" />
                                            </div>
                                            <p className="font-medium text-sm mt-3 mb-1">IAM Identity Center</p>
                                            <p className="text-xs text-muted-foreground">
                                                Enterprise SSO with custom start URL.
                                            </p>
                                        </div>
                                        {!deviceCode && (
                                            <div className="space-y-3">
                                                <Input
                                                    value={idcForm.startUrl}
                                                    onChange={(e) =>
                                                        setIdcForm({ ...idcForm, startUrl: e.target.value })
                                                    }
                                                    placeholder="Start URL (https://mycompany.awsapps.com/start)"
                                                    className="text-xs"
                                                />
                                                <Input
                                                    value={idcForm.region}
                                                    onChange={(e) => setIdcForm({ ...idcForm, region: e.target.value })}
                                                    placeholder="Region (e.g. us-east-1)"
                                                    className="text-xs"
                                                />
                                                <Button
                                                    onClick={handleStartIDC}
                                                    disabled={loading || !idcForm.startUrl}
                                                    size="sm"
                                                    className="w-full gap-2"
                                                >
                                                    {loading ? (
                                                        <Loader2 size={13} className="animate-spin" />
                                                    ) : (
                                                        <ExternalLink size={13} />
                                                    )}
                                                    Start Login
                                                </Button>
                                            </div>
                                        )}
                                        <DeviceCodePanel />
                                    </div>
                                )}

                                {importMode === 'social' && (
                                    <div className="space-y-4 max-w-sm mx-auto text-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-green-500/10 mx-auto">
                                            <Globe size={20} className="text-green-500" />
                                        </div>
                                        <div>
                                            <p className="font-medium text-sm mb-1">Social Login</p>
                                            <p className="text-xs text-muted-foreground">
                                                Authenticate with Google or GitHub via Kiro Identity.
                                            </p>
                                        </div>
                                        {!socialLogin && (
                                            <>
                                                <div className="flex gap-2 justify-center">
                                                    {(['google', 'github'] as const).map((p) => (
                                                        <button
                                                            key={p}
                                                            onClick={() => setSocialProvider(p)}
                                                            className={cn(
                                                                'flex items-center gap-2 px-4 py-2.5 rounded-lg text-xs font-medium cursor-pointer transition-all border',
                                                                socialProvider === p
                                                                    ? 'bg-primary/5 text-primary border-primary/30'
                                                                    : 'bg-transparent text-muted-foreground border-border hover:bg-muted',
                                                            )}
                                                        >
                                                            {p === 'google' ? (
                                                                <Globe size={14} />
                                                            ) : (
                                                                <GitBranch size={14} />
                                                            )}
                                                            {p === 'google' ? 'Google' : 'GitHub'}
                                                        </button>
                                                    ))}
                                                </div>
                                                <Button
                                                    onClick={handleStartSocial}
                                                    disabled={loading}
                                                    size="sm"
                                                    className="gap-2"
                                                >
                                                    {loading ? (
                                                        <Loader2 size={13} className="animate-spin" />
                                                    ) : (
                                                        <Globe size={13} />
                                                    )}
                                                    Start Login
                                                </Button>
                                            </>
                                        )}
                                        {socialLogin && (
                                            <div className="space-y-3 text-left rounded-lg border bg-muted/40 p-4">
                                                <div className="space-y-1.5 text-xs text-muted-foreground">
                                                    <p className="flex items-start gap-2">
                                                        <CheckCircle2
                                                            size={12}
                                                            className="text-green-500 mt-0.5 shrink-0"
                                                        />{' '}
                                                        Login page opened in browser.
                                                    </p>
                                                    <p className="flex items-start gap-2">
                                                        <CheckCircle2
                                                            size={12}
                                                            className="text-green-500 mt-0.5 shrink-0"
                                                        />{' '}
                                                        After login, copy the{' '}
                                                        <code className="bg-muted px-1 rounded text-[10px]">
                                                            kiro://
                                                        </code>{' '}
                                                        callback URL.
                                                    </p>
                                                    <p className="flex items-start gap-2">
                                                        <CheckCircle2
                                                            size={12}
                                                            className="text-green-500 mt-0.5 shrink-0"
                                                        />{' '}
                                                        Paste it below to complete.
                                                    </p>
                                                </div>
                                                <Input
                                                    value={socialCallbackUrl}
                                                    onChange={(e) => setSocialCallbackUrl(e.target.value)}
                                                    placeholder="kiro://kiro.kiroAgent/authenticate-success?..."
                                                    className="text-xs font-mono"
                                                />
                                                <div className="flex gap-2">
                                                    <Button
                                                        onClick={handleExchangeSocial}
                                                        disabled={loading || !socialCallbackUrl}
                                                        size="sm"
                                                        className="flex-1"
                                                    >
                                                        {loading ? 'Processing…' : 'Submit'}
                                                    </Button>
                                                    <Button asChild variant="outline" size="sm">
                                                        <a
                                                            href={socialLogin.loginUrl}
                                                            target="_blank"
                                                            rel="noopener noreferrer"
                                                        >
                                                            <ExternalLink size={13} />
                                                        </a>
                                                    </Button>
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                )}

                                {importMode === 'file' && (
                                    <div className="text-center space-y-4 max-w-sm mx-auto">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 mx-auto">
                                            <Upload size={20} className="text-primary" />
                                        </div>
                                        <div>
                                            <p className="font-medium text-sm mb-1">Import from File</p>
                                            <p className="text-xs text-muted-foreground">
                                                Upload your{' '}
                                                <code className="bg-muted px-1.5 py-0.5 rounded text-[11px]">
                                                    kiro-auth-token.json
                                                </code>{' '}
                                                file.
                                            </p>
                                        </div>
                                        <label className="flex flex-col items-center justify-center gap-3 p-8 rounded-xl border-2 border-dashed cursor-pointer hover:border-primary hover:bg-muted/40 transition-all">
                                            <Upload size={24} className="text-muted-foreground" />
                                            <div className="text-center">
                                                <p className="text-sm font-medium">
                                                    {loading ? 'Processing…' : 'Click to select JSON file'}
                                                </p>
                                                <p className="text-xs text-muted-foreground mt-0.5">
                                                    Only{' '}
                                                    <code className="bg-muted px-1 rounded">kiro-auth-token.json</code>{' '}
                                                    supported
                                                </p>
                                            </div>
                                            <input
                                                type="file"
                                                accept=".json"
                                                onChange={handleFileUpload}
                                                className="hidden"
                                                disabled={loading}
                                            />
                                        </label>
                                    </div>
                                )}

                                {importMode === 'manual' && (
                                    <div className="space-y-4 max-w-md mx-auto">
                                        <div className="text-center">
                                            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-amber-500/10 mx-auto">
                                                <Play size={20} className="text-amber-500" />
                                            </div>
                                            <p className="font-medium text-sm mt-3 mb-1">Manual Import</p>
                                            <p className="text-xs text-muted-foreground">
                                                Paste your refresh token and configuration manually.
                                            </p>
                                        </div>
                                        <div className="space-y-3">
                                            <Textarea
                                                value={form.refreshToken}
                                                onChange={(e) => setForm({ ...form, refreshToken: e.target.value })}
                                                placeholder="Refresh Token *"
                                                rows={3}
                                                className="text-xs font-mono"
                                            />
                                            <div className="grid grid-cols-2 gap-3">
                                                <select
                                                    value={form.authMethod}
                                                    onChange={(e) => setForm({ ...form, authMethod: e.target.value })}
                                                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-xs shadow-sm"
                                                >
                                                    <option value="builder-id">AWS Builder ID</option>
                                                    <option value="idc">IAM Identity Center</option>
                                                    <option value="social">Social Login</option>
                                                </select>
                                                <Input
                                                    value={form.region}
                                                    onChange={(e) => setForm({ ...form, region: e.target.value })}
                                                    placeholder="Region (optional)"
                                                    className="text-xs"
                                                />
                                            </div>
                                            <Button
                                                onClick={handleManualImport}
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
                    )}

                    {/* OpenAI */}
                    {provider === 'openai' && (
                        <div className="max-w-lg mx-auto space-y-5">
                            <div className="flex justify-center gap-1 p-1 bg-muted rounded-lg w-fit mx-auto">
                                {(['oauth', 'apikey'] as const).map((mode) => (
                                    <button
                                        key={mode}
                                        onClick={() => {
                                            clearOpenAIPolling();
                                            setOpenaiMode(mode);
                                            setError('');
                                        }}
                                        className={cn(
                                            'flex items-center gap-1.5 px-4 py-2 rounded-md text-xs font-medium cursor-pointer transition-all',
                                            openaiMode === mode
                                                ? 'bg-background text-foreground shadow-sm'
                                                : 'text-muted-foreground hover:text-foreground',
                                        )}
                                    >
                                        {mode === 'oauth' ? (
                                            <>
                                                <Globe size={13} /> OAuth (Recommended)
                                            </>
                                        ) : (
                                            <>
                                                <Shield size={13} /> API Key
                                            </>
                                        )}
                                    </button>
                                ))}
                            </div>

                            {openaiMode === 'oauth' && (
                                <div className="text-center space-y-4">
                                    {!openaiOAuthSession ? (
                                        <>
                                            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-emerald-500/10 mx-auto">
                                                <ExternalLink size={20} className="text-emerald-500" />
                                            </div>
                                            <div>
                                                <p className="font-medium text-sm mb-1">OpenAI OAuth</p>
                                                <p className="text-xs text-muted-foreground">
                                                    Securely connect via PKCE flow. No API key needed.
                                                </p>
                                            </div>
                                            <Button
                                                onClick={async () => {
                                                    setLoading(true);
                                                    setError('');
                                                    try {
                                                        const res = await api.startOpenAIOAuth();
                                                        setOpenaiOAuthSession({
                                                            sessionId: res.sessionId,
                                                            authUrl: res.authUrl,
                                                        });
                                                        window.open(res.authUrl, '_blank');
                                                        openaiPollRef.current = setInterval(async () => {
                                                            try {
                                                                const poll = await api.pollOpenAIOAuth(res.sessionId);
                                                                if (poll.status === 'pending') return;
                                                                if (openaiPollRef.current)
                                                                    clearInterval(openaiPollRef.current);
                                                                if (poll.status === 'success') {
                                                                    handleSuccess(
                                                                        `Connected! ${poll.email || poll.name || ''}`,
                                                                    );
                                                                } else {
                                                                    setError(poll.error || 'Authorization failed');
                                                                    setOpenaiOAuthSession(null);
                                                                }
                                                            } catch (e: any) {
                                                                if (openaiPollRef.current)
                                                                    clearInterval(openaiPollRef.current);
                                                                setError(e.message);
                                                                setOpenaiOAuthSession(null);
                                                            }
                                                        }, 2000);
                                                    } catch (e: any) {
                                                        setError(e.message);
                                                    } finally {
                                                        setLoading(false);
                                                    }
                                                }}
                                                disabled={loading}
                                                size="sm"
                                                className="gap-2 bg-emerald-600 hover:bg-emerald-700"
                                            >
                                                {loading ? (
                                                    <Loader2 size={13} className="animate-spin" />
                                                ) : (
                                                    <ExternalLink size={13} />
                                                )}
                                                Start Login
                                            </Button>
                                        </>
                                    ) : (
                                        <div className="space-y-4 text-left rounded-lg border bg-muted/40 p-5">
                                            <div className="text-center">
                                                <Loader2
                                                    size={24}
                                                    className="animate-spin text-emerald-600 mx-auto mb-2"
                                                />
                                                <p className="text-sm font-medium">Waiting for authorization…</p>
                                                <a
                                                    href={openaiOAuthSession.authUrl}
                                                    target="_blank"
                                                    rel="noopener noreferrer"
                                                    className="text-xs text-emerald-600 hover:underline inline-flex items-center gap-1 mt-1"
                                                >
                                                    Open browser manually <ExternalLink size={10} />
                                                </a>
                                            </div>
                                            <div>
                                                <p className="text-[10px] text-muted-foreground mb-1.5 flex items-center gap-1">
                                                    <AlertTriangle size={10} /> Paste callback URL if not redirected
                                                    automatically:
                                                </p>
                                                <div className="flex gap-2">
                                                    <Input
                                                        value={openaiManualCallback}
                                                        onChange={(e) => setOpenaiManualCallback(e.target.value)}
                                                        placeholder="http://localhost:1455/auth/callback?..."
                                                        className="text-xs font-mono"
                                                    />
                                                    <Button
                                                        onClick={async () => {
                                                            if (!openaiManualCallback.trim()) return;
                                                            setLoading(true);
                                                            setError('');
                                                            try {
                                                                if (openaiPollRef.current)
                                                                    clearInterval(openaiPollRef.current);
                                                                const poll = await api.pollOpenAIOAuth(
                                                                    openaiOAuthSession.sessionId,
                                                                    openaiManualCallback.trim(),
                                                                );
                                                                if (poll.status === 'success') {
                                                                    handleSuccess(
                                                                        `Connected! ${poll.email || poll.name || ''}`,
                                                                    );
                                                                } else {
                                                                    setError(poll.error || 'Authorization failed');
                                                                    setOpenaiOAuthSession(null);
                                                                }
                                                            } catch (e: any) {
                                                                setError(e.message);
                                                                setOpenaiOAuthSession(null);
                                                            } finally {
                                                                setLoading(false);
                                                            }
                                                        }}
                                                        disabled={loading || !openaiManualCallback.trim()}
                                                        size="sm"
                                                    >
                                                        {loading ? (
                                                            <Loader2 size={12} className="animate-spin" />
                                                        ) : (
                                                            'Submit'
                                                        )}
                                                    </Button>
                                                </div>
                                            </div>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                className="w-full"
                                                onClick={() => {
                                                    if (openaiPollRef.current) clearInterval(openaiPollRef.current);
                                                    setOpenaiOAuthSession(null);
                                                    setOpenaiManualCallback('');
                                                    setError('');
                                                }}
                                            >
                                                Cancel
                                            </Button>
                                        </div>
                                    )}
                                </div>
                            )}

                            {openaiMode === 'apikey' && selectedProviderMeta && (
                                <ApiKeyConnectionForm
                                    key="openai-apikey"
                                    provider={selectedProviderMeta}
                                    loading={loading}
                                    onSubmit={handleCreateConnection}
                                />
                            )}
                        </div>
                    )}

                    {provider === 'xai' && (
                        <div className="max-w-lg mx-auto space-y-5">
                            <div className="text-center">
                                <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-slate-500/10 mx-auto">
                                    <ProviderLogoIcon provider="xai" size={20} />
                                </div>
                                <p className="font-medium text-sm mt-3 mb-1">Grok Build</p>
                                <p className="text-xs text-muted-foreground">
                                    Connect your Grok Build account. Models route as{' '}
                                    <code className="bg-muted px-1.5 py-0.5 rounded text-[11px]">grok/&lt;model&gt;</code>.
                                </p>
                            </div>

                            {/* Mode selector */}
                            <div className="flex gap-1 p-1 bg-muted rounded-lg w-fit mx-auto">
                                {([['oauth', 'OAuth Flow'], ['file', 'Import File']] as const).map(([mode, label]) => (
                                    <button
                                        key={mode}
                                        onClick={() => { clearXAIAuth(); setXaiMode(mode); setError(''); }}
                                        className={cn(
                                            'flex items-center gap-1.5 px-4 py-2 rounded-md text-xs font-medium cursor-pointer transition-all',
                                            xaiMode === mode
                                                ? 'bg-background text-foreground shadow-sm'
                                                : 'text-muted-foreground hover:text-foreground',
                                        )}
                                    >
                                        {mode === 'oauth' ? <ExternalLink size={13} /> : <Upload size={13} />}
                                        {label}
                                    </button>
                                ))}
                            </div>

                            {xaiMode === 'oauth' && (
                                <>
                                    {!xaiOAuthSession ? (
                                        <div className="space-y-4 text-center">
                                            <Button
                                                onClick={handleStartXAIOAuth}
                                                disabled={loading}
                                                size="sm"
                                                className="gap-2 bg-slate-900 hover:bg-slate-800 text-white"
                                            >
                                                {loading ? (
                                                    <Loader2 size={13} className="animate-spin" />
                                                ) : (
                                                    <ExternalLink size={13} />
                                                )}
                                                {loading ? 'Starting…' : 'Connect Grok Build'}
                                            </Button>
                                            <p className="text-[10px] text-muted-foreground">
                                                Browser opens to xAI OAuth. Paste the callback URL here after authorization.
                                            </p>
                                        </div>
                                    ) : (
                                        <div className="space-y-4 rounded-lg border bg-muted/40 p-5">
                                            <div className="text-center space-y-1">
                                                <p className="text-sm font-medium">Finish Grok authorization</p>
                                                {xaiOAuthSession.redirectUri && (
                                                    <p className="text-[10px] text-muted-foreground break-all">
                                                        Expected redirect:{' '}
                                                        <code className="bg-muted px-1 rounded">{xaiOAuthSession.redirectUri}</code>
                                                    </p>
                                                )}
                                            </div>
                                            <Input
                                                value={xaiManualCallback}
                                                onChange={(e) => setXaiManualCallback(e.target.value)}
                                                placeholder="http://127.0.0.1:56121/callback?code=...&state=..."
                                                className="text-xs font-mono"
                                            />
                                            <div className="flex gap-2">
                                                <Button
                                                    onClick={handleExchangeXAIOAuth}
                                                    disabled={loading || !xaiManualCallback.trim()}
                                                    size="sm"
                                                    className="flex-1 bg-slate-900 hover:bg-slate-800 text-white"
                                                >
                                                    {loading ? 'Processing…' : 'Submit Callback'}
                                                </Button>
                                                <Button asChild variant="outline" size="sm">
                                                    <a href={xaiOAuthSession.authUrl} target="_blank" rel="noopener noreferrer">
                                                        <ExternalLink size={13} />
                                                    </a>
                                                </Button>
                                                <Button type="button" variant="outline" size="sm" onClick={clearXAIAuth}>
                                                    Cancel
                                                </Button>
                                            </div>
                                        </div>
                                    )}
                                </>
                            )}

                            {xaiMode === 'file' && (
                                <div className="text-center space-y-4">
                                    <p className="text-xs text-muted-foreground">
                                        Upload a Grok auth JSON file (e.g. exported from another CLI tool). Expected fields:{' '}
                                        <code className="bg-muted px-1 rounded text-[11px]">access_token</code>,{' '}
                                        <code className="bg-muted px-1 rounded text-[11px]">refresh_token</code>,{' '}
                                        <code className="bg-muted px-1 rounded text-[11px]">email</code>.
                                    </p>
                                    <label
                                        className="flex flex-col items-center justify-center gap-3 p-8 rounded-xl border-2 border-dashed cursor-pointer hover:border-primary hover:bg-muted/40 transition-all"
                                        onDragOver={(e) => { e.preventDefault(); e.currentTarget.classList.add('border-primary', 'bg-muted/40'); }}
                                        onDragLeave={(e) => { e.currentTarget.classList.remove('border-primary', 'bg-muted/40'); }}
                                        onDrop={(e) => {
                                            e.preventDefault();
                                            e.currentTarget.classList.remove('border-primary', 'bg-muted/40');
                                            const files = e.dataTransfer.files;
                                            if (files && files.length > 0) processXAIFiles(Array.from(files));
                                        }}
                                    >
                                        <Upload size={24} className="text-muted-foreground" />
                                        <div className="text-center">
                                            <p className="text-sm font-medium">
                                                {loading ? 'Importing…' : 'Drop file here or click to select'}
                                            </p>
                                            <p className="text-xs text-muted-foreground mt-0.5">
                                                Grok auth JSON format
                                            </p>
                                        </div>
                                        <input
                                            type="file"
                                            accept=".json"
                                            multiple
                                            onChange={handleXAIFileUpload}
                                            className="hidden"
                                            disabled={loading}
                                        />
                                    </label>
                                </div>
                            )}
                        </div>
                    )}

                    {/* Qwen */}
                    {provider === 'qwen' && (
                        <div className="max-w-lg mx-auto space-y-5">
                            <div className="text-center">
                                <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-[#6366F1]/10 mx-auto">
                                    <ProviderLogoIcon provider="qwen" size={20} />
                                </div>
                                <p className="font-medium text-sm mt-3 mb-1">Qwen AI</p>
                                <p className="text-xs text-muted-foreground">
                                    Connect to{' '}
                                    <a
                                        href="https://qwen.ai"
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="text-[#6366F1] hover:underline"
                                    >
                                        Qwen
                                    </a>{' '}
                                    — Free coding models via OAuth or paid via API key.
                                </p>
                            </div>

                            {/* Mode selector */}
                            <div className="flex gap-1 p-1 bg-muted rounded-lg w-fit mx-auto">
                                {(
                                    [
                                        ['oauth', 'OAuth (Free)'],
                                        ['apikey', 'API Key'],
                                    ] as const
                                ).map(([mode, label]) => (
                                    <button
                                        key={mode}
                                        onClick={() => {
                                            clearQwenPolling();
                                            setQwenMode(mode);
                                            setError('');
                                        }}
                                        className={cn(
                                            'flex flex-1 items-center justify-center text-xs py-2 px-4 rounded-md transition-all font-medium',
                                            qwenMode === mode
                                                ? 'bg-background text-foreground shadow-sm'
                                                : 'text-muted-foreground hover:text-foreground',
                                        )}
                                    >
                                        {mode === 'oauth' ? (
                                            <Globe size={13} className="mr-1.5" />
                                        ) : (
                                            <KeyRound size={13} className="mr-1.5" />
                                        )}
                                        {label}
                                    </button>
                                ))}
                            </div>

                            {qwenMode === 'oauth' && (
                                <div className="space-y-4 text-center">
                                    {!qwenDeviceCode ? (
                                        <>
                                            <Button
                                                onClick={handleStartQwenOAuth}
                                                disabled={loading}
                                                size="sm"
                                                className="bg-[#6366F1] hover:bg-[#5558E6]"
                                            >
                                                {loading ? (
                                                    <>
                                                        <Loader2 size={14} className="animate-spin mr-2" />
                                                        Starting…
                                                    </>
                                                ) : (
                                                    <>
                                                        <Play size={14} className="mr-2" />
                                                        Start Qwen Login
                                                    </>
                                                )}
                                            </Button>
                                            <p className="text-[10px] text-muted-foreground">
                                                Free tier: ~1,000–2,000 requests/day. No credit card needed.
                                            </p>
                                        </>
                                    ) : (
                                        <div className="space-y-3 rounded-lg border bg-muted/40 p-5">
                                            <div className="text-center">
                                                <p className="text-xs text-muted-foreground mb-2">
                                                    Enter this code on qwen.ai:
                                                </p>
                                                <div className="text-3xl font-mono font-bold tracking-[0.25em] text-[#6366F1] mb-4 select-all">
                                                    {qwenDeviceCode.userCode}
                                                </div>
                                                <Button
                                                    asChild
                                                    size="sm"
                                                    className="gap-2 bg-[#6366F1] hover:bg-[#5558E6]"
                                                >
                                                    <a
                                                        href={
                                                            qwenDeviceCode.verificationUriComplete ||
                                                            qwenDeviceCode.verificationUri
                                                        }
                                                        target="_blank"
                                                        rel="noopener noreferrer"
                                                    >
                                                        <ExternalLink size={14} /> Open Qwen Authorization
                                                    </a>
                                                </Button>
                                            </div>
                                            {qwenPolling && (
                                                <div className="flex items-center justify-center gap-2 text-xs text-muted-foreground mt-3 pt-3 border-t">
                                                    <Loader2 size={12} className="animate-spin" /> Waiting for
                                                    authorization ({qwenDeviceCode.interval}s)…
                                                </div>
                                            )}
                                        </div>
                                    )}
                                </div>
                            )}

                            {qwenMode === 'apikey' && selectedProviderMeta && (
                                <ApiKeyConnectionForm
                                    key="qwen-apikey"
                                    provider={selectedProviderMeta}
                                    loading={loading}
                                    onSubmit={handleCreateConnection}
                                />
                            )}
                        </div>
                    )}

                    {selectedProviderMeta &&
                        ['glm', 'minimax', 'anthropic', 'gemini', 'cline', 'openai-compatible'].includes(provider) && (
                            <div className="space-y-4">
                                <div className="text-center max-w-lg mx-auto">
                                    <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-muted mx-auto">
                                        <ProviderLogoIcon provider={provider} size={20} />
                                    </div>
                                    <p className="font-medium text-sm mt-3 mb-1">{selectedProviderMeta.name}</p>
                                    <p className="text-xs text-muted-foreground">{selectedProviderMeta.ui.description}</p>
                                </div>
                                <ApiKeyConnectionForm
                                    key={provider}
                                    provider={selectedProviderMeta}
                                    loading={loading}
                                    onSubmit={handleCreateConnection}
                                />
                            </div>
                        )}

                </div>

                {/* Error banner */}
                {error && (
                    <div className="border-t border-destructive/20">
                        <div className="flex items-center gap-3 px-6 py-3 bg-destructive/5 text-destructive text-sm">
                            <AlertTriangle size={16} className="shrink-0" />
                            {error}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
