import { useState } from 'react';
import { CheckCircle2, Loader2, RefreshCw, Save, Search } from 'lucide-react';
import { api } from '@/api';
import { Button } from '@/components/ui/button';
import { getModelProviderId } from '@/lib/provider-registry';
import type { Connection } from '@/types/connections';

type CreateResult = {
    id?: string;
    name?: string;
    routePrefix?: string;
};

type Props = {
    providerName: string;
    result: CreateResult;
    onAddAnother: () => void;
    onDone: () => void;
};

function modelPrefix(connection: Connection): string {
    if (connection.provider === 'openai-compatible' && connection.routePrefix) return connection.routePrefix;
    return getModelProviderId(connection.provider);
}

export function ConnectionResultPanel({ providerName, result, onAddAnother, onDone }: Props) {
    const [testing, setTesting] = useState(false);
    const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null);
    const [detecting, setDetecting] = useState(false);
    const [models, setModels] = useState<string[]>([]);
    const [modelNote, setModelNote] = useState('');
    const [saving, setSaving] = useState(false);
    const [actionError, setActionError] = useState('');

    const runTest = async () => {
        if (!result.id) return;
        setTesting(true);
        setActionError('');
        try {
            const response = await api.testConnection(result.id);
            const ok = response.status === 'ok';
            setTestResult({ ok, message: response.message ?? (ok ? 'Connection test passed.' : 'Connection test failed.') });
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : 'Connection test failed.';
            setTestResult({ ok: false, message });
        } finally {
            setTesting(false);
        }
    };

    const detectModels = async () => {
        if (!result.id) return;
        setDetecting(true);
        setActionError('');
        try {
            const response = await api.fetchConnectionModels(result.id);
            setModels(response.models ?? []);
            setModelNote(response.note ?? (response.source === 'provider-config' ? 'These are provider default models. They were not fetched live.' : 'Models fetched from the provider.'));
        } catch (error: unknown) {
            setActionError(error instanceof Error ? error.message : 'Failed to detect models.');
        } finally {
            setDetecting(false);
        }
    };

    const saveModels = async () => {
        if (!result.id) return;
        setSaving(true);
        setActionError('');
        try {
            const connections = await api.getConnections() as Connection[];
            const connection = connections.find((item) => item.id === result.id);
            const prefix = connection ? modelPrefix(connection) : '';
            const normalized = models.map((model) => model.startsWith(`${prefix}/`) ? model.slice(prefix.length + 1) : model);
            await api.updateConnection(result.id, { supportedModels: normalized, setModels: true });
            setModelNote('Detected models saved to this connection.');
        } catch (error: unknown) {
            setActionError(error instanceof Error ? error.message : 'Failed to save models.');
        } finally {
            setSaving(false);
        }
    };

    return (
        <section className="mx-auto max-w-2xl space-y-5 rounded-xl border bg-card p-6">
            <div className="flex items-start gap-3">
                <CheckCircle2 className="mt-0.5 h-6 w-6 shrink-0 text-emerald-500" />
                <div>
                    <h2 className="text-lg font-semibold">Connection added</h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {result.name ?? providerName} is ready to use. You can verify it now or configure it later from Connections.
                    </p>
                </div>
            </div>

            {result.id ? (
                <div className="space-y-3 rounded-lg border bg-muted/20 p-4">
                    <div>
                        <h3 className="text-sm font-medium">Recommended next steps</h3>
                        <p className="mt-1 text-xs text-muted-foreground">Testing is optional. Model discovery may return configured defaults for providers without a live model API.</p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                        <Button size="sm" variant="outline" onClick={() => void runTest()} disabled={testing}>
                            {testing ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="mr-1.5 h-3.5 w-3.5" />}
                            {testing ? 'Testing…' : 'Test connection'}
                        </Button>
                        <Button size="sm" variant="outline" onClick={() => void detectModels()} disabled={detecting}>
                            {detecting ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Search className="mr-1.5 h-3.5 w-3.5" />}
                            {detecting ? 'Detecting…' : 'Detect models'}
                        </Button>
                    </div>
                    {testResult && <p className={testResult.ok ? 'text-xs text-emerald-600' : 'text-xs text-destructive'}>{testResult.message}</p>}
                    {actionError && <p className="text-xs text-destructive">{actionError}</p>}
                    {models.length > 0 && (
                        <div className="space-y-3 border-t pt-3">
                            <p className="text-xs text-muted-foreground">{modelNote}</p>
                            <div className="max-h-36 overflow-y-auto rounded border bg-background p-2 text-xs">
                                {models.map((model) => <div key={model} className="py-0.5 font-mono">{model}</div>)}
                            </div>
                            <Button size="sm" onClick={() => void saveModels()} disabled={saving}>
                                {saving ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Save className="mr-1.5 h-3.5 w-3.5" />}
                                {saving ? 'Saving…' : 'Save detected models'}
                            </Button>
                        </div>
                    )}
                </div>
            ) : (
                <p className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 text-xs text-muted-foreground">
                    This provider completed its authorization flow, but did not return a connection ID. You can test and configure it from Connections.
                </p>
            )}

            <div className="flex flex-wrap justify-between gap-2 border-t pt-4">
                <Button variant="outline" onClick={onAddAnother}>Add another connection</Button>
                <Button onClick={onDone}>Go to Connections</Button>
            </div>
        </section>
    );
}
