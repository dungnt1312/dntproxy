import { useId, type ReactNode } from 'react';
import { Copy, ExternalLink, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { copyText } from './helpers';

type Props = {
    title: string;
    authUrl: string;
    waiting?: boolean;
    expectedRedirect?: string;
    callbackLabel?: string;
    callbackHint?: string;
    callbackPlaceholder?: string;
    callbackValue: string;
    onCallbackChange: (value: string) => void;
    onSubmit: () => void;
    onCancel: () => void;
    loading?: boolean;
    submitLabel?: string;
    children?: ReactNode;
};

/** OAuth sign-in step: open/copy the sign-in URL, then paste the callback to finish. */
export function OAuthWaitingPanel({
    title,
    authUrl,
    waiting = false,
    expectedRedirect,
    callbackLabel = 'Callback URL',
    callbackHint,
    callbackPlaceholder = 'Paste the callback URL…',
    callbackValue,
    onCallbackChange,
    onSubmit,
    onCancel,
    loading = false,
    submitLabel = 'Finish',
    children,
}: Props) {
    const id = useId();
    return (
        <div className="space-y-4 rounded-xl border bg-muted/40 p-5 text-left">
            <div className="space-y-3 text-center">
                <p className="flex items-center justify-center gap-2 text-sm font-medium" aria-live="polite">
                    {waiting && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
                    {title}
                </p>
                <div className="flex flex-wrap items-center justify-center gap-2">
                    <Button asChild className="gap-2">
                        <a href={authUrl} target="_blank" rel="noopener noreferrer">
                            <ExternalLink className="size-4" /> Open sign-in page
                        </a>
                    </Button>
                    <Button
                        type="button"
                        variant="outline"
                        className="gap-1.5"
                        onClick={async () => {
                            if (await copyText(authUrl)) toast.success('Sign-in URL copied');
                            else toast.error('Could not copy the URL');
                        }}
                        title="Copy URL to open in another browser"
                    >
                        <Copy className="size-4" /> Copy URL
                    </Button>
                </div>
                {expectedRedirect && (
                    <p className="break-all text-xs text-muted-foreground">
                        Expected redirect: <code className="rounded bg-muted px-1">{expectedRedirect}</code>
                    </p>
                )}
            </div>

            {children}

            <form
                className="space-y-1.5"
                onSubmit={(e) => {
                    e.preventDefault();
                    if (!loading && callbackValue.trim()) onSubmit();
                }}
            >
                <label htmlFor={`${id}-cb`} className="text-sm font-medium">
                    {callbackLabel}
                </label>
                {callbackHint && (
                    <p id={`${id}-hint`} className="text-xs text-muted-foreground">
                        {callbackHint}
                    </p>
                )}
                <div className="flex gap-2">
                    <Input
                        id={`${id}-cb`}
                        autoComplete="off"
                        spellCheck={false}
                        value={callbackValue}
                        onChange={(event) => onCallbackChange(event.target.value)}
                        placeholder={callbackPlaceholder}
                        aria-describedby={callbackHint ? `${id}-hint` : undefined}
                        className="font-mono text-sm"
                    />
                    <Button type="submit" disabled={loading || !callbackValue.trim()} className="gap-1.5">
                        {loading && <Loader2 className="size-4 animate-spin" />}
                        {submitLabel}
                    </Button>
                </div>
            </form>

            <Button type="button" variant="outline" className="w-full" onClick={onCancel}>
                Cancel
            </Button>
        </div>
    );
}
