import { useEffect, useRef, useState } from 'react';
import { ExternalLink } from 'lucide-react';
import { api } from '../../../api';
import { ApiKeyConnectionForm } from '../ApiKeyConnectionForm';
import { Button } from '@/components/ui/button';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';
import {
    DEVICE_CODE_FALLBACK_SECS,
    errorMessage,
    pickResultId,
    type OnSuccess,
} from './helpers';
import { OAuthWaitingPanel } from './oauth-waiting-panel';
import { SetupIntro } from './setup-intro';

const OPENAI_POLL_INTERVAL_MS = 2000;

type Props = {
    provider: ProviderConfigMeta;
    mode: string;
    loading: boolean;
    onCreate: (payload: CreateConnectionPayload) => Promise<void>;
    onSuccess: OnSuccess;
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
    const [pollTimedOut, setPollTimedOut] = useState(false);
    const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
    const pollDeadlineRef = useRef<number | null>(null);

    const clearSession = () => {
        if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
        }
        pollDeadlineRef.current = null;
        setSession(null);
        setCallback('');
        setPollTimedOut(false);
    };

    useEffect(() => () => {
        if (pollRef.current) clearInterval(pollRef.current);
    }, []);

    useEffect(() => {
        onBusyChange(starting || !!session || Boolean(callback.trim()));
        return () => onBusyChange(false);
    }, [starting, session, callback, onBusyChange]);

    // The browser tab is NOT auto-opened here: window.open after `await` loses
    // user activation and gets popup-blocked. OAuthWaitingPanel owns the CTA.
    const startLogin = async () => {
        setStarting(true);
        onError('');
        setPollTimedOut(false);
        try {
            const res = await api.startOpenAIOAuth();
            setSession({ sessionId: res.sessionId, authUrl: res.authUrl });
            pollDeadlineRef.current = Date.now() + DEVICE_CODE_FALLBACK_SECS * 1000;
            pollRef.current = setInterval(async () => {
                if (pollDeadlineRef.current !== null && Date.now() >= pollDeadlineRef.current) {
                    if (pollRef.current) clearInterval(pollRef.current);
                    pollRef.current = null;
                    setPollTimedOut(true);
                    return;
                }
                try {
                    const poll = await api.pollOpenAIOAuth(res.sessionId);
                    if (poll.status === 'pending') return;
                    if (pollRef.current) clearInterval(pollRef.current);
                    pollRef.current = null;
                    if (poll.status === 'success') {
                        onSuccess(`Connected! ${poll.email || poll.name || ''}`, pickResultId(poll));
                        clearSession();
                    } else {
                        onError(poll.error || 'Authorization failed');
                        clearSession();
                    }
                } catch (error: unknown) {
                    // Transient errors keep the interval running; deadline stops it.
                    onError(`${errorMessage(error, 'Authorization failed')} — retrying…`);
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
            pollRef.current = null;
            const poll = await api.pollOpenAIOAuth(session.sessionId, callback.trim());
            if (poll.status === 'success') {
                onSuccess(`Connected! ${poll.email || poll.name || ''}`, pickResultId(poll));
                clearSession();
            } else {
                onError(poll.error || 'Authorization failed');
                clearSession();
            }
        } catch (error: unknown) {
            onError(errorMessage(error, 'Authorization failed'));
        } finally {
            setStarting(false);
        }
    };

    return (
        <div className="space-y-5">
            {mode === 'oauth' && !session && (
                <div className="space-y-4">
                    <SetupIntro
                        icon={<ExternalLink size={20} className="text-emerald-500" />}
                        title="OpenAI OAuth"
                        description="Securely connect via the PKCE flow. No API key needed."
                    />
                    <Button onClick={() => void startLogin()} disabled={starting} size="sm" className="gap-2 bg-emerald-600 hover:bg-emerald-700">
                        <ExternalLink size={13} />
                        Start Login
                    </Button>
                </div>
            )}

            {mode === 'oauth' && session && (
                <OAuthWaitingPanel
                    title={pollTimedOut ? 'Auto-wait timed out — paste the callback to finish' : 'Waiting for authorization…'}
                    waiting={!pollTimedOut}
                    authUrl={session.authUrl}
                    callbackHint={pollTimedOut ? 'Automatic polling stopped. Paste the callback URL once you have signed in:' : undefined}
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
