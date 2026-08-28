import { useId, useState, type ComponentProps } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

/**
 * A masked field for secrets that are *not* this app's login credential —
 * provider API keys, refresh tokens — with a reveal toggle.
 *
 * The `autoComplete` value is load-bearing. Chrome ignores `"off"` on password
 * inputs and offers the credential it saved for the origin, which is how the
 * dashboard key ended up in the Kiro API-key box while the matching "username"
 * (the origin URL) was pushed into the nearest plain input, the connections
 * search box. `"new-password"` is the value Chrome honours, and unlike CSS
 * masking tricks it keeps a real password input, so the value stays hidden in
 * Firefox too.
 */
export function SecretInput({
    className,
    revealLabel = 'secret',
    ...props
}: Omit<ComponentProps<typeof Input>, 'type'> & { revealLabel?: string }) {
    const [revealed, setRevealed] = useState(false);
    const fallbackName = useId();

    return (
        <div className="relative">
            <Input
                {...props}
                type={revealed ? 'text' : 'password'}
                name={props.name ?? fallbackName}
                autoComplete="new-password"
                spellCheck={false}
                data-1p-ignore
                data-lpignore="true"
                className={cn('pr-9', className)}
            />
            <button
                type="button"
                onClick={() => setRevealed((v) => !v)}
                aria-label={`${revealed ? 'Hide' : 'Show'} ${revealLabel}`}
                aria-pressed={revealed}
                className="absolute right-1 top-1 flex h-7 w-7 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            >
                {revealed ? <EyeOff size={13} /> : <Eye size={13} />}
            </button>
        </div>
    );
}
