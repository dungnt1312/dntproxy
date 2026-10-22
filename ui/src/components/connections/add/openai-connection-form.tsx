import { useEffect, useRef, useState } from 'react';
import { ExternalLink, Loader2 } from 'lucide-react';
import { api } from '../../../api';
import { ApiKeyConnectionForm } from '../ApiKeyConnectionForm';
import { Button } from '@/components/ui/button';
import { BulkAutoLoginForm } from './bulk-auto-login-form';
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
    /** Fires when bulk auto-login lands new/updated connections. */
    onImported?: () => void;
};

export function OpenAIConnectionForm({
    provider,
    mode,
    loading,
    onCreate,
    onSuccess,
    onError,
    onBusyChange,
    onImported,
}: Props) {
    const [session, setSession] = useState<{ sessionId: string; authUrl: string } | null>(null);
    const [callback, setCallback] = useState('');
    const [starting, setStarting] = useState(false);
    const [pollTimedOut, setPollTimedOut] = useState(false);
    const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    // Bumped on every start/cancel/finish; stale in-flight polls compare and bail.
    const genRef = useRef(0);

    const stopPolling = () => {
        genRef.current++;
        if (pollRef.current) clearTimeout(pollRef.current);
        pollRef.current = null;
    };

    const clearSession = () => {
        stopPolling();
        setSession(null);
        setCallback('');
        setPollTimedOut(false);
    };

    useEffect(() => () => {
        genRef.current++;
        if (pollRef.current) clearTimeout(pollRef.current);
    }, []);

    useEffect(() => {
        onBusyChange(starting || !!session || Boolean(callback.trim()));
        return () => onBusyChange(false);
    }, [starting, session, callback, onBusyChange]);

    // The browser tab is NOT auto-opened here: window.open after `await` loses
    // user activation and gets popup-blocked. OAuthWaitingPanel owns the CTA.
    const startLogin = async () => {
        clearSession();
        setStarting(true);
        onError('');
        try {
            const res = await api.startOpenAIOAuth();
            setSession({ sessionId: res.sessionId, authUrl: res.authUrl });
            const gen = ++genRef.current;
            const deadline = Date.now() + DEVICE_CODE_FALLBACK_SECS * 1000;
            let hadError = false;
            // Sequential setTimeout loop: never more than one request in flight.
            const tick = async () => {
                if (gen !== genRef.current) return;
                if (Date.now() >= deadline) {
                    setPollTimedOut(true);
                    return;
                }
                try {
                    const poll = await api.pollOpenAIOAuth(res.sessionId);
                    if (gen !== genRef.current) return;
                    if (hadError) {
                        hadError = false;
                        onError('');
                    }
                    if (poll.status === 'pending') {
                        pollRef.current = setTimeout(tick, OPENAI_POLL_INTERVAL_MS);
                        return;
                    }
                    clearSession();
                    if (poll.status === 'success') onSuccess(`Connected${poll.email ? ` (${poll.email})` : ''}`, pickResultId(poll));
                    else onError(poll.error || 'Authorization failed');
                } catch (error: unknown) {
                    if (gen !== genRef.current) return;
                    hadError = true;
                    onError(`${errorMessage(error, 'Authorization check failed')} — retrying…`);
                    pollRef.current = setTimeout(tick, OPENAI_POLL_INTERVAL_MS * 3);
                }
            };
            pollRef.current = setTimeout(tick, OPENAI_POLL_INTERVAL_MS);
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
        stopPolling();
        try {
            const poll = await api.pollOpenAIOAuth(session.sessionId, callback.trim());
            if (poll.status === 'success') {
                clearSession();
                onSuccess(`Connected${poll.email ? ` (${poll.email})` : ''}`, pickResultId(poll));
            } else {
                // Keep the session so the user can fix the pasted URL and retry.
                onError(poll.error || 'Authorization failed — check the callback URL and try again.');
                setPollTimedOut(true);
            }
        } catch (error: unknown) {
            onError(errorMessage(error, 'Authorization failed'));
        } finally {
            setStarting(false);
        }
    };

    return (
        <div className="space-y-5">
            {mode === 'bulk' && <BulkAutoLoginForm onImported={onImported ?? (() => {})} />}

            {mode === 'oauth' && !session && (
                <div className="space-y-4">
                    <SetupIntro
                        icon={<ExternalLink size={20} className="text-primary" />}
                        title="OpenAI OAuth"
                        description="Securely connect via the PKCE flow. No API key needed."
                    />
                    <Button onClick={() => void startLogin()} disabled={starting} className="gap-2">
                        {starting ? <Loader2 className="size-4 animate-spin" /> : <ExternalLink className="size-4" />}
                        {starting ? 'Starting…' : 'Start login'}
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
