import { useCallback, useEffect, useRef, useState } from 'react';
import { Check, CircleSlash, Loader2, SkipForward, Square, Users, X } from 'lucide-react';
import { api } from '../../../api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Progress } from '@/components/ui/progress';
import { Badge } from '@/components/ui/badge';
import { errorMessage } from './helpers';
import { SetupIntro } from './setup-intro';
import type { AutoLoginAccountResult, AutoLoginStatus } from '@/types/auto-login';

const POLL_INTERVAL_MS = 1500;

type Props = {
    /** Reloads the host connections list as soon as new accounts land. */
    onImported: () => void;
};

/**
 * Bulk OpenAI auto-login: paste email|password|2FA lines, dntproxy drives an
 * automated browser per account and saves each result as a connection.
 */
export function BulkAutoLoginForm({ onImported }: Props) {
    const [text, setText] = useState('');
    // Kept as text so the field can be cleared while typing; clamped on use/blur.
    const [workersText, setWorkersText] = useState('3');
    const workers = Math.max(1, Math.min(10, Number.parseInt(workersText, 10) || 1));
    const [headless, setHeadless] = useState(false);
    const [skipExisting, setSkipExisting] = useState(true);
    const [status, setStatus] = useState<AutoLoginStatus | null>(null);
    const [starting, setStarting] = useState(false);
    const [error, setError] = useState('');
    const [pollWarning, setPollWarning] = useState('');
    const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const genRef = useRef(0);
    const seenDoneRef = useRef(0);
    const onImportedRef = useRef(onImported);
    useEffect(() => {
        onImportedRef.current = onImported;
    });

    const stopPolling = useCallback(() => {
        genRef.current++;
        if (pollRef.current) clearTimeout(pollRef.current);
        pollRef.current = null;
    }, []);

    /** Sequential status loop — never overlaps requests; surfaces repeated failures. */
    const startPolling = useCallback(() => {
        stopPolling();
        const gen = genRef.current;
        let failures = 0;
        const tick = async () => {
            if (gen !== genRef.current) return;
            try {
                const next: AutoLoginStatus = await api.getOpenAIAutoLoginStatus();
                if (gen !== genRef.current) return;
                failures = 0;
                setPollWarning('');
                setStatus(next);
                if (next.done > seenDoneRef.current) {
                    seenDoneRef.current = next.done;
                    onImportedRef.current();
                }
                if (!next.running) return;
            } catch (err: unknown) {
                if (gen !== genRef.current) return;
                failures++;
                if (failures >= 3) setPollWarning(`Lost contact with the server (${errorMessage(err, 'network error')}) — still retrying…`);
            }
            pollRef.current = setTimeout(tick, failures > 0 ? POLL_INTERVAL_MS * 2 : POLL_INTERVAL_MS);
        };
        pollRef.current = setTimeout(tick, POLL_INTERVAL_MS);
    }, [stopPolling]);

    useEffect(() => {
        // Adopt an in-flight run (e.g. the dialog was reopened mid-run).
        void api
            .getOpenAIAutoLoginStatus()
            .then((s: AutoLoginStatus) => {
                if (s.running || (s.total > 0 && s.results.length < s.total)) {
                    seenDoneRef.current = s.done;
                    setStatus(s);
                    startPolling();
                }
            })
            .catch(() => {});
        return stopPolling;
    }, [startPolling, stopPolling]);

    const accountCount = text
        .split('\n')
        .map((l) => l.trim())
        .filter((l) => l && !l.startsWith('#')).length;

    const start = async () => {
        setStarting(true);
        setError('');
        try {
            await api.startOpenAIAutoLogin({ accounts: text.split('\n'), workers, headless, skipExisting });
            seenDoneRef.current = 0;
            setStatus({ running: true, stopped: false, total: accountCount, done: 0, failed: 0, cancelled: 0, skipped: 0, workers, headless, active: [], results: [] });
            startPolling();
        } catch (err: unknown) {
            setError(errorMessage(err, 'Failed to start bulk auto-login'));
        } finally {
            setStarting(false);
        }
    };

    const stop = async () => {
        setError('');
        try {
            const res = await api.stopOpenAIAutoLogin();
            if (res?.status) setStatus(res.status);
        } catch (err: unknown) {
            setError(errorMessage(err, 'Failed to stop'));
        }
    };

    const reset = () => {
        stopPolling();
        setStatus(null);
        setText('');
        setError('');
        setPollWarning('');
    };

    if (status) {
        return <RunProgress status={status} error={error || pollWarning} onStop={stop} onReset={reset} />;
    }

    return (
        <div className="w-full space-y-4">
            <SetupIntro
                icon={<Users size={18} className="text-primary" />}
                title="Bulk auto-login"
                description="Paste one account per line as email|password|2FA (2FA optional). A built-in browser signs in each account and saves the tokens — tabs/comma separators also work."
            />

            <label htmlFor="bulk-accounts" className="sr-only">Accounts, one per line</label>
            <Textarea
                id="bulk-accounts"
                value={text}
                onChange={(e) => setText(e.target.value)}
                spellCheck={false}
                autoComplete="off"
                placeholder={'user1@gmail.com|password123|JBSWY3DPEHPK3PXP\nuser2@gmail.com|password456'}
                className="h-32 resize-none border-border bg-background/50 font-mono text-sm"
            />

            <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
                <label className="flex items-center gap-2">
                    <span className="text-muted-foreground">Parallel</span>
                    <Input
                        type="number"
                        inputMode="numeric"
                        min={1}
                        max={10}
                        value={workersText}
                        onChange={(e) => setWorkersText(e.target.value)}
                        onBlur={() => setWorkersText(String(workers))}
                        className="h-9 w-16 text-center"
                    />
                </label>
                <label className="flex cursor-pointer items-center gap-2" title="Accounts that already have a valid connection are left untouched">
                    <span className="text-muted-foreground">Skip healthy existing</span>
                    <Switch checked={skipExisting} onCheckedChange={setSkipExisting} />
                </label>
                <label className="flex cursor-pointer items-center gap-2">
                    <span className="text-muted-foreground">Headless</span>
                    <Switch checked={headless} onCheckedChange={setHeadless} />
                </label>
            </div>

            <div className="flex items-center justify-between gap-2">
                <p className="text-sm leading-tight text-muted-foreground">
                    {headless
                        ? 'Headless mode fails more often on bot checks.'
                        : 'Browser windows will open on this machine while accounts sign in.'}
                </p>
                <Button onClick={() => void start()} disabled={starting || accountCount === 0} className="shrink-0 gap-1.5">
                    {starting ? <Loader2 className="size-4 animate-spin" /> : <Users className="size-4" />}
                    Sign in {accountCount || ''} {accountCount === 1 ? 'account' : 'accounts'}
                </Button>
            </div>

            {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        </div>
    );
}

// ─── Progress view ────────────────────────────────────────────────────────────

function RunProgress({ status, error, onStop, onReset }: { status: AutoLoginStatus; error: string; onStop: () => void; onReset: () => void }) {
    const processed = status.done + status.failed + status.cancelled + status.skipped;
    const pct = status.total > 0 ? Math.round((processed / status.total) * 100) : 0;

    return (
        <div className="w-full space-y-4">
            <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-sm font-medium" aria-live="polite">
                    {status.running ? <Loader2 className="size-4 animate-spin text-muted-foreground" /> : <Check className="size-4 text-success" />}
                    {status.running ? `Signing in ${processed}/${status.total}…` : `Finished — ${processed}/${status.total}`}
                </div>
                {status.running ? (
                    <Button size="sm" variant="outline" className="gap-1.5" onClick={onStop}>
                        <Square className="size-3.5" />
                        Stop all
                    </Button>
                ) : (
                    <Button size="sm" variant="outline" onClick={onReset}>
                        Add more
                    </Button>
                )}
            </div>

            <Progress value={pct} className="h-1.5" aria-label="Sign-in progress" />

            <div className="flex flex-wrap gap-3 text-sm text-muted-foreground">
                <span className="text-success">{status.done} connected</span>
                <span className="text-destructive">{status.failed} failed</span>
                {status.skipped > 0 && <span>{status.skipped} skipped (healthy)</span>}
                {status.cancelled > 0 && <span>{status.cancelled} cancelled</span>}
                <span className="ml-auto">
                    {status.workers} workers · {status.headless ? 'headless' : 'headed'}
                </span>
            </div>

            {status.results.length > 0 && (
                <div className="max-h-64 space-y-1 overflow-y-auto rounded-lg border bg-muted/10 p-2">
                    {status.results.map((r, i) => (
                        <ResultRow key={`${r.email}-${i}`} result={r} />
                    ))}
                </div>
            )}

            {status.running && status.active.length > 0 && (
                <p className="truncate text-sm text-muted-foreground">
                    In flight: <span className="font-mono">{status.active.join(', ')}</span>
                </p>
            )}

            {status.stopped && !status.running && <p className="text-sm text-warning">Stopped by user — remaining accounts were cancelled.</p>}
            {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        </div>
    );
}

function ResultRow({ result }: { result: AutoLoginAccountResult }) {
    let icon = <X className="size-4 shrink-0 text-destructive" aria-label="Failed" />;
    let note = <span className="max-w-[45%] shrink-0 truncate text-xs text-destructive" title={result.error}>{result.error || result.status}</span>;
    if (result.status === 'success') {
        icon = <Check className="size-4 shrink-0 text-success" aria-label="Connected" />;
        note = <span className="shrink-0 text-xs text-muted-foreground">{result.replaced ? 'updated' : 'added'}</span>;
    } else if (result.status === 'skipped') {
        icon = <SkipForward className="size-4 shrink-0 text-muted-foreground" aria-label="Skipped" />;
        note = <span className="max-w-[45%] shrink-0 truncate text-xs text-muted-foreground" title={result.error}>{result.error || 'skipped'}</span>;
    } else if (result.status === 'stopped') {
        icon = <CircleSlash className="size-4 shrink-0 text-muted-foreground" aria-label="Cancelled" />;
        note = <span className="shrink-0 text-xs text-muted-foreground">cancelled</span>;
    }
    return (
        <div className="flex items-center gap-2 rounded px-1.5 py-1 text-sm hover:bg-muted/30">
            {icon}
            <span className="min-w-0 flex-1 truncate font-mono text-xs" title={result.error || result.email}>
                {result.email}
            </span>
            {result.plan && <Badge variant="outline" className="h-5 px-1.5 text-xs uppercase">{result.plan}</Badge>}
            {note}
        </div>
    );
}
