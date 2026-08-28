import { useEffect, useState } from 'react';
import { Check, Copy, ExternalLink, Loader2, RotateCcw, TimerOff } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

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
    codeClassName?: string;
    buttonClassName?: string;
    onCancel: () => void;
};

function fmtRemain(totalSecs: number): string {
    const m = Math.floor(totalSecs / 60);
    const s = totalSecs % 60;
    return `${m}:${String(s).padStart(2, '0')}`;
}

/**
 * Shared device-code authorization panel (Kiro / Qwen): big select-all code,
 * link to the authorization page, expiry countdown, waiting spinner, and a
 * Cancel button. When the code expires an explicit restart action appears
 * instead of leaving an indefinite "waiting" state.
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
    codeClassName,
    buttonClassName,
    onCancel,
}: DeviceCodePanelProps) {
    // Ticker drives a seconds-remaining counter — no impure Date.now() in render.
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
            <div className="mt-4 space-y-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-5 text-center">
                <TimerOff size={22} className="mx-auto text-amber-600" />
                <p className="text-sm font-medium">Authorization code expired</p>
                <p className="text-xs text-muted-foreground">
                    The waiting session ended without confirmation. Request a new code — your selected method stays unchanged.
                </p>
                <div className="flex gap-2 pt-1">
                    <Button variant="outline" size="sm" className="flex-1" onClick={onCancel}>
                        Cancel
                    </Button>
                    <Button size="sm" className={cn('flex-1 gap-1.5', buttonClassName)} onClick={onRestart}>
                        <RotateCcw size={13} /> Get new code
                    </Button>
                </div>
            </div>
        );
    }

    const href = verificationUriComplete || verificationUri;
    if (!href) return null;

    const copyCode = () => {
        void navigator.clipboard?.writeText(userCode);
        setCodeCopied(true);
        toast.success('Code copied');
        setTimeout(() => setCodeCopied(false), 1500);
    };

    return (
        <div className="mt-4 space-y-3 rounded-xl border bg-muted/40 p-5">
            <div className="text-center">
                <p className="mb-2 text-xs text-muted-foreground">{prompt}</p>
                <div className="mb-4 flex items-center justify-center gap-2">
                    <div
                        className={cn(
                            'select-all font-mono text-3xl font-bold tracking-[0.25em] text-primary',
                            codeClassName,
                        )}
                    >
                        {userCode}
                    </div>
                    <button
                        type="button"
                        onClick={copyCode}
                        aria-label="Copy code"
                        title="Copy code"
                        className="shrink-0 cursor-pointer rounded-md border bg-background p-1.5 text-muted-foreground transition-colors hover:text-foreground"
                    >
                        {codeCopied ? <Check size={14} className="text-emerald-500" /> : <Copy size={14} />}
                    </button>
                </div>
                <div className="flex flex-wrap items-center justify-center gap-2">
                    <Button asChild size="sm" className={cn('gap-2', buttonClassName)}>
                        <a href={href} target="_blank" rel="noopener noreferrer">
                            <ExternalLink size={14} /> Open authorization page
                        </a>
                    </Button>
                    <Button
                        variant="outline"
                        size="sm"
                        className="gap-1.5"
                        onClick={() => {
                            void navigator.clipboard?.writeText(href);
                            toast.success('Authorization URL copied');
                        }}
                        title="Copy URL (paste into another browser if needed)"
                    >
                        <Copy size={13} /> Copy URL
                    </Button>
                </div>
            </div>
            {(waiting || (countdownActive && secsLeft !== null)) && (
                <div className="mt-3 flex flex-wrap items-center justify-center gap-x-3 gap-y-1 border-t pt-3 text-xs text-muted-foreground">
                    {countdownActive && secsLeft !== null && (
                        <span
                            className={cn(
                                'inline-flex items-center gap-1 rounded bg-background px-1.5 py-0.5 font-mono tabular-nums',
                                secsLeft <= 60 && 'text-amber-600',
                            )}
                            title="Code expires in"
                        >
                            ⏱ {fmtRemain(secsLeft)}
                        </span>
                    )}
                    {waiting && (
                        <span className="inline-flex items-center gap-1.5">
                            <Loader2 size={12} className="animate-spin" /> Waiting for authorization…
                        </span>
                    )}
                </div>
            )}
            <Button variant="outline" size="sm" className="w-full" onClick={onCancel}>
                Cancel
            </Button>
        </div>
    );
}
