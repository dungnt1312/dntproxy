import { useCallback, useEffect, useRef, useState } from 'react';
import { Play } from 'lucide-react';
import { api } from '../../../api';
import { DeviceCodePanel } from '../DeviceCodePanel';
import { ProviderLogoIcon } from '../helpers';
import { ApiKeyConnectionForm } from '../ApiKeyConnectionForm';
import { Button } from '@/components/ui/button';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';
import {
    DEVICE_CODE_FALLBACK_SECS,
    errorMessage,
    pickResultId,
    type OnSuccess,
} from './helpers';
import { SetupIntro } from './setup-intro';
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
    const [codeExpired, setCodeExpired] = useState(false);
    const [polling, setPolling] = useState(false);
    const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const [pollExpiresAt, setPollExpiresAt] = useState<number | null>(null);

    const clearPolling = useCallback(() => {
        if (pollRef.current) {
            clearTimeout(pollRef.current);
            pollRef.current = null;
        }
        setPollExpiresAt(null);
        setDeviceCode(null);
        setPolling(false);
        setCodeExpired(false);
    }, []);

    useEffect(() => () => {
        if (pollRef.current) clearTimeout(pollRef.current);
    }, []);

    useEffect(() => {
        onBusyChange(starting || polling || !!deviceCode || codeExpired);
        return () => onBusyChange(false);
    }, [starting, polling, deviceCode, codeExpired, onBusyChange]);

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
                    const res = await api.pollQwenOAuth(sessionId);
                    if (res.status === 'pending') {
                        pollRef.current = setTimeout(poll, ms);
                        return;
                    }
                    if (res.status === 'success') {
                        onSuccess(`Qwen connected! ${res.email ? `(${res.email})` : res.name ?? ''}`, pickResultId(res));
                        clearPolling();
                        return;
                    }
                    onError(res.errorDescription || res.error || 'Authorization failed');
                    clearPolling();
                } catch (error: unknown) {
                    // Keep polling through transient network errors; the expiry deadline is the hard stop.
                    onError(`${errorMessage(error, 'Authorization failed')} — retrying…`);
                    pollRef.current = setTimeout(poll, Math.max(ms * 2, 10_000));
                }
            };
            pollRef.current = setTimeout(poll, ms);
        },
        [onError, onSuccess, clearPolling],
    );

    const startLoginFlow = useCallback(() => {
        setStarting(true);
        onError('');
        void api
            .startQwenOAuth()
            .then((res: DeviceCodeState) => {
                setDeviceCode(res);
                setPolling(true);
                startPolling(res.sessionId, res.interval, res.expiresIn);
            })
            .catch((error: unknown) => onError(errorMessage(error, 'Failed to start Qwen login')))
            .finally(() => setStarting(false));
    }, [onError, startPolling]);

    return (
        <div className="space-y-5">
            <SetupIntro
                icon={<ProviderLogoIcon provider="qwen" size={20} />}
                title="Qwen AI"
                description={
                    <>
                        Connect to{' '}
                        <a href="https://qwen.ai" target="_blank" rel="noopener noreferrer" className="text-[#6366F1] hover:underline">
                            Qwen
                        </a>{' '}
                        — free coding models via OAuth, or the paid tier via API key.
                    </>
                }
            />
            {mode === 'oauth' && !deviceCode && !codeExpired && (
                <div className="space-y-4">
                    <Button onClick={startLoginFlow} disabled={starting} size="sm" className="bg-[#6366F1] hover:bg-[#5558E6]">
                        <Play size={14} className="mr-2" />
                        {starting ? 'Starting…' : 'Start Qwen login'}
                    </Button>
                    <p className="text-[10px] text-muted-foreground">
                        Free tier: ~1,000–2,000 requests/day. No credit card needed.
                    </p>
                </div>
            )}

            {(deviceCode || codeExpired) && mode === 'oauth' && (
                <DeviceCodePanel
                    prompt="Enter this code on qwen.ai:"
                    userCode={deviceCode?.userCode ?? '—'}
                    verificationUri={deviceCode?.verificationUri}
                    verificationUriComplete={deviceCode?.verificationUriComplete}
                    expiresAtMs={pollExpiresAt ?? undefined}
                    expired={codeExpired}
                    waiting={polling}
                    codeClassName="text-[#6366F1]"
                    buttonClassName="bg-[#6366F1] hover:bg-[#5558E6]"
                    onCancel={() => {
                        clearPolling();
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
