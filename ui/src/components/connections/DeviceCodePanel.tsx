import { useEffect, useState } from 'react';
import { Check, Copy, ExternalLink, Loader2, RotateCcw, Timer, TimerOff } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { copyText } from './add/helpers';

type DeviceCodePanelProps = {
    /** Instruction shown above the code, e.g. "Enter this code on qwen.ai:" */
    prompt?: string;
    userCode: string;
    verificationUri?: string;
    verificationUriComplete?: string;
    /** Epoch ms when the device code expires; a ticking countdown is shown until then. */
    expiresAtMs?: number;
    /** True once the code expired — polling stopped and restart is offered. */
    expired?: boolean;
    /** Restart callback shown when expired (e.g. "Get new code"). */
    onRestart?: () => void;
    waiting: boolean;
    onCancel: () => void;
};

function fmtRemain(totalSecs: number): string {
    const m = Math.floor(totalSecs / 60);
    const s = totalSecs % 60;
    return `${m}:${String(s).padStart(2, '0')}`;
}

/**
 * Shared device-code authorization panel (Kiro / Qwen): big code with copy,
 * link to the authorization page, expiry countdown and Cancel. On expiry an
 * explicit restart action replaces the indefinite "waiting" state.
 */
export function DeviceCodePanel({
    prompt = 'Enter this code on the authorization page:',
    userCode,
    verificationUri,
    verificationUriComplete,
    expiresAtMs,
    expired = false,
    onRestart,
    waiting,
    onCancel,
}: DeviceCodePanelProps) {
    const [secsLeft, setSecsLeft] = useState<number | null>(null);
    const [codeCopied, setCodeCopied] = useState(false);
    const countdownActive = expiresAtMs !== undefined && !expired;
    useEffect(() => {
        if (!countdownActive) return;
        const update = () => setSecsLeft(Math.max(0, Math.ceil((expiresAtMs - Date.now()) / 1000)));
        update();
        const t = setInterval(update, 1000);
        return () => clearInterval(t);
    }, [countdownActive, expiresAtMs]);

    if (expired) {
        return (
            <div role="status" className="space-y-3 rounded-xl border border-warning/30 bg-warning/10 p-5 text-center">
                <TimerOff className="mx-auto size-6 text-warning" />
                <p className="text-sm font-medium">Authorization code expired</p>
                <p className="text-sm text-muted-foreground">The session ended without confirmation. Request a new code to try again.</p>
                <div className="flex gap-2 pt-1">
                    <Button variant="outline" className="flex-1" onClick={onCancel}>
                        Cancel
                    </Button>
                    {onRestart && (
                        <Button className="flex-1 gap-1.5" onClick={onRestart}>
                            <RotateCcw className="size-4" /> Get new code
                        </Button>
                    )}
                </div>
            </div>
        );
    }

    const href = verificationUriComplete || verificationUri;

    const copyCode = async () => {
        if (await copyText(userCode)) {
            setCodeCopied(true);
            setTimeout(() => setCodeCopied(false), 1500);
        } else {
            toast.error('Could not copy — select the code manually');
        }
    };

    return (
        <div className="space-y-4 rounded-xl border bg-muted/40 p-5 text-center">
            <p className="text-sm text-muted-foreground">{prompt}</p>
            <div className="flex items-center justify-center gap-2">
                <output aria-label="Device code" className="select-all font-mono text-3xl font-bold tracking-[0.25em]">
                    {userCode}
                </output>
                <Button variant="outline" size="icon" className="size-9 shrink-0" onClick={() => void copyCode()} aria-label="Copy code" title="Copy code">
                    {codeCopied ? <Check className="size-4 text-success" /> : <Copy className="size-4" />}
                </Button>
            </div>

            {href ? (
                <div className="flex flex-wrap items-center justify-center gap-2">
                    <Button asChild className="gap-2">
                        <a href={href} target="_blank" rel="noopener noreferrer">
                            <ExternalLink className="size-4" /> Open authorization page
                        </a>
                    </Button>
                    <Button
                        variant="outline"
                        className="gap-1.5"
                        onClick={async () => {
                            if (await copyText(href)) toast.success('Authorization URL copied');
                            else toast.error('Could not copy the URL');
                        }}
                        title="Copy URL to open in another browser"
                    >
                        <Copy className="size-4" /> Copy URL
                    </Button>
                </div>
            ) : (
                <p className="text-sm text-warning">The provider did not return an authorization URL. Cancel and try again.</p>
            )}

            {(waiting || (countdownActive && secsLeft !== null)) && (
                <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1 border-t pt-3 text-sm text-muted-foreground" aria-live="polite">
                    {waiting && (
                        <span className="inline-flex items-center gap-1.5">
                            <Loader2 className="size-4 animate-spin" /> Waiting for authorization…
                        </span>
                    )}
                    {countdownActive && secsLeft !== null && (
                        <span className={cn('inline-flex items-center gap-1 font-mono tabular-nums', secsLeft <= 60 && 'text-warning')} title="Code expires in">
                            <Timer className="size-4" /> {fmtRemain(secsLeft)}
                        </span>
                    )}
                </div>
            )}
            <Button variant="outline" className="w-full" onClick={onCancel}>
                Cancel
            </Button>
        </div>
    );
}
