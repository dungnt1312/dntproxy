import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { AlertTriangle, ArrowLeft, Link2, Loader2 } from 'lucide-react';
import { api } from '../../api';
import { ProviderLogoIcon } from '../connections/helpers';
import { ApiKeyConnectionForm } from '../connections/ApiKeyConnectionForm';
import { CommandCodeConnectionForm } from '../connections/CommandCodeConnectionForm';
import { KiroConnectionForm } from '../connections/add/kiro-connection-form';
import { OpenAIConnectionForm } from '../connections/add/openai-connection-form';
import { ProviderPicker } from '../connections/add/provider-picker';
import { QwenConnectionForm } from '../connections/add/qwen-connection-form';
import { SetupCard } from '../connections/add/setup-card';
import { confirmLeaveSetup, errorMessage } from '../connections/add/helpers';
import { useAddConnectionGuard } from '../connections/add/use-add-connection-guard';
import { XaiConnectionForm } from '../connections/add/xai-connection-form';
import { MethodPicker } from '../connections/add/method-picker';
import { ConnectionResultPanel } from '../connections/add/connection-result-panel';
import { Button } from '@/components/ui/button';
import {
    canonicalProviderId,
    PROVIDER_META,
    usesGenericApiKeyForm,
} from '@/lib/provider-registry';
import { useProviderCatalog } from '@/lib/use-provider-catalog';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';

type CreateResult = { id?: string; name?: string; routePrefix?: string };

function isKnownAddable(id: string): boolean {
    const meta = PROVIDER_META[id];
    return Boolean(meta && meta.canAdd !== false);
}

export default function AddConnectionPage() {
    const navigate = useNavigate();
    const [searchParams, setSearchParams] = useSearchParams();
    const { providers, loading: catalogLoading, error: catalogError, reload } = useProviderCatalog();
    const rawProvider = searchParams.get('provider') ?? '';
    const providerId = rawProvider ? canonicalProviderId(rawProvider) : '';
    const selected = providers.find((provider) => provider.id === providerId);
    const method = searchParams.get('method') ?? '';
    const [result, setResult] = useState<CreateResult | null>(null);
    const showSetup = Boolean(selected && !result && (method || selected.ui.authFlows.length <= 1));
    const showMethodPicker = Boolean(selected && !showSetup && !result);

    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const [formBusy, setFormBusy] = useState(false);
    const errorBannerRef = useRef<HTMLDivElement>(null);
    const { markIntentionalNav } = useAddConnectionGuard(formBusy);

    useEffect(() => {
        if (!providerId || catalogLoading) return;
        if (!selected && !isKnownAddable(providerId)) setSearchParams({}, { replace: true });
    }, [catalogLoading, providerId, selected, setSearchParams]);

    useEffect(() => {
        if (!error) return;
        errorBannerRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        errorBannerRef.current?.focus();
    }, [error]);

    const handleSuccess = useCallback((message: string, createResult?: CreateResult) => {
        setError('');
        setResult(createResult ?? {});
    }, []);

    const handleCreateConnection = useCallback(async (payload: CreateConnectionPayload) => {
        setLoading(true);
        setError('');
        try {
            const response = await api.createConnection(payload) as CreateResult;
            handleSuccess(`${selected?.name ?? payload.provider} connected!`, response);
        } catch (err: unknown) {
            setError(errorMessage(err, 'Failed to add connection'));
        } finally {
            setLoading(false);
        }
    }, [handleSuccess, selected?.name]);

    const goToPicker = () => {
        if (formBusy && !confirmLeaveSetup()) return;
        setFormBusy(false);
        setError('');
        setResult(null);
        setSearchParams({});
    };

    const handleBack = () => {
        if (result) {
            setResult(null);
            return;
        }
        if (showSetup) {
            if (formBusy && !confirmLeaveSetup()) return;
            setFormBusy(false);
            setError('');
            setSearchParams({ provider: providerId });
            return;
        }
        if (showMethodPicker) {
            goToPicker();
            return;
        }
        markIntentionalNav();
        navigate('/connections');
    };

    const title = selected?.name ?? 'Add Connection';
    const subtitle = result
        ? 'Step 3 of 3 — verify your connection'
        : showSetup
            ? 'Step 2 of 3 — configure your connection'
            : showMethodPicker
                ? 'Step 2 of 3 — choose how to connect'
                : 'Step 1 of 3 — pick a provider';

    return (
        <div className="space-y-6">
            <div className="flex items-center gap-4">
                <Button variant="ghost" size="icon" onClick={handleBack} className="shrink-0" aria-label="Go back">
                    <ArrowLeft className="h-5 w-5" />
                </Button>
                <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                        {selected ? <ProviderLogoIcon provider={providerId} size={24} /> : <Link2 className="h-5 w-5 text-primary" />}
                    </div>
                    <div className="min-w-0">
                        <h1 className="text-2xl font-bold tracking-tight">{selected ? title : 'Add Connection'}</h1>
                        <p className="truncate text-sm text-muted-foreground">{subtitle}</p>
                    </div>
                </div>
            </div>

            {!selected && catalogError && (
                <div role="status" className="flex flex-wrap items-center gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 p-3 text-sm text-muted-foreground">
                    <AlertTriangle className="h-4 w-4 text-amber-500" />
                    <span className="flex-1">Couldn&apos;t refresh the provider catalog. Showing saved providers.</span>
                    <Button size="sm" variant="outline" onClick={() => void reload()}>Retry</Button>
                </div>
            )}

            {!selected && (
                <ProviderPicker
                    providers={providers.map((provider) => ({
                        id: provider.id,
                        name: provider.name,
                        description: provider.ui.description,
                        auth: provider.ui.authFlows.join(', '),
                        recommended: provider.id === 'kiro' || provider.id === 'openai',
                        icon: <ProviderLogoIcon provider={provider.id} size={24} />,
                    }))}
                    onSelect={(id) => {
                        setError('');
                        setSearchParams({ provider: id });
                    }}
                />
            )}

            {providerId && !selected && catalogLoading && (
                <div className="flex flex-col items-center justify-center rounded-xl border py-16 text-sm text-muted-foreground">
                    <Loader2 className="mb-3 h-5 w-5 animate-spin text-primary" />
                    Loading {PROVIDER_META[providerId]?.label ?? 'provider'} setup…
                </div>
            )}

            {showMethodPicker && selected && (
                <MethodPicker provider={selected} onSelect={(next) => setSearchParams({ provider: providerId, method: next })} />
            )}

            {showSetup && selected && (
                <SetupForm
                    provider={selected}
                    method={method || selected.ui.authFlows[0]}
                    error={error}
                    errorRef={errorBannerRef}
                    loading={loading}
                    onBusyChange={setFormBusy}
                    onCreate={handleCreateConnection}
                    onError={setError}
                    onSuccess={handleSuccess}
                    onMethodChange={(next) => setSearchParams({ provider: providerId, method: next })}
                />
            )}

            {result && selected && (
                <ConnectionResultPanel
                    providerName={selected.name}
                    result={result}
                    onAddAnother={goToPicker}
                    onDone={() => {
                        markIntentionalNav();
                        navigate('/connections');
                    }}
                />
            )}
        </div>
    );
}

