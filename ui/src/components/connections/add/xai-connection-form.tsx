import { useEffect, useState } from 'react';
import { ExternalLink } from 'lucide-react';
import { api } from '../../../api';
import { ProviderLogoIcon } from '../helpers';
import { Button } from '@/components/ui/button';
import { FileDropzone } from './file-dropzone';
import { errorMessage } from './helpers';
import { OAuthWaitingPanel } from './oauth-waiting-panel';
import { SetupIntro } from './setup-intro';

type Props = {
    mode: string;
    onSuccess: (message: string) => void;
    onError: (message: string) => void;
    onBusyChange: (busy: boolean) => void;
};

export function XaiConnectionForm({ mode, onSuccess, onError, onBusyChange }: Props) {
    const [loading, setLoading] = useState(false);
    const [session, setSession] = useState<{ sessionId: string; authUrl: string; redirectUri?: string } | null>(null);
    const [callback, setCallback] = useState('');

    useEffect(() => {
        onBusyChange(loading || !!session || Boolean(callback.trim()));
        return () => onBusyChange(false);
    }, [loading, session, callback, onBusyChange]);

    const run = async (fn: () => Promise<void>) => {
        setLoading(true);
        onError('');
        try {
            await fn();
        } catch (error: unknown) {
            onError(errorMessage(error, 'Grok setup failed'));
        } finally {
            setLoading(false);
        }
    };

    const importFiles = async (files: File[]) => {
        setLoading(true);
        onError('');
        let imported = 0;
        let skipped = 0;
        const errors: string[] = [];
        for (const file of files) {
            try {
                const data = JSON.parse(await file.text());
                const res = await api.importXAIAuthFile(data);
                if (res.duplicate) skipped++;
                else imported++;
            } catch (error: unknown) {
                errors.push(`${file.name}: ${errorMessage(error, 'failed')}`);
            }
        }
        setLoading(false);
        if (imported > 0 || skipped > 0) {
            const parts = [
                imported > 0 ? `${imported} imported` : '',
                skipped > 0 ? `${skipped} skipped (duplicate)` : '',
                errors.length > 0 ? `${errors.length} failed` : '',
            ].filter(Boolean);
            onSuccess(`Grok batch import: ${parts.join(', ')}`);
            return;
        }
        if (errors.length > 0) onError(errors.join('\n'));
    };

    return (
        <div className="mx-auto max-w-lg space-y-5">
            <SetupIntro
                icon={<ProviderLogoIcon provider="xai" size={20} />}
                title="Grok Build"
                description={
                    <>
                        Connect your Grok Build account. Models route as{' '}
                        <code className="rounded bg-muted px-1.5 py-0.5 text-[11px]">grok/&lt;model&gt;</code>.
                    </>
                }
            />
            {mode === 'oauth' && !session && (
                <div className="space-y-4 text-center">
                    <Button
                        onClick={() =>
                            void run(async () => {
                                const res = await api.startXAIOAuth();
                                setSession({
                                    sessionId: res.sessionId,
                                    authUrl: res.authUrl,
                                    redirectUri: res.redirectUri,
                                });
                                window.open(res.authUrl, '_blank');
                            })
                        }
                        disabled={loading}
                        size="sm"
                        className="gap-2 bg-slate-900 text-white hover:bg-slate-800"
                    >
                        <ExternalLink size={13} />
                        {loading ? 'Starting…' : 'Connect Grok Build'}
                    </Button>
                    <p className="text-[10px] text-muted-foreground">
                        Browser opens to xAI OAuth. Paste the callback URL here after authorization.
                    </p>
                </div>
            )}

            {mode === 'oauth' && session && (
                <OAuthWaitingPanel
                    title="Finish Grok authorization"
                    authUrl={session.authUrl}
                    expectedRedirect={session.redirectUri}
                    callbackPlaceholder="http://127.0.0.1:56121/callback?code=...&state=..."
                    callbackValue={callback}
                    onCallbackChange={setCallback}
                    submitLabel="Submit Callback"
                    onSubmit={() =>
                        void run(async () => {
                            const res = await api.exchangeXAIOAuth(session.sessionId, callback.trim());
                            onSuccess(`Grok Build connected! ${res.email || res.name || ''}`);
                        })
                    }
                    onCancel={() => {
                        setSession(null);
                        setCallback('');
                        onError('');
                    }}
                    loading={loading}
                />
            )}

            {mode === 'file' && (
                <div className="space-y-4 text-center">
                    <p className="text-xs text-muted-foreground">
                        Upload a Grok auth JSON file. Expected fields:{' '}
                        <code className="rounded bg-muted px-1 text-[11px]">access_token</code>,{' '}
                        <code className="rounded bg-muted px-1 text-[11px]">refresh_token</code>,{' '}
                        <code className="rounded bg-muted px-1 text-[11px]">email</code>.
                    </p>
                    <FileDropzone
                        multiple
                        disabled={loading}
                        title={loading ? 'Importing…' : 'Drop file here or click to select'}
                        hint="Grok auth JSON format"
                        ariaLabel="Select Grok auth JSON files"
                        onFiles={(files) => void importFiles(files)}
                    />
                </div>
            )}
        </div>
    );
}
