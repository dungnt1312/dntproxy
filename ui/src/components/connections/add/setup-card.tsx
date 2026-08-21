import type { ReactNode, RefObject } from 'react';
import { AlertTriangle, KeyRound } from 'lucide-react';
import { Badge } from '@/components/ui/badge';

type Props = {
    name: string;
    description: string;
    icon: ReactNode;
    authLabel: string;
    error?: string;
    errorRef?: RefObject<HTMLDivElement | null>;
    children: ReactNode;
};

export function SetupCard({ name, description, icon, authLabel, error, errorRef, children }: Props) {
    return (
        <div className="rounded-xl border bg-card">
            <div className="flex flex-col gap-3 border-b bg-muted/20 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background">
                        {icon}
                    </div>
                    <div className="min-w-0">
                        <p className="truncate text-sm font-semibold">{name} setup</p>
                        <p className="truncate text-xs text-muted-foreground">{description}</p>
                    </div>
                </div>
                <Badge variant="outline" className="w-fit gap-1 text-xs">
                    <KeyRound size={12} /> {authLabel}
                </Badge>
            </div>
            <div className="p-6">{children}</div>
            {error && (
                <div className="border-t border-destructive/20">
                    <div
                        ref={errorRef}
                        role="alert"
                        tabIndex={-1}
                        className="flex items-center gap-3 bg-destructive/5 px-6 py-3 text-sm text-destructive outline-none focus-visible:ring-2 focus-visible:ring-destructive/40 focus-visible:ring-inset"
                    >
                        <AlertTriangle size={16} className="shrink-0" />
                        <span className="min-w-0 break-words">{error}</span>
                    </div>
                </div>
            )}
        </div>
    );
}
