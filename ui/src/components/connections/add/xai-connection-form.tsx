import { useEffect, useState } from 'react';
import { ExternalLink, Loader2, ShieldCheck } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '../../../api';
import { ProviderLogoIcon } from '../helpers';
import { Button } from '@/components/ui/button';
import { FileDropzone } from './file-dropzone';
import {
    errorMessage,
    pickResultId,
    type CreateResult,
    type OnSuccess,
} from './helpers';
import { OAuthWaitingPanel } from './oauth-waiting-panel';
import { SetupIntro } from './setup-intro';

type Props = {
    mode: string;
    onSuccess: OnSuccess;
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
        let firstNewId: string | undefined;
        let firstName: string | undefined;
        const errors: string[] = [];
        for (const file of files) {
            try {
                let data: unknown;
                try {
                    data = JSON.parse(await file.text());
                } catch {
                    throw new Error('not valid JSON');
                }
                const res = (await api.importXAIAuthFile(data as object)) as CreateResult & { duplicate?: boolean };
                if (res.duplicate) skipped++;
                else {
                    imported++;
                    if (!firstNewId) {
                        firstNewId = res.id;
                        firstName = res.name;
                    }
                }
            } catch (error: unknown) {
                errors.push(`${file.name}: ${errorMessage(error, 'failed')}`);
            }
        }
        setLoading(false);
        if (errors.length > 0) {
            // Always surface per-file failures, even when other files imported fine.
            toast.error(`${errors.length} file${errors.length > 1 ? 's' : ''} failed to import`, {
                description: errors.slice(0, 5).join('\n') + (errors.length > 5 ? `\n+${errors.length - 5} more` : ''),
            });
        }
        if (imported > 0) {
            onSuccess(`Grok import: ${imported} imported`, firstNewId ? { id: firstNewId, name: firstName } : undefined);
            return;
        }
        if (skipped > 0 && errors.length === 0) {
            onError(`All ${skipped} file${skipped > 1 ? 's are' : ' is'} already connected — nothing new to import.`);
            return;
        }
        if (errors.length > 0) onError(errors.join('\n'));
    };

    return (
        <div className="space-y-5">
            <SetupIntro
                icon={<ProviderLogoIcon provider="xai" size={20} />}
                title="Grok Build"
                description={
                    <>
                        Connect your Grok Build account. Models route as{' '}
                        <code className="rounded bg-muted px-1.5 py-0.5 text-xs">grok/&lt;model&gt;</code>.
                    </>
                }
            />
            {mode === 'oauth' && !session && (
                <div className="space-y-4">
                    <Button
                        onClick={() =>
                            void run(async () => {
                                // No auto window.open — popup blockers kill it post-await.
                                // The waiting panel owns a prominent manual CTA.
                                const res = await api.startXAIOAuth();
                                setSession({
                                    sessionId: res.sessionId,
                                    authUrl: res.authUrl,
                                    redirectUri: res.redirectUri,
                                });
                            })
                        }
                        disabled={loading}
                        className="gap-2"
                    >
                        {loading ? <Loader2 className="size-4 animate-spin" /> : <ExternalLink className="size-4" />}
                        {loading ? 'Starting…' : 'Connect Grok Build'}
                    </Button>
                    <p className="text-sm text-muted-foreground">
                        You'll open the xAI sign-in page, then paste the callback URL here after authorizing.
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
                    submitLabel="Finish"
                    onSubmit={() =>
                        void run(async () => {
                            const res = await api.exchangeXAIOAuth(session.sessionId, callback.trim());
                            onSuccess(`Grok Build connected!`, pickResultId(res));
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
                <div className="space-y-4">
                    <p className="text-sm text-muted-foreground">
                        <ShieldCheck className="mr-1 inline size-4 text-success" />
                        Upload Grok auth JSON files with <code className="rounded bg-muted px-1 text-xs">access_token</code>,{' '}
                        <code className="rounded bg-muted px-1 text-xs">refresh_token</code> and{' '}
                        <code className="rounded bg-muted px-1 text-xs">email</code>. Multiple files are imported in one go; duplicates are skipped.
                    </p>
                    <FileDropzone
                        multiple
                        disabled={loading}
                        title={loading ? 'Importing…' : 'Drop files here or click to select'}
                        hint="Grok auth JSON format"
                        ariaLabel="Select Grok auth JSON files"
                        onFiles={(files) => void importFiles(files)}
                    />
                </div>
            )}
        </div>
    );
}
