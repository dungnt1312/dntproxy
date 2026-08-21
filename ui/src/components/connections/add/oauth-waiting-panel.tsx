import type { ReactNode } from 'react';
import { AlertTriangle, ExternalLink, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

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
    submitLabel = 'Submit',
    children,
}: Props) {
    return (
        <div className="space-y-4 rounded-lg border bg-muted/40 p-5 text-left">
            <div className="space-y-1 text-center">
                {waiting && <Loader2 size={24} className="mx-auto mb-2 animate-spin text-primary" />}
                <p className="text-sm font-medium">{title}</p>
                <a
                    href={authUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mt-1 inline-flex items-center gap-1 text-xs text-primary hover:underline"
                >
                    Open browser manually <ExternalLink size={10} />
                </a>
                {expectedRedirect && (
                    <p className="break-all text-[10px] text-muted-foreground">
                        Expected redirect: <code className="rounded bg-muted px-1">{expectedRedirect}</code>
                    </p>
                )}
            </div>

            {children}

            <div className="space-y-1">
                <label htmlFor="oauth-callback-url" className="flex items-center gap-1 text-xs font-medium">
                    {callbackHint && <AlertTriangle size={10} className="text-muted-foreground" />}
                    {callbackLabel}
                </label>
                {callbackHint && <p className="text-[10px] text-muted-foreground">{callbackHint}</p>}
                <div className="flex gap-2">
                    <Input
                        id="oauth-callback-url"
                        autoComplete="off"
                        spellCheck={false}
                        value={callbackValue}
                        onChange={(event) => onCallbackChange(event.target.value)}
                        placeholder={callbackPlaceholder}
                        className="font-mono text-xs"
                    />
                    <Button
                        type="button"
                        onClick={onSubmit}
                        disabled={loading || !callbackValue.trim()}
                        size="sm"
                    >
                        {loading ? <Loader2 size={12} className="animate-spin" /> : submitLabel}
                    </Button>
                </div>
            </div>

            <Button type="button" variant="outline" size="sm" className="w-full" onClick={onCancel}>
                Cancel
            </Button>
        </div>
    );
}
