import { useEffect, useRef, useState } from 'react';
import { CheckCircle2, Gauge, Loader2, RefreshCw, Save, Search } from 'lucide-react';
import { api } from '@/api';
import { Button } from '@/components/ui/button';
import { stripModelForConnection } from '../BulkModelsModal';
import { quotaStore, useQuotaEntry } from '@/components/screens/connections/quota-store';
import type { Connection } from '@/types/connections';

type CreateResult = {
    id?: string;
    name?: string;
    routePrefix?: string;
};

type Props = {
    providerName: string;
    providerId: string;
    result: CreateResult;
    onAddAnother: () => void;
    onDone: () => void;
};

export function ConnectionResultPanel({ providerName, providerId, result, onAddAnother, onDone }: Props) {
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

    // Auto-verify once the panel opens — the step exists to prove the connection works.
    const autoRanRef = useRef(false);
    useEffect(() => {
        if (autoRanRef.current || !result.id) return;
        autoRanRef.current = true;
        void runTest();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [result.id]);

    const detectModels = async () => {
        if (!result.id) return;
        setDetecting(true);
        setActionError('');
        try {
            const response = await api.fetchConnectionModels(result.id);
            setModels(response.models ?? []);
            setModelNote(
                response.note ??
                    (response.source === 'provider-config'
                        ? 'These are provider default models. They were not fetched live.'
                        : 'Models fetched from the provider.'),
            );
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
            // Same normalization as the rest of the app — one shared helper.
            const stub = { id: 'stub', name: 'stub', provider: providerId, isActive: true } as Connection;
            const normalized = models.map((m) => stripModelForConnection(m, stub));
            await api.updateConnection(result.id, { supportedModels: normalized, setModels: true });
            setModelNote('Detected models saved to this connection.');
        } catch (error: unknown) {
            setActionError(error instanceof Error ? error.message : 'Failed to save models.');
        } finally {
            setSaving(false);
        }
    };

    return (
        <section className="w-full space-y-5 rounded-xl border bg-card p-6">
            <div className="flex items-start gap-3">
                <CheckCircle2 className="mt-0.5 h-6 w-6 shrink-0 text-emerald-500" />
                <div>
                    <h2 className="text-lg font-semibold">Connection added</h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                        <span className="font-medium text-foreground">{result.name ?? providerName}</span> is ready to serve requests. Verify it below, or configure it later from Connections.
                    </p>
                </div>
            </div>

            {result.id ? (
                <VerifyCard
                    id={result.id}
                    testing={testing}
                    testResult={testResult}
                    detecting={detecting}
                    models={models}
                    modelNote={modelNote}
                    saving={saving}
                    actionError={actionError}
                    onRunTest={runTest}
                    onDetect={detectModels}
                    onSave={saveModels}
                />
            ) : (
                <p className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 text-xs text-muted-foreground">
                    The provider completed authorization but did not return a connection ID. You can test and configure it from Connections.
                </p>
            )}

            <div className="flex flex-wrap justify-between gap-2 border-t pt-4">
                <Button variant="outline" onClick={onAddAnother}>
                    Add another connection
                </Button>
                <Button onClick={onDone}>Go to Connections</Button>
            </div>
        </section>
    );
}

// ─── Verify card ──────────────────────────────────────────────────────────────

function VerifyCard({
    id,
    testing,
    testResult,
    detecting,
    models,
    modelNote,
    saving,
    actionError,
    onRunTest,
    onDetect,
    onSave,
}: {
    id: string;
    testing: boolean;
    testResult: { ok: boolean; message: string } | null;
    detecting: boolean;
    models: string[];
    modelNote: string;
    saving: boolean;
    actionError: string;
    onRunTest: () => void;
    onDetect: () => void;
    onSave: () => void;
}) {
    const entry = useQuotaEntry(id);
    const [quotaTouched, setQuotaTouched] = useState(false);

    const worstPct = (() => {
        let worst = 0;
        for (const b of entry?.data?.quotas ?? []) {
            if (!b.unlimited && b.pct > worst) worst = b.pct;
        }
        return worst;
    })();

    const checkQuota = () => {
        quotaStore.refresh([id]);
        setQuotaTouched(true);
    };

    return (
        <div className="space-y-3 rounded-lg border bg-muted/20 p-4">
            {/* Test row */}
            <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium">Test</span>
                {testResult && (
                    <span className={testResult.ok ? 'flex items-center gap-1 text-xs text-emerald-600' : 'flex items-center gap-1 text-xs text-destructive'}>
                        {testResult.ok ? <CheckCircle2 size={12} /> : '✗'} {testResult.ok ? 'Pass' : 'Fail'}
                    </span>
                )}
                <Button size="sm" variant="outline" onClick={onRunTest} disabled={testing} className="ml-auto h-7">
                    {testing ? <Loader2 className="mr-1.5 h-3 w-3 animate-spin" /> : <RefreshCw className="mr-1.5 h-3 w-3" />}
                    {testing ? 'Testing…' : testResult ? 'Run again' : 'Run now'}
                </Button>
            </div>
            {testResult && <p className={testResult.ok ? 'text-[11px] text-emerald-600' : 'text-[11px] text-destructive'}>{testResult.message}</p>}

            {/* Quota row */}
            <div className="flex flex-wrap items-center gap-2 border-t pt-3">
                <span className="text-sm font-medium">Quota</span>
                {quotaTouched && entry?.data && !entry.error && (
                    <span className="font-mono text-xs tabular-nums text-foreground/80">{worstPct}% used</span>
                )}
                {quotaTouched && entry?.error && <span className="text-xs text-destructive">{entry.error}</span>}
                {!quotaTouched && <span className="text-[11px] text-muted-foreground">not checked</span>}
                <Button size="sm" variant="outline" className="ml-auto h-7" onClick={checkQuota}>
                    {quotaStore.isFetching(id) ? <Loader2 className="mr-1.5 h-3 w-3 animate-spin" /> : <Gauge className="mr-1.5 h-3 w-3" />}
                    {quotaTouched ? 'Refresh' : 'Check'}
                </Button>
            </div>

            {/* Models row */}
            <div className="border-t pt-3">
                <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium">Models</span>
                    {models.length > 0 && (
                        <span className="font-mono text-xs tabular-nums text-foreground/80">{models.length} found</span>
                    )}
                    <Button size="sm" variant="outline" onClick={onDetect} disabled={detecting} className="ml-auto h-7">
                        {detecting ? <Loader2 className="mr-1.5 h-3 w-3 animate-spin" /> : <Search className="mr-1.5 h-3 w-3" />}
                        {detecting ? 'Detecting…' : models.length > 0 ? 'Re-scan' : 'Detect models'}
                    </Button>
                </div>
                {models.length > 0 && (
                    <div className="mt-2 space-y-2">
                        <p className="text-[11px] text-muted-foreground">{modelNote}</p>
                        <div className="max-h-36 overflow-y-auto rounded border bg-background p-2 text-xs">
                            {models.map((model) => (
                                <div key={model} className="py-0.5 font-mono">
                                    {model}
                                </div>
                            ))}
                        </div>
                        <Button size="sm" onClick={onSave} disabled={saving} className="h-7">
                            {saving ? <Loader2 className="mr-1.5 h-3 w-3 animate-spin" /> : <Save className="mr-1.5 h-3 w-3" />}
                            {saving ? 'Saving…' : 'Save detected models'}
                        </Button>
                    </div>
                )}
            </div>

            {actionError && <p className="border-t pt-2 text-xs text-destructive">{actionError}</p>}
        </div>
    );
}
