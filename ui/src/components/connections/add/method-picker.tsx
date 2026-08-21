import { KeyRound, Link, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { ProviderConfigMeta } from '@/types/provider-metadata';

const methodDetails: Record<string, { label: string; description: string; icon: typeof KeyRound }> = {
    oauth: { label: 'Sign in with OAuth', description: 'Connect securely in your browser without pasting a key.', icon: Link },
    apikey: { label: 'Use an API key', description: 'Paste a key created in the provider console.', icon: KeyRound },
    file: { label: 'Import a credential file', description: 'Upload an existing provider credential export.', icon: Upload },
    import: { label: 'Import credentials', description: 'Import an existing provider credential.', icon: Upload },
};

function detailFor(method: string) {
    return methodDetails[method] ?? {
        label: method.replace(/[-_]/g, ' '),
        description: `Connect using ${method.replace(/[-_]/g, ' ')}.`,
        icon: KeyRound,
    };
}

type Props = {
    provider: ProviderConfigMeta;
    onSelect: (method: string) => void;
};

export function MethodPicker({ provider, onSelect }: Props) {
    const recommended = provider.ui.preferredAuthMethod;

    return (
        <section className="mx-auto max-w-2xl space-y-4 rounded-xl border bg-card p-5">
            <div>
                <h2 className="text-base font-semibold">How would you like to connect?</h2>
                <p className="mt-1 text-sm text-muted-foreground">Choose the method that matches the account or credentials you already have.</p>
            </div>
            <div className="space-y-3">
                {provider.ui.authFlows.map((method) => {
                    const detail = detailFor(method);
                    const Icon = detail.icon;
                    return (
                        <div key={method} className="flex items-center gap-4 rounded-lg border p-4">
                            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
                                <Icon size={18} />
                            </div>
                            <div className="min-w-0 flex-1">
                                <div className="flex items-center gap-2">
                                    <h3 className="text-sm font-medium capitalize">{detail.label}</h3>
                                    {method === recommended && <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">Recommended</span>}
                                </div>
                                <p className="mt-0.5 text-xs text-muted-foreground">{detail.description}</p>
                            </div>
                            <Button size="sm" onClick={() => onSelect(method)}>Continue</Button>
                        </div>
                    );
                })}
            </div>
        </section>
    );
}
