import { useCallback, useEffect, useRef, useState } from 'react';
import type { DeviceCodeState } from '../helpers';
import { DEVICE_CODE_FALLBACK_SECS, errorMessage } from './helpers';

/** Shape returned by the device-code poll endpoints (Kiro, Qwen). */
export interface DevicePollResponse {
    status: 'pending' | 'success' | string;
    id?: string;
    name?: string;
    email?: string;
    error?: string;
    errorDescription?: string;
    connection?: { id?: string; name?: string };
}

interface Options {
    poll: (sessionId: string) => Promise<DevicePollResponse>;
    onSuccess: (res: DevicePollResponse) => void;
    onError: (message: string) => void;
}

/**
 * Device-code polling with a cancellation generation: every start/cancel bumps
 * the generation, and any in-flight request that resolves under an older
 * generation is ignored. This is what stops a cancelled login from still
 * creating a connection when its last poll returns "success".
 */
export function useDeviceCodePoll({ poll, onSuccess, onError }: Options) {
    const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null);
    const [polling, setPolling] = useState(false);
    const [expired, setExpired] = useState(false);
    const [expiresAt, setExpiresAt] = useState<number | null>(null);
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const genRef = useRef(0);
    const disposedRef = useRef(false);

    // Latest callbacks without restarting the loop.
    const cbRef = useRef({ poll, onSuccess, onError });
    useEffect(() => {
        cbRef.current = { poll, onSuccess, onError };
    });

    const stopTimer = () => {
        if (timerRef.current) clearTimeout(timerRef.current);
        timerRef.current = null;
    };

    const cancel = useCallback(() => {
        genRef.current++;
        stopTimer();
        setDeviceCode(null);
        setPolling(false);
        setExpired(false);
        setExpiresAt(null);
    }, []);

    useEffect(() => {
        const gen = genRef; // stable ref object; bumping .current invalidates in-flight polls
        disposedRef.current = false;
        return () => {
            disposedRef.current = true;
            gen.current++;
            stopTimer();
        };
    }, []);

    /**
     * Snapshot to take *before* awaiting the start request; pass it to start().
     * A cancel or unmount in between makes the late start a no-op.
     */
    const ticket = useCallback(() => genRef.current, []);

    const start = useCallback((code: DeviceCodeState, startedAt?: number) => {
        if (disposedRef.current) return;
        if (startedAt !== undefined && startedAt !== genRef.current) return;
        const gen = ++genRef.current;
        stopTimer();
        const ms = Math.max(code.interval || 0, 3) * 1000;
        const deadline = Date.now() + Math.max(30, code.expiresIn ?? DEVICE_CODE_FALLBACK_SECS) * 1000;
        setDeviceCode(code);
        setExpired(false);
        setPolling(true);
        setExpiresAt(deadline);
        let hadTransientError = false;

        const tick = async () => {
            if (gen !== genRef.current) return;
            if (Date.now() >= deadline) {
                setPolling(false);
                setExpired(true);
                return;
            }
            try {
                const res = await cbRef.current.poll(code.sessionId);
                if (gen !== genRef.current) return; // cancelled while in flight
                if (hadTransientError) {
                    hadTransientError = false;
                    cbRef.current.onError('');
                }
                if (res.status === 'pending') {
                    timerRef.current = setTimeout(tick, ms);
                    return;
                }
                genRef.current++;
                setDeviceCode(null);
                setPolling(false);
                setExpiresAt(null);
                if (res.status === 'success') cbRef.current.onSuccess(res);
                else cbRef.current.onError(res.errorDescription || res.error || 'Authorization failed');
            } catch (error: unknown) {
                if (gen !== genRef.current) return;
                // Transient failure: keep the session; the deadline is the hard stop.
                hadTransientError = true;
                cbRef.current.onError(`${errorMessage(error, 'Authorization check failed')} — retrying…`);
                timerRef.current = setTimeout(tick, Math.max(ms * 2, 10_000));
            }
        };
        timerRef.current = setTimeout(tick, ms);
    }, []);

    return { deviceCode, polling, expired, expiresAt, start, cancel, ticket };
}