function SetupForm({ provider, method, error, errorRef, loading, onBusyChange, onCreate, onError, onSuccess, onMethodChange }: {
    provider: ProviderConfigMeta;
    method: string;
    error: string;
    errorRef: RefObject<HTMLDivElement | null>;
    loading: boolean;
    onBusyChange: (busy: boolean) => void;
    onCreate: (payload: CreateConnectionPayload) => Promise<void>;
    onError: (message: string) => void;
    onSuccess: (message: string, result?: CreateResult) => void;
    onMethodChange: (method: string) => void;
}) {
    return (
        <SetupCard name={provider.name} description={provider.ui.description} icon={<ProviderLogoIcon provider={provider.id} size={24} />} authLabel={method} error={error} errorRef={errorRef}>
            {provider.id === 'kiro' && <KiroConnectionForm initialMethod={method} onMethodChange={onMethodChange} onSuccess={onSuccess} onError={onError} onBusyChange={onBusyChange} />}
            {provider.id === 'openai' && <OpenAIConnectionForm provider={provider} mode={method} loading={loading} onCreate={onCreate} onSuccess={onSuccess} onError={onError} onBusyChange={onBusyChange} />}
            {provider.id === 'xai' && <XaiConnectionForm mode={method} onSuccess={onSuccess} onError={onError} onBusyChange={onBusyChange} />}
            {provider.id === 'qwen' && <QwenConnectionForm provider={provider} mode={method} loading={loading} onCreate={onCreate} onSuccess={onSuccess} onError={onError} onBusyChange={onBusyChange} />}
            {provider.id === 'commandcode' && <CommandCodeConnectionForm loading={loading} onCreate={onCreate} onSuccess={onSuccess} onError={onError} onBusyChange={onBusyChange} />}
            {usesGenericApiKeyForm(provider) && <ApiKeyConnectionForm key={provider.id} provider={provider} loading={loading} onSubmit={onCreate} />}
        </SetupCard>
    );
}
