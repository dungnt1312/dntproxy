import { useCallback, useEffect, useState } from 'react';
import { Loader2, Play } from 'lucide-react';
import { api } from '../../../api';
import { DeviceCodePanel } from '../DeviceCodePanel';
import { ProviderLogoIcon } from '../helpers';
import { ApiKeyConnectionForm } from '../ApiKeyConnectionForm';
import { Button } from '@/components/ui/button';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';
import { errorMessage, pickResultId, type OnSuccess } from './helpers';
import { SetupIntro } from './setup-intro';
import { useDeviceCodePoll } from './use-device-code-poll';
import type { DeviceCodeState } from '../helpers';

type Props = {
    provider: ProviderConfigMeta;
    mode: string;
    loading: boolean;
    onCreate: (payload: CreateConnectionPayload) => Promise<void>;
    onSuccess: OnSuccess;
    onError: (message: string) => void;
    onBusyChange: (busy: boolean) => void;
};

export function QwenConnectionForm({ provider, mode, loading, onCreate, onSuccess, onError, onBusyChange }: Props) {
    const [starting, setStarting] = useState(false);
    const device = useDeviceCodePoll({
        poll: (sessionId) => api.pollQwenOAuth(sessionId),
        onSuccess: (res) => onSuccess(`Qwen connected${res.email ? ` (${res.email})` : ''}`, pickResultId(res)),
        onError,
    });

    useEffect(() => {
        onBusyChange(starting || device.polling || !!device.deviceCode || device.expired);
        return () => onBusyChange(false);
    }, [starting, device.polling, device.deviceCode, device.expired, onBusyChange]);

    const { start, cancel, ticket } = device;
    const startLoginFlow = useCallback(() => {
        cancel();
        const t = ticket();
        setStarting(true);
        onError('');
        void api
            .startQwenOAuth()
            .then((res: DeviceCodeState) => start(res, t))
            .catch((error: unknown) => onError(errorMessage(error, 'Failed to start Qwen login')))
            .finally(() => setStarting(false));
    }, [cancel, onError, start, ticket]);

    return (
        <div className="space-y-5">
            <SetupIntro
                icon={<ProviderLogoIcon provider="qwen" size={20} />}
                title="Qwen AI"
                description={
                    <>
                        Connect to{' '}
                        <a href="https://qwen.ai" target="_blank" rel="noopener noreferrer" className="text-primary underline-offset-2 hover:underline">
                            Qwen
                        </a>{' '}
                        — free coding models via OAuth, or the paid tier via API key.
                    </>
                }
            />
            {mode === 'oauth' && !device.deviceCode && !device.expired && (
                <Button onClick={startLoginFlow} disabled={starting} className="gap-2">
                    {starting ? <Loader2 className="size-4 animate-spin" /> : <Play className="size-4" />}
                    {starting ? 'Starting…' : 'Start Qwen login'}
                </Button>
            )}

            {mode === 'oauth' && (device.deviceCode || device.expired) && (
                <DeviceCodePanel
                    prompt="Enter this code on qwen.ai:"
                    userCode={device.deviceCode?.userCode ?? '—'}
                    verificationUri={device.deviceCode?.verificationUri}
                    verificationUriComplete={device.deviceCode?.verificationUriComplete}
                    expiresAtMs={device.expiresAt ?? undefined}
                    expired={device.expired}
                    waiting={device.polling}
                    onCancel={() => {
                        cancel();
                        onError('');
                    }}
                    onRestart={startLoginFlow}
                />
            )}

            {mode === 'apikey' && (
                <ApiKeyConnectionForm key="qwen-apikey" provider={provider} loading={loading} onSubmit={onCreate} />
            )}
        </div>
    );
}
