import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
    ArrowLeft,
    AlertTriangle,
    CheckCircle2,
    ExternalLink,
    Globe,
    GitBranch,
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
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { getProviderMeta } from '@/lib/provider-registry';

export default function AddConnectionPage() {
    const navigate = useNavigate();
    const [provider, setProvider] = useState('kiro');
    const [importMode, setImportMode] = useState<ImportMode>('detect');
    const [form, setForm] = useState({
        refreshToken: '',
        clientId: '',
        clientSecret: '',
        region: '',
        authMethod: 'builder-id',
    });
    const [openaiForm, setOpenaiForm] = useState({
        name: '',
        apiKey: '',
        supportedModels: '',
    });
    const [customForm, setCustomForm] = useState({
        name: '',
        apiKey: '',
        baseUrl: '',
        supportedModels: '',
    });
    const [glmForm, setGlmForm] = useState({
        name: '',
        apiKey: '',
        baseUrl: '',
        supportedModels: '',
    });
    const [minimaxForm, setMinimaxForm] = useState({
        name: '',
        apiKey: '',
        baseUrl: '',
        supportedModels: '',
    });
    const [qwenMode, setQwenMode] = useState<'oauth' | 'apikey'>('oauth');
    const [qwenForm, setQwenForm] = useState({
        name: '',
        apiKey: '',
        baseUrl: '',
        supportedModels: '',
    });
    const [anthropicForm, setAnthropicForm] = useState({
        name: '',
        apiKey: '',
        baseUrl: '',
        supportedModels: '',
    });
    const [geminiForm, setGeminiForm] = useState({
        name: '',
        apiKey: '',
        baseUrl: '',
        supportedModels: '',
    });
    const [qwenDeviceCode, setQwenDeviceCode] = useState<DeviceCodeState | null>(null);
    const [qwenPolling, setQwenPolling] = useState(false);
    const qwenPollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null);
    const [polling, setPolling] = useState(false);
    const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const [idcForm, setIdcForm] = useState({ startUrl: '', region: '' });
    const [socialLogin, setSocialLogin] = useState<SocialLoginState | null>(null);
    const [socialCallbackUrl, setSocialCallbackUrl] = useState('');
    const [socialProvider, setSocialProvider] = useState<'google' | 'github'>('google');
    const [openaiMode, setOpenaiMode] = useState<'oauth' | 'apikey'>('oauth');
    const [openaiOAuthSession, setOpenaiOAuthSession] = useState<{
        sessionId: string;
        authUrl: string;
    } | null>(null);
    const [openaiManualCallback, setOpenaiManualCallback] = useState('');
    const openaiPollRef = useRef<ReturnType<typeof setInterval> | null>(null);

    useEffect(() => {
        return () => {
            if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
            if (openaiPollRef.current) clearInterval(openaiPollRef.current);
            if (qwenPollRef.current) clearTimeout(qwenPollRef.current);
        };
    }, []);

    const resetForm = () => {
        setForm({
            refreshToken: '',
            clientId: '',
            clientSecret: '',
            region: '',
            authMethod: 'builder-id',
        });
        setOpenaiForm({ name: '', apiKey: '', supportedModels: '' });
        setCustomForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' });
        setGlmForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' });
        setMinimaxForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' });
        setQwenForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' });
        setAnthropicForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' });
        setGeminiForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' });
        setQwenDeviceCode(null);
        setQwenPolling(false);
        if (qwenPollRef.current) {
            clearTimeout(qwenPollRef.current);
            qwenPollRef.current = null;
        }
        setIdcForm({ startUrl: '', region: '' });
        setDeviceCode(null);
        setPolling(false);
        setSocialLogin(null);
        setSocialCallbackUrl('');
        setOpenaiOAuthSession(null);
        setOpenaiManualCallback('');
        if (openaiPollRef.current) {
            clearInterval(openaiPollRef.current);
            openaiPollRef.current = null;
        }
        setError('');
        if (pollTimerRef.current) {
            clearTimeout(pollTimerRef.current);
            pollTimerRef.current = null;
        }
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
        const file = e.target.files?.[0];
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

    const handleAddOpenAI = async () => {
        setLoading(true);
        setError('');
        try {
            const models = parseSupportedModels(openaiForm.supportedModels);
            await api.addOpenAIConnection({
                name: openaiForm.name || undefined,
                apiKey: openaiForm.apiKey,
                supportedModels: models.length > 0 ? models : undefined,
            });
            handleSuccess('OpenAI added!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleAddCustom = async () => {
        setLoading(true);
        setError('');
        try {
            const models = parseSupportedModels(customForm.supportedModels);
            await api.addCustomConnection({
                name: customForm.name || undefined,
                apiKey: customForm.apiKey || undefined,
                baseUrl: customForm.baseUrl,
                supportedModels: models.length > 0 ? models : undefined,
            });
            handleSuccess('Custom added!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleAddGLM = async () => {
        setLoading(true);
        setError('');
        try {
            const models = parseSupportedModels(glmForm.supportedModels);
            await api.addGLMConnection({
                name: glmForm.name || undefined,
                apiKey: glmForm.apiKey,
                baseUrl: glmForm.baseUrl || undefined,
                supportedModels: models.length > 0 ? models : undefined,
            });
            handleSuccess('GLM connected!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleAddMiniMax = async () => {
        setLoading(true);
        setError('');
        try {
            const models = parseSupportedModels(minimaxForm.supportedModels);
            await api.addMiniMaxConnection({
                name: minimaxForm.name || undefined,
                apiKey: minimaxForm.apiKey,
                baseUrl: minimaxForm.baseUrl || undefined,
                supportedModels: models.length > 0 ? models : undefined,
            });
            handleSuccess('MiniMax connected!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
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

    const handleAddQwenAPIKey = async () => {
        setLoading(true);
        setError('');
        try {
            const models = parseSupportedModels(qwenForm.supportedModels);
            await api.addQwenConnection({
                name: qwenForm.name || undefined,
                apiKey: qwenForm.apiKey,
                baseUrl: qwenForm.baseUrl || undefined,
                supportedModels: models.length > 0 ? models : undefined,
            });
            handleSuccess('Qwen connected!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleAddAnthropic = async () => {
        setLoading(true);
        setError('');
        try {
            const models = parseSupportedModels(anthropicForm.supportedModels);
            await api.addAnthropicConnection({
                name: anthropicForm.name || undefined,
                apiKey: anthropicForm.apiKey,
                baseUrl: anthropicForm.baseUrl || undefined,
                supportedModels: models.length > 0 ? models : undefined,
            });
            handleSuccess('Anthropic connected!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

    const handleAddGemini = async () => {
        setLoading(true);
        setError('');
        try {
            const models = parseSupportedModels(geminiForm.supportedModels);
            await api.addGeminiConnection({
                name: geminiForm.name || undefined,
                apiKey: geminiForm.apiKey,
                baseUrl: geminiForm.baseUrl || undefined,
                supportedModels: models.length > 0 ? models : undefined,
            });
            handleSuccess('Gemini connected!');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    };

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

    const providerTabs = [
        {
            id: 'kiro',
            name: 'AWS / Kiro',
            icon: <ProviderLogoIcon provider="kiro" size={24} />,
            description: 'Amazon CodeWhisperer / Kiro',
        },
        {
            id: 'openai',
            name: 'OpenAI',
            icon: <ProviderLogoIcon provider="openai" size={24} />,
            description: 'ChatGPT, GPT-4, o-series',
        },
        {
            id: 'qwen',
            name: 'Qwen',
            icon: <ProviderLogoIcon provider="qwen" size={24} />,
            description: 'Alibaba Qwen models',
        },
        {
            id: 'glm',
            name: 'GLM',
            icon: <ProviderLogoIcon provider="glm" size={24} />,
            description: 'Zhipu AI / GLM',
        },
        {
            id: 'minimax',
            name: 'MiniMax',
            icon: <ProviderLogoIcon provider="minimax" size={24} />,
            description: 'MiniMax M2 series',
        },
        {
            id: 'anthropic',
            name: 'Anthropic',
            icon: <ProviderLogoIcon provider="anthropic" size={24} />,
            description: 'Claude via Anthropic API',
        },
        {
            id: 'gemini',
            name: 'Gemini',
            icon: <ProviderLogoIcon provider="gemini" size={24} />,
            description: 'Google Gemini models',
        },
        {
            id: 'openai-compatible',
            name: 'Custom',
            icon: <ProviderLogoIcon provider="openai-compatible" size={24} />,
            description: 'OpenAI-compatible API',
        },
    ];

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

    const selectedProvider = providerTabs.find((p) => p.id === provider);

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
        providerId: 'anthropic' | 'gemini';
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

            {/* Provider tabs */}
            <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-2">
                {providerTabs.map((p) => (
                    <button
                        key={p.id}
                        onClick={() => {
                            setProvider(p.id);
                            resetForm();
                        }}
                        className={cn(
                            'flex flex-col items-center gap-1.5 p-4 rounded-xl text-sm font-medium transition-all cursor-pointer border',
                            provider === p.id
                                ? 'bg-primary/5 border-primary/30 text-foreground shadow-sm ring-1 ring-primary/10'
                                : 'bg-card border-border text-muted-foreground hover:bg-muted hover:text-foreground hover:border-primary/20',
                        )}
                    >
                        <div
                            className={cn(
                                'flex h-9 w-9 items-center justify-center rounded-lg transition-colors',
                                provider === p.id ? 'bg-primary/10' : 'bg-muted/60',
                            )}
                        >
                            {p.icon}
                        </div>
                        <span className="text-xs font-semibold">{p.name}</span>
                        <span className="text-[10px] text-muted-foreground text-center leading-tight">
                            {p.description}
                        </span>
                    </button>
                ))}
            </div>

            {/* Content */}
            <div className="rounded-xl border bg-card">
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
                                            setImportMode(m.id);
                                            setDeviceCode(null);
                                            setPolling(false);
                                            setSocialLogin(null);
                                            setError('');
                                        }}
                                        className={cn(
                                            'flex flex-col items-center gap-1 p-3 rounded-lg text-xs font-medium transition-all cursor-pointer border',
                                            importMode === m.id
                                                ? 'bg-primary/5 border-primary/30 text-primary ring-1 ring-primary/10'
                                                : 'bg-transparent border-transparent text-muted-foreground hover:bg-muted hover:border-border',
                                        )}
                                    >
                                        {m.icon}
                                        <span>{m.label}</span>
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
                                            setOpenaiMode(mode);
                                            setOpenaiOAuthSession(null);
                                            setOpenaiManualCallback('');
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

                            {openaiMode === 'apikey' && (
                                <div className="space-y-4">
                                    <div className="text-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-emerald-500/10 mx-auto">
                                            <Shield size={20} className="text-emerald-500" />
                                        </div>
                                        <p className="font-medium text-sm mt-3 mb-1">API Key</p>
                                        <p className="text-xs text-muted-foreground">
                                            Use your OpenAI API key directly.
                                        </p>
                                    </div>
                                    <div className="space-y-3">
                                        <Input
                                            type="password"
                                            value={openaiForm.apiKey}
                                            onChange={(e) => setOpenaiForm({ ...openaiForm, apiKey: e.target.value })}
                                            placeholder="API Key (sk-proj-…) *"
                                            className="text-xs font-mono"
                                        />
                                        <Input
                                            value={openaiForm.name}
                                            onChange={(e) => setOpenaiForm({ ...openaiForm, name: e.target.value })}
                                            placeholder="Display Name (optional)"
                                            className="text-xs"
                                        />
                                        <Textarea
                                            value={openaiForm.supportedModels}
                                            onChange={(e) =>
                                                setOpenaiForm({
                                                    ...openaiForm,
                                                    supportedModels: e.target.value,
                                                })
                                            }
                                            placeholder="Supported Models (one per line, optional)"
                                            rows={3}
                                            className="text-xs font-mono"
                                        />
                                        <Button
                                            onClick={handleAddOpenAI}
                                            disabled={loading || !openaiForm.apiKey}
                                            size="sm"
                                            className="w-full bg-emerald-600 hover:bg-emerald-700"
                                        >
                                            {loading ? 'Adding…' : 'Add Connection'}
                                        </Button>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}

                    {/* Custom API */}
                    {provider === 'openai-compatible' && (
                        <div className="max-w-lg mx-auto space-y-5">
                            <div className="text-center">
                                <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-purple-500/10 mx-auto">
                                    <ProviderLogoIcon provider="openai-compatible" size={20} />
                                </div>
                                <p className="font-medium text-sm mt-3 mb-1">OpenAI-Compatible API</p>
                                <p className="text-xs text-muted-foreground">
                                    Connect to any OpenAI-compatible endpoint (e.g. Together, Anyscale, local LLMs).
                                </p>
                            </div>
                            <div className="space-y-3">
                                <Input
                                    value={customForm.baseUrl}
                                    onChange={(e) => setCustomForm({ ...customForm, baseUrl: e.target.value })}
                                    placeholder="Base URL (e.g. https://api.together.xyz/v1) *"
                                    className="text-xs font-mono"
                                />
                                <div className="grid grid-cols-2 gap-3">
                                    <Input
                                        type="password"
                                        value={customForm.apiKey}
                                        onChange={(e) => setCustomForm({ ...customForm, apiKey: e.target.value })}
                                        placeholder="API Key (optional)"
                                        className="text-xs font-mono"
                                    />
                                    <Input
                                        value={customForm.name}
                                        onChange={(e) => setCustomForm({ ...customForm, name: e.target.value })}
                                        placeholder="Display Name"
                                        className="text-xs"
                                    />
                                </div>
                                <Textarea
                                    value={customForm.supportedModels}
                                    onChange={(e) =>
                                        setCustomForm({
                                            ...customForm,
                                            supportedModels: e.target.value,
                                        })
                                    }
                                    placeholder="Supported Models (one per line, optional)"
                                    rows={3}
                                    className="text-xs font-mono"
                                />
                                <Button
                                    onClick={handleAddCustom}
                                    disabled={loading || !customForm.baseUrl}
                                    size="sm"
                                    className="w-full bg-purple-600 hover:bg-purple-700"
                                >
                                    {loading ? 'Adding…' : 'Add Custom Connection'}
                                </Button>
                            </div>
                        </div>
                    )}

                    {/* GLM (Zhipu AI) */}
                    {provider === 'glm' && (
                        <div className="max-w-lg mx-auto space-y-5">
                            <div className="text-center">
                                <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-[#0066FF]/10 mx-auto">
                                    <ProviderLogoIcon provider="glm" size={20} />
                                </div>
                                <p className="font-medium text-sm mt-3 mb-1">Zhipu AI (GLM)</p>
                                <p className="text-xs text-muted-foreground">
                                    Connect to{' '}
                                    <a
                                        href="https://open.bigmodel.cn"
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="text-[#0066FF] hover:underline"
                                    >
                                        bigmodel.cn
                                    </a>{' '}
                                    — GLM-4, GLM-4-Flash, and more.
                                </p>
                            </div>
                            <div className="space-y-3">
                                <Input
                                    type="password"
                                    value={glmForm.apiKey}
                                    onChange={(e) => setGlmForm({ ...glmForm, apiKey: e.target.value })}
                                    placeholder="API Key *"
                                    className="text-xs font-mono"
                                />
                                <div className="grid grid-cols-2 gap-3">
                                    <Input
                                        value={glmForm.name}
                                        onChange={(e) => setGlmForm({ ...glmForm, name: e.target.value })}
                                        placeholder="Display Name (optional)"
                                        className="text-xs"
                                    />
                                    <Input
                                        value={glmForm.baseUrl}
                                        onChange={(e) => setGlmForm({ ...glmForm, baseUrl: e.target.value })}
                                        placeholder="Base URL (default: bigmodel.cn)"
                                        className="text-xs font-mono"
                                    />
                                </div>
                                <Textarea
                                    value={glmForm.supportedModels}
                                    onChange={(e) => setGlmForm({ ...glmForm, supportedModels: e.target.value })}
                                    placeholder="Supported Models (one per line, auto-populated if empty)"
                                    rows={3}
                                    className="text-xs font-mono"
                                />
                                <Button
                                    onClick={handleAddGLM}
                                    disabled={loading || !glmForm.apiKey}
                                    size="sm"
                                    className="w-full bg-[#0066FF] hover:bg-[#0055DD]"
                                >
                                    {loading ? 'Adding…' : 'Add GLM Connection'}
                                </Button>
                            </div>
                        </div>
                    )}

                    {/* MiniMax */}
                    {provider === 'minimax' && (
                        <div className="max-w-lg mx-auto space-y-5">
                            <div className="text-center">
                                <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-[#FF6B35]/10 mx-auto">
                                    <ProviderLogoIcon provider="minimax" size={20} />
                                </div>
                                <p className="font-medium text-sm mt-3 mb-1">MiniMax</p>
                                <p className="text-xs text-muted-foreground">
                                    Connect to{' '}
                                    <a
                                        href="https://platform.minimax.io"
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="text-[#FF6B35] hover:underline"
                                    >
                                        MiniMax
                                    </a>{' '}
                                    — M2 series models.
                                </p>
                            </div>
                            <div className="space-y-3">
                                <Input
                                    type="password"
                                    value={minimaxForm.apiKey}
                                    onChange={(e) => setMinimaxForm({ ...minimaxForm, apiKey: e.target.value })}
                                    placeholder="API Key *"
                                    className="text-xs font-mono"
                                />
                                <div className="grid grid-cols-2 gap-3">
                                    <Input
                                        value={minimaxForm.name}
                                        onChange={(e) => setMinimaxForm({ ...minimaxForm, name: e.target.value })}
                                        placeholder="Display Name (optional)"
                                        className="text-xs"
                                    />
                                    <Input
                                        value={minimaxForm.baseUrl}
                                        onChange={(e) =>
                                            setMinimaxForm({
                                                ...minimaxForm,
                                                baseUrl: e.target.value,
                                            })
                                        }
                                        placeholder="Base URL (default: api.minimax.io)"
                                        className="text-xs font-mono"
                                    />
                                </div>
                                <Textarea
                                    value={minimaxForm.supportedModels}
                                    onChange={(e) =>
                                        setMinimaxForm({
                                            ...minimaxForm,
                                            supportedModels: e.target.value,
                                        })
                                    }
                                    placeholder="Supported Models (one per line, auto-populated if empty)"
                                    rows={3}
                                    className="text-xs font-mono"
                                />
                                <Button
                                    onClick={handleAddMiniMax}
                                    disabled={loading || !minimaxForm.apiKey}
                                    size="sm"
                                    className="w-full bg-[#FF6B35] hover:bg-[#E85A25]"
                                >
                                    {loading ? 'Adding…' : 'Add MiniMax Connection'}
                                </Button>
                            </div>
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
                                        ['oauth', '🔓 OAuth (Free)'],
                                        ['apikey', '🔑 API Key'],
                                    ] as const
                                ).map(([mode, label]) => (
                                    <button
                                        key={mode}
                                        onClick={() => setQwenMode(mode)}
                                        className={cn(
                                            'flex-1 text-xs py-2 px-4 rounded-md transition-all font-medium',
                                            qwenMode === mode
                                                ? 'bg-background text-foreground shadow-sm'
                                                : 'text-muted-foreground hover:text-foreground',
                                        )}
                                    >
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

                            {qwenMode === 'apikey' && (
                                <div className="space-y-4">
                                    <p className="text-[10px] text-muted-foreground text-center">
                                        Get API key from{' '}
                                        <a
                                            href="https://dashscope.aliyun.com"
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            className="text-[#6366F1] hover:underline"
                                        >
                                            DashScope Console
                                        </a>
                                        .
                                    </p>
                                    <div className="space-y-3">
                                        <Input
                                            type="password"
                                            value={qwenForm.apiKey}
                                            onChange={(e) => setQwenForm({ ...qwenForm, apiKey: e.target.value })}
                                            placeholder="API Key *"
                                            className="text-xs font-mono"
                                        />
                                        <div className="grid grid-cols-2 gap-3">
                                            <Input
                                                value={qwenForm.name}
                                                onChange={(e) => setQwenForm({ ...qwenForm, name: e.target.value })}
                                                placeholder="Display Name (optional)"
                                                className="text-xs"
                                            />
                                            <Input
                                                value={qwenForm.baseUrl}
                                                onChange={(e) => setQwenForm({ ...qwenForm, baseUrl: e.target.value })}
                                                placeholder="Base URL (default: DashScope)"
                                                className="text-xs font-mono"
                                            />
                                        </div>
                                        <Textarea
                                            value={qwenForm.supportedModels}
                                            onChange={(e) =>
                                                setQwenForm({
                                                    ...qwenForm,
                                                    supportedModels: e.target.value,
                                                })
                                            }
                                            placeholder="Supported Models (one per line, auto-populated if empty)"
                                            rows={3}
                                            className="text-xs font-mono"
                                        />
                                        <Button
                                            onClick={handleAddQwenAPIKey}
                                            disabled={loading || !qwenForm.apiKey}
                                            size="sm"
                                            className="w-full bg-[#6366F1] hover:bg-[#5558E6]"
                                        >
                                            {loading ? 'Adding…' : 'Add Qwen Connection'}
                                        </Button>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}

                    {provider === 'anthropic' &&
                        renderApiKeyProviderForm({
                            providerId: 'anthropic',
                            title: 'Anthropic',
                            description: 'Connect Claude models with an Anthropic API key from',
                            form: anthropicForm,
                            setForm: setAnthropicForm,
                            onSubmit: handleAddAnthropic,
                            docsUrl: 'https://console.anthropic.com/settings/keys',
                            docsLabel: 'Anthropic Console',
                        })}

                    {provider === 'gemini' &&
                        renderApiKeyProviderForm({
                            providerId: 'gemini',
                            title: 'Gemini',
                            description: 'Connect Google Gemini models with an API key from',
                            form: geminiForm,
                            setForm: setGeminiForm,
                            onSubmit: handleAddGemini,
                            docsUrl: 'https://aistudio.google.com/app/apikey',
                            docsLabel: 'Google AI Studio',
                        })}
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
