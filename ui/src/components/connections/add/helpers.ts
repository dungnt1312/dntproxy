/** Clipboard write that reports whether it actually succeeded. */
export async function copyText(text: string): Promise<boolean> {
    try {
        await navigator.clipboard.writeText(text);
        return true;
    } catch {
        return false;
    }
}

export function errorMessage(error: unknown, fallback: string): string {
    return error instanceof Error && error.message ? error.message : fallback;
}

/** What a completed flow reports back so the Verify step can test/detect/quota. */
export interface CreateResult {
    id?: string;
    name?: string;
    routePrefix?: string;
}

/** Success callback shared by every provider form. */
export type OnSuccess = (message: string, result?: CreateResult) => void;

/** Default device-code lifetime when the API omits expiresIn (10 minutes). */
export const DEVICE_CODE_FALLBACK_SECS = 600;

/** Extract the connection id from the assorted success shapes across auth endpoints. */
export function pickResultId(res: unknown): { id?: string; name?: string } {
    const r = res as { id?: string; name?: string; connection?: { id?: string; name?: string } } | null;
    if (!r) return {};
    if (r.connection?.id || r.connection?.name) return { id: r.connection?.id, name: r.connection?.name };
    return { id: r.id, name: r.name };
}
