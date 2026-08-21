import type { ReactNode } from 'react';

type Props = {
    icon: ReactNode;
    title: string;
    description: ReactNode;
};

export function SetupIntro({ icon, title, description }: Props) {
    return (
        <div className="mx-auto max-w-sm space-y-2 text-center">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">{icon}</div>
            <p className="text-sm font-medium">{title}</p>
            <p className="text-xs text-muted-foreground">{description}</p>
        </div>
    );
}
