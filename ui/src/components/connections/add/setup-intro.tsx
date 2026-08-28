import type { ReactNode } from 'react';

type Props = {
    icon: ReactNode;
    title: string;
    description: ReactNode;
};

/** Left-aligned intro block — matches the modal's section rhythm. */
export function SetupIntro({ icon, title, description }: Props) {
    return (
        <div className="flex items-start gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">{icon}</div>
            <div className="min-w-0">
                <p className="text-sm font-medium leading-tight">{title}</p>
                <p className="mt-0.5 text-xs leading-snug text-muted-foreground">{description}</p>
            </div>
        </div>
    );
}
