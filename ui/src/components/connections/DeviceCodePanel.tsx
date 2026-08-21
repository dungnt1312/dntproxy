import { ExternalLink, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

type DeviceCodePanelProps = {
    /** Instruction shown above the code, e.g. "Enter this code on qwen.ai:" */
    prompt?: string;
    userCode: string;
    verificationUri?: string;
    verificationUriComplete?: string;
    /** Poll interval in seconds, shown while waiting. */
    intervalSec?: number;
    waiting: boolean;
    codeClassName?: string;
    buttonClassName?: string;
    onCancel: () => void;
};

/**
 * Shared device-code authorization panel (Kiro / Qwen): big select-all code,
 * link to the authorization page, waiting spinner, and a Cancel button.
 */
export function DeviceCodePanel({
    prompt = 'Enter this code on the authorization page:',
    userCode,
    verificationUri,
    verificationUriComplete,
    intervalSec,
    waiting,
    codeClassName,
    buttonClassName,
    onCancel,
}: DeviceCodePanelProps) {
    const href = verificationUriComplete || verificationUri;
    if (!href) return null;

    return (
        <div className="mt-4 space-y-3 rounded-xl border bg-muted/40 p-5">
            <div className="text-center">
                <p className="mb-2 text-xs text-muted-foreground">{prompt}</p>
                <div
                    className={cn(
                        'mb-4 select-all font-mono text-3xl font-bold tracking-[0.25em] text-primary',
                        codeClassName,
                    )}
                >
                    {userCode}
                </div>
                <Button asChild size="sm" className={cn('gap-2', buttonClassName)}>
                    <a href={href} target="_blank" rel="noopener noreferrer">
                        <ExternalLink size={14} /> Open Authorization Page
                    </a>
                </Button>
            </div>
            {waiting && (
                <div className="mt-3 flex items-center justify-center gap-2 border-t pt-3 text-xs text-muted-foreground">
                    <Loader2 size={12} className="animate-spin" /> Waiting for authorization
                    {intervalSec ? ` (${intervalSec}s)` : ''}…
                </div>
            )}
            <Button variant="outline" size="sm" className="w-full" onClick={onCancel}>
                Cancel
            </Button>
        </div>
    );
}
