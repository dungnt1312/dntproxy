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
    const [workers, setWorkers] = useState(3);
    const [headless, setHeadless] = useState(false);
    const [skipExisting, setSkipExisting] = useState(true);
    const [status, setStatus] = useState<AutoLoginStatus | null>(null);
    const [starting, setStarting] = useState(false);
    const [error, setError] = useState('');
    const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
    const seenDoneRef = useRef(0);

    const stopPolling = useCallback(() => {
        if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
        }
    }, []);

    const startPolling = useCallback(() => {
        stopPolling();
        pollRef.current = setInterval(async () => {
            try {
                const next: AutoLoginStatus = await api.getOpenAIAutoLoginStatus();
                setStatus(next);
                if (next.done > seenDoneRef.current) {
                    seenDoneRef.current = next.done;
                    onImported();
                }
                if (!next.running) stopPolling();
            } catch {
                // transient — deadline logic lives server-side; keep polling
            }
        }, POLL_INTERVAL_MS);
    }, [onImported, stopPolling]);

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
    };

    if (status) {
        return <RunProgress status={status} error={error} onStop={stop} onReset={reset} />;
    }

    return (
        <div className="w-full space-y-4">
            <SetupIntro
                icon={<Users size={18} className="text-emerald-500" />}
                title="Bulk auto-login"
                description="Paste one account per line as email|password|2FA (2FA optional). A built-in browser signs in each account and saves the tokens — tabs/comma separators also work."
            />

            <Textarea
                value={text}
                onChange={(e) => setText(e.target.value)}
                spellCheck={false}
                autoComplete="off"
                placeholder={'user1@gmail.com|password123|JBSWY3DPEHPK3PXP\nuser2@gmail.com|password456'}
                className="h-32 resize-none border-border bg-background/50 font-mono text-xs"
            />

            <div className="flex items-center justify-between gap-3 text-xs">
                <label className="flex items-center gap-2">
                    <span className="text-muted-foreground">Parallel</span>
                    <Input
                        type="number"
                        min={1}
                        max={10}
                        value={workers}
                        onChange={(e) => setWorkers(Math.max(1, Math.min(10, Number(e.target.value) || 1)))}
                        className="h-7 w-16 text-center"
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
                <p className="text-[11px] leading-tight text-muted-foreground">
                    {headless
                        ? 'Headless mode fails more often on bot checks.'
                        : 'Browser windows will open on this machine while accounts sign in.'}
                </p>
                <Button onClick={() => void start()} disabled={starting || accountCount === 0} size="sm" className="gap-1.5 bg-emerald-600 hover:bg-emerald-700">
                    {starting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Users className="h-3.5 w-3.5" />}
                    Sign in {accountCount || ''} {accountCount === 1 ? 'account' : 'accounts'}
                </Button>
            </div>

            {error && <p className="text-xs text-destructive">{error}</p>}
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
                <div className="flex items-center gap-2 text-sm font-medium">
                    {status.running ? <Loader2 className="h-4 w-4 animate-spin text-emerald-500" /> : <Check className="h-4 w-4 text-emerald-500" />}
                    {status.running ? `Signing in ${processed}/${status.total}…` : `Finished — ${processed}/${status.total}`}
                </div>
                {status.running ? (
                    <Button size="sm" variant="outline" className="h-7 gap-1.5" onClick={onStop}>
                        <Square className="h-3 w-3" />
                        Stop all
                    </Button>
                ) : (
                    <Button size="sm" variant="outline" className="h-7" onClick={onReset}>
                        Add more
                    </Button>
                )}
            </div>

            <Progress value={pct} className="h-1.5" />

            <div className="flex flex-wrap gap-3 text-[11px] text-muted-foreground">
                <span className="text-emerald-600">{status.done} connected</span>
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
                <p className="truncate text-[11px] text-muted-foreground">
                    In flight: <span className="font-mono">{status.active.join(', ')}</span>
                </p>
            )}

            {status.stopped && !status.running && <p className="text-[11px] text-amber-600">Stopped by user — remaining accounts were cancelled.</p>}
            {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
    );
}

function ResultRow({ result }: { result: AutoLoginAccountResult }) {
    let icon = <X className="h-3.5 w-3.5 shrink-0 text-destructive" />;
    let note = <span className="max-w-[45%] shrink-0 truncate text-[10px] text-destructive" title={result.error}>{result.error || result.status}</span>;
    if (result.status === 'success') {
        icon = <Check className="h-3.5 w-3.5 shrink-0 text-emerald-500" />;
        note = <span className="shrink-0 text-[10px] text-muted-foreground">{result.replaced ? 'updated' : 'added'}</span>;
    } else if (result.status === 'skipped') {
        icon = <SkipForward className="h-3.5 w-3.5 shrink-0 text-sky-500" />;
        note = <span className="max-w-[45%] shrink-0 truncate text-[10px] text-muted-foreground" title={result.error}>{result.error || 'skipped'}</span>;
    } else if (result.status === 'stopped') {
        icon = <CircleSlash className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />;
        note = <span className="shrink-0 text-[10px] text-muted-foreground">cancelled</span>;
    }
    return (
        <div className="flex items-center gap-2 rounded px-1.5 py-1 text-xs hover:bg-muted/30">
            {icon}
            <span className="min-w-0 flex-1 truncate font-mono" title={result.error || result.email}>
                {result.email}
            </span>
            {result.plan && <Badge variant="outline" className="h-4 px-1.5 text-[9px] uppercase">{result.plan}</Badge>}
            {note}
        </div>
    );
}
