import { useCallback, useEffect, useRef, useState } from 'react';
import { Play } from 'lucide-react';
import { api } from '../../../api';
import { DeviceCodePanel } from '../DeviceCodePanel';
import { ProviderLogoIcon } from '../helpers';
import { ApiKeyConnectionForm } from '../ApiKeyConnectionForm';
import { Button } from '@/components/ui/button';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';
import { errorMessage } from './helpers';
import { SetupIntro } from './setup-intro';
import type { DeviceCodeState } from '../helpers';

type Props = {
    provider: ProviderConfigMeta;
    mode: string;
    loading: boolean;
    onCreate: (payload: CreateConnectionPayload) => Promise<void>;
    onSuccess: (message: string) => void;
    onError: (message: string) => void;
    onBusyChange: (busy: boolean) => void;
};

export function QwenConnectionForm({
    provider,
    mode,
    loading,
    onCreate,
    onSuccess,
    onError,
    onBusyChange,
}: Props) {
    const [starting, setStarting] = useState(false);
    const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null);
    const [polling, setPolling] = useState(false);
    const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    const clearPolling = useCallback(() => {
        if (pollRef.current) {
            clearTimeout(pollRef.current);
            pollRef.current = null;
        }
        setDeviceCode(null);
        setPolling(false);
    }, []);

    useEffect(() => () => {
        if (pollRef.current) clearTimeout(pollRef.current);
    }, []);

    useEffect(() => {
        onBusyChange(starting || polling || !!deviceCode);
        return () => onBusyChange(false);
    }, [starting, polling, deviceCode, onBusyChange]);

    const startPolling = useCallback((sessionId: string, interval: number) => {
        const ms = Math.max(interval, 3) * 1000;
        const poll = async () => {
            try {
                const res = await api.pollQwenOAuth(sessionId);
                if (res.status === 'pending') {
                    pollRef.current = setTimeout(poll, ms);
                    return;
                }
                if (res.status === 'success') {
                    onSuccess(`Qwen connected! ${res.email ? `(${res.email})` : res.name ?? ''}`);
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
        pollRef.current = setTimeout(poll, ms);
    }, [onError, onSuccess]);

    return (
        <div className="mx-auto max-w-lg space-y-5">
            <SetupIntro
                icon={<ProviderLogoIcon provider="qwen" size={20} />}
                title="Qwen AI"
                description={
                    <>
                        Connect to{' '}
                        <a href="https://qwen.ai" target="_blank" rel="noopener noreferrer" className="text-[#6366F1] hover:underline">
                            Qwen
                        </a>{' '}
                        — Free coding models via OAuth or paid via API key.
                    </>
                }
            />
            {mode === 'oauth' && !deviceCode && (
                <div className="space-y-4 text-center">
                    <Button
                        onClick={() => {
                            setStarting(true);
                            onError('');
                            void api
                                .startQwenOAuth()
                                .then((res) => {
                                    setDeviceCode(res);
                                    setPolling(true);
                                    startPolling(res.sessionId, res.interval);
                                })
                                .catch((error: unknown) => onError(errorMessage(error, 'Failed to start Qwen login')))
                                .finally(() => setStarting(false));
                        }}
                        disabled={starting}
                        size="sm"
                        className="bg-[#6366F1] hover:bg-[#5558E6]"
                    >
                        <Play size={14} className="mr-2" />
                        {starting ? 'Starting…' : 'Start Qwen Login'}
                    </Button>
                    <p className="text-[10px] text-muted-foreground">
                        Free tier: ~1,000–2,000 requests/day. No credit card needed.
                    </p>
                </div>
            )}

            {mode === 'oauth' && deviceCode && (
                <DeviceCodePanel
                    prompt="Enter this code on qwen.ai:"
                    userCode={deviceCode.userCode}
                    verificationUri={deviceCode.verificationUri}
                    verificationUriComplete={deviceCode.verificationUriComplete}
                    intervalSec={deviceCode.interval}
                    waiting={polling}
                    codeClassName="text-[#6366F1]"
                    buttonClassName="bg-[#6366F1] hover:bg-[#5558E6]"
                    onCancel={clearPolling}
                />
            )}

            {mode === 'apikey' && (
                <ApiKeyConnectionForm key="qwen-apikey" provider={provider} loading={loading} onSubmit={onCreate} />
            )}
        </div>
    );
}
