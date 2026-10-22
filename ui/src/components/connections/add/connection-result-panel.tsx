import { useEffect, useRef, useState } from 'react';
import { CheckCircle2, Gauge, Loader2, RefreshCw, Save, Search, XCircle } from 'lucide-react';
import { api } from '@/api';
import { Button } from '@/components/ui/button';
import { fetchModelList } from '../model-fetch';
import { stripModelForConnection } from '../model-prefix';
import { primaryQuotaPct, quotaStore, useQuotaEntry } from '@/components/screens/connections/quota-store';
import type { Connection } from '@/types/connections';
import { errorMessage, type CreateResult } from './helpers';

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
    const [modelsLive, setModelsLive] = useState(true);
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
            setTestResult({ ok: false, message: errorMessage(error, 'Connection test failed.') });
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
            const fetched = await fetchModelList(result.id);
            setModels(fetched.models);
            setModelsLive(fetched.live);
            setModelNote(
                fetched.live
                    ? 'Models fetched from the provider.'
                    : `These are the built-in default models, not the provider's live list.${fetched.note ? ` ${fetched.note}` : ''}`,
            );
        } catch (error: unknown) {
            setActionError(errorMessage(error, 'Failed to detect models.'));
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
            const stub: Connection = { id: result.id, name: result.name ?? '', provider: providerId, isActive: true, routePrefix: result.routePrefix };
            const normalized = models.map((m) => stripModelForConnection(m, stub));
            await api.updateConnection(result.id, { supportedModels: normalized, setModels: true });
            setModelNote('Detected models saved to this connection.');
        } catch (error: unknown) {
            setActionError(errorMessage(error, 'Failed to save models.'));
        } finally {
            setSaving(false);
        }
    };

    return (
        <section className="w-full space-y-5 rounded-xl border bg-card p-6">
            <div className="flex items-start gap-3">
                <CheckCircle2 className="mt-0.5 size-6 shrink-0 text-success" />
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
                    modelsLive={modelsLive}
                    modelNote={modelNote}
                    saving={saving}
                    actionError={actionError}
                    onRunTest={runTest}
                    onDetect={detectModels}
                    onSave={saveModels}
                />
            ) : (
                <p className="rounded-lg border border-warning/30 bg-warning/10 p-3 text-sm text-muted-foreground">
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
    modelsLive,
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
    modelsLive: boolean;
    modelNote: string;
    saving: boolean;
    actionError: string;
    onRunTest: () => void;
    onDetect: () => void;
    onSave: () => void;
}) {
    const entry = useQuotaEntry(id);
    const [quotaTouched, setQuotaTouched] = useState(false);
    // Same number the connections list shows for this account.
    const pct = primaryQuotaPct(entry);
    const fetchingQuota = quotaStore.isFetching(id);

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
                    <span className={testResult.ok ? 'flex items-center gap-1 text-sm text-success' : 'flex items-center gap-1 text-sm text-destructive'}>
                        {testResult.ok ? <CheckCircle2 className="size-4" /> : <XCircle className="size-4" />} {testResult.ok ? 'Passed' : 'Failed'}
                    </span>
                )}
                {testing && <span className="text-sm text-muted-foreground">Testing…</span>}
                <Button size="sm" variant="outline" onClick={onRunTest} disabled={testing} className="ml-auto gap-1.5">
                    {testing ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
                    {testResult ? 'Run again' : 'Run test'}
                </Button>
            </div>
            {testResult && !testResult.ok && <p role="alert" className="text-sm text-destructive">{testResult.message}</p>}

            {/* Quota row */}
            <div className="flex flex-wrap items-center gap-2 border-t pt-3">
                <span className="text-sm font-medium">Quota</span>
                {quotaTouched && entry?.data && !entry.error && (
                    <span className="font-mono text-sm tabular-nums">{pct === null ? 'No quota data' : `${pct}% used`}</span>
                )}
                {quotaTouched && entry?.error && <span className="text-sm text-destructive">{entry.error}</span>}
                {!quotaTouched && <span className="text-sm text-muted-foreground">Not checked</span>}
                <Button size="sm" variant="outline" className="ml-auto gap-1.5" onClick={checkQuota} disabled={fetchingQuota}>
                    {fetchingQuota ? <Loader2 className="size-3.5 animate-spin" /> : <Gauge className="size-3.5" />}
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
                    <Button size="sm" variant="outline" onClick={onDetect} disabled={detecting} className="ml-auto gap-1.5">
                        {detecting ? <Loader2 className="size-3.5 animate-spin" /> : <Search className="size-3.5" />}
                        {detecting ? 'Detecting…' : models.length > 0 ? 'Re-scan' : 'Detect models'}
                    </Button>
                </div>
                {models.length > 0 && (
                    <div className="mt-2 space-y-2">
                        <p className={modelsLive ? 'text-sm text-muted-foreground' : 'text-sm text-warning'}>{modelNote}</p>
                        <ul className="max-h-36 overflow-y-auto rounded border bg-background p-2 font-mono text-xs">
                            {models.map((model) => (
                                <li key={model} className="py-0.5">
                                    {model}
                                </li>
                            ))}
                        </ul>
                        {modelsLive ? (
                            <Button size="sm" onClick={onSave} disabled={saving} className="gap-1.5">
                                {saving ? <Loader2 className="size-3.5 animate-spin" /> : <Save className="size-3.5" />}
                                {saving ? 'Saving…' : 'Save detected models'}
                            </Button>
                        ) : (
                            <p className="text-xs text-muted-foreground">These defaults are already what this connection uses, so there is nothing to save.</p>
                        )}
                    </div>
                )}
            </div>

            {actionError && <p role="alert" className="border-t pt-2 text-sm text-destructive">{actionError}</p>}
        </div>
    );
}
