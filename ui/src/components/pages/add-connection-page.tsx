import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { ArrowLeft, Link2, Loader2 } from 'lucide-react';
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
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';
import {
    canonicalProviderId,
    PROVIDER_META,
    usesGenericApiKeyForm,
} from '@/lib/provider-registry';
import { useProviderCatalog } from '@/lib/use-provider-catalog';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';

function isKnownAddable(id: string): boolean {
    const meta = PROVIDER_META[id];
    return Boolean(meta && meta.canAdd !== false);
}

export default function AddConnectionPage() {
    const navigate = useNavigate();
    const [searchParams, setSearchParams] = useSearchParams();
    const { providers, loading: catalogLoading } = useProviderCatalog();
    const rawProvider = searchParams.get('provider') ?? '';
    const providerId = rawProvider ? canonicalProviderId(rawProvider) : '';
    const selected = providers.find((provider) => provider.id === providerId);
    const showSetup =
        Boolean(selected) || Boolean(providerId && catalogLoading && isKnownAddable(providerId));

    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const [formBusy, setFormBusy] = useState(false);
    const errorBannerRef = useRef<HTMLDivElement>(null);
    const { markIntentionalNav } = useAddConnectionGuard(formBusy);

    useEffect(() => {
        if (!providerId || catalogLoading) return;
        if (!selected && !isKnownAddable(providerId)) {
            setSearchParams({}, { replace: true });
        }
    }, [catalogLoading, providerId, selected, setSearchParams]);

    useEffect(() => {
        if (!error) return;
        errorBannerRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        errorBannerRef.current?.focus();
    }, [error]);

    const handleSuccess = useCallback((message: string) => {
        markIntentionalNav();
        toast.success(message);
        navigate('/connections');
    }, [markIntentionalNav, navigate]);

    const handleCreateConnection = useCallback(async (payload: CreateConnectionPayload) => {
        setLoading(true);
        setError('');
        try {
            await api.createConnection(payload);
            handleSuccess(`${selected?.name ?? payload.provider} connected!`);
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
        setSearchParams({});
    };

    const handleBack = () => {
        if (showSetup) {
            goToPicker();
            return;
        }
        markIntentionalNav();
        navigate('/connections');
    };

    const title = showSetup ? (selected?.name ?? PROVIDER_META[providerId]?.label ?? 'Connection') : 'Add Connection';
    const subtitle = showSetup
        ? `Step 2 of 2 — choose how to connect ${title}`
        : 'Step 1 of 2 — pick a provider';

    return (
        <div className="space-y-6">
            <div className="flex items-center gap-4">
                <Button
                    variant="ghost"
                    size="icon"
                    onClick={handleBack}
                    className="shrink-0"
                    aria-label={showSetup ? 'Back to provider list' : 'Back to connections'}
                >
                    <ArrowLeft className="h-5 w-5" />
                </Button>
                <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                        {showSetup ? (
                            <ProviderLogoIcon provider={providerId} size={24} />
                        ) : (
                            <Link2 className="h-5 w-5 text-primary" />
                        )}
                    </div>
                    <div className="min-w-0">
                        <h1 className="text-2xl font-bold tracking-tight">{showSetup ? title : 'Add Connection'}</h1>
                        <p className="truncate text-sm text-muted-foreground">{subtitle}</p>
                    </div>
                </div>
            </div>

            {!showSetup && (
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

            {showSetup && !selected && (
                <div className="flex flex-col items-center justify-center rounded-xl border py-16 text-sm text-muted-foreground">
                    <Loader2 className="mb-3 h-5 w-5 animate-spin text-primary" />
                    Loading {title} setup…
                </div>
            )}

            {selected && (
                <SetupForm
                    provider={selected}
                    error={error}
                    errorRef={errorBannerRef}
                    loading={loading}
                    onBusyChange={setFormBusy}
                    onCreate={handleCreateConnection}
                    onError={setError}
                    onSuccess={handleSuccess}
                />
            )}
        </div>
    );
}

function SetupForm({
    provider,
    error,
    errorRef,
    loading,
    onBusyChange,
    onCreate,
    onError,
    onSuccess,
}: {
    provider: ProviderConfigMeta;
    error: string;
    errorRef: RefObject<HTMLDivElement | null>;
    loading: boolean;
    onBusyChange: (busy: boolean) => void;
    onCreate: (payload: CreateConnectionPayload) => Promise<void>;
    onError: (message: string) => void;
    onSuccess: (message: string) => void;
}) {
    return (
        <SetupCard
            name={provider.name}
            description={provider.ui.description}
            icon={<ProviderLogoIcon provider={provider.id} size={24} />}
            authLabel={provider.ui.authFlows.join(', ')}
            error={error}
            errorRef={errorRef}
        >
            {provider.id === 'kiro' && (
                <KiroConnectionForm onSuccess={onSuccess} onError={onError} onBusyChange={onBusyChange} />
            )}
            {provider.id === 'openai' && (
                <OpenAIConnectionForm
                    provider={provider}
                    loading={loading}
                    onCreate={onCreate}
                    onSuccess={onSuccess}
                    onError={onError}
                    onBusyChange={onBusyChange}
                />
            )}
            {provider.id === 'xai' && (
                <XaiConnectionForm onSuccess={onSuccess} onError={onError} onBusyChange={onBusyChange} />
            )}
            {provider.id === 'qwen' && (
                <QwenConnectionForm
                    provider={provider}
                    loading={loading}
                    onCreate={onCreate}
                    onSuccess={onSuccess}
                    onError={onError}
                    onBusyChange={onBusyChange}
                />
            )}
            {provider.id === 'commandcode' && (
                <CommandCodeConnectionForm
                    loading={loading}
                    onCreate={onCreate}
                    onSuccess={onSuccess}
                    onError={onError}
                    onBusyChange={onBusyChange}
                />
            )}
            {usesGenericApiKeyForm(provider) && (
                <div className="space-y-4">
                    <ApiKeyConnectionForm
                        key={provider.id}
                        provider={provider}
                        loading={loading}
                        onSubmit={onCreate}
                    />
                </div>
            )}
        </SetupCard>
    );
}
