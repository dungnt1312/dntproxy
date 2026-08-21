import { useEffect, useRef, useState } from 'react';
import { ExternalLink } from 'lucide-react';
import { api } from '../../../api';
import { ApiKeyConnectionForm } from '../ApiKeyConnectionForm';
import { Button } from '@/components/ui/button';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';
import { errorMessage } from './helpers';
import { OAuthWaitingPanel } from './oauth-waiting-panel';
import { SetupIntro } from './setup-intro';

const OPENAI_POLL_INTERVAL_MS = 2000;

type Props = {
    provider: ProviderConfigMeta;
    mode: string;
    loading: boolean;
    onCreate: (payload: CreateConnectionPayload) => Promise<void>;
    onSuccess: (message: string) => void;
    onError: (message: string) => void;
    onBusyChange: (busy: boolean) => void;
};

export function OpenAIConnectionForm({
    provider,
    mode,
    loading,
    onCreate,
    onSuccess,
    onError,
    onBusyChange,
}: Props) {
    const [session, setSession] = useState<{ sessionId: string; authUrl: string } | null>(null);
    const [callback, setCallback] = useState('');
    const [starting, setStarting] = useState(false);
    const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

    const clearSession = () => {
        if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
        }
        setSession(null);
        setCallback('');
    };

    useEffect(() => () => {
        if (pollRef.current) clearInterval(pollRef.current);
    }, []);

    useEffect(() => {
        onBusyChange(starting || !!session || Boolean(callback.trim()));
        return () => onBusyChange(false);
    }, [starting, session, callback, onBusyChange]);

    const startLogin = async () => {
        setStarting(true);
        onError('');
        try {
            const res = await api.startOpenAIOAuth();
            setSession({ sessionId: res.sessionId, authUrl: res.authUrl });
            window.open(res.authUrl, '_blank');
            pollRef.current = setInterval(async () => {
                try {
                    const poll = await api.pollOpenAIOAuth(res.sessionId);
                    if (poll.status === 'pending') return;
                    if (pollRef.current) clearInterval(pollRef.current);
                    if (poll.status === 'success') {
                        onSuccess(`Connected! ${poll.email || poll.name || ''}`);
                    } else {
                        onError(poll.error || 'Authorization failed');
                        setSession(null);
                    }
                } catch (error: unknown) {
                    if (pollRef.current) clearInterval(pollRef.current);
                    onError(errorMessage(error, 'Authorization failed'));
                    setSession(null);
                }
            }, OPENAI_POLL_INTERVAL_MS);
        } catch (error: unknown) {
            onError(errorMessage(error, 'Failed to start OpenAI OAuth'));
        } finally {
            setStarting(false);
        }
    };

    const submitCallback = async () => {
        if (!session || !callback.trim()) return;
        setStarting(true);
        onError('');
        try {
            if (pollRef.current) clearInterval(pollRef.current);
            const poll = await api.pollOpenAIOAuth(session.sessionId, callback.trim());
            if (poll.status === 'success') {
                onSuccess(`Connected! ${poll.email || poll.name || ''}`);
            } else {
                onError(poll.error || 'Authorization failed');
                setSession(null);
            }
        } catch (error: unknown) {
            onError(errorMessage(error, 'Authorization failed'));
            setSession(null);
        } finally {
            setStarting(false);
        }
    };

    return (
        <div className="mx-auto max-w-lg space-y-5">
            {mode === 'oauth' && !session && (
                <div className="space-y-4 text-center">
                    <SetupIntro
                        icon={<ExternalLink size={20} className="text-emerald-500" />}
                        title="OpenAI OAuth"
                        description="Securely connect via PKCE flow. No API key needed."
                    />
                    <Button onClick={() => void startLogin()} disabled={starting} size="sm" className="gap-2 bg-emerald-600 hover:bg-emerald-700">
                        <ExternalLink size={13} />
                        Start Login
                    </Button>
                </div>
            )}

            {mode === 'oauth' && session && (
                <OAuthWaitingPanel
                    title="Waiting for authorization…"
                    authUrl={session.authUrl}
                    waiting
                    callbackHint="Paste callback URL if not redirected automatically:"
                    callbackPlaceholder="http://localhost:1455/auth/callback?..."
                    callbackValue={callback}
                    onCallbackChange={setCallback}
                    onSubmit={() => void submitCallback()}
                    onCancel={() => {
                        clearSession();
                        onError('');
                    }}
                    loading={starting}
                />
            )}

            {mode === 'apikey' && (
                <ApiKeyConnectionForm key="openai-apikey" provider={provider} loading={loading} onSubmit={onCreate} />
            )}
        </div>
    );
}
