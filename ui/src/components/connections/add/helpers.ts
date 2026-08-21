export const LEAVE_SETUP_MESSAGE =
    'Connection setup is not finished (OAuth pending or unsaved input). Leave anyway?';

export function confirmLeaveSetup(): boolean {
    return window.confirm(LEAVE_SETUP_MESSAGE);
}

export function errorMessage(error: unknown, fallback: string): string {
    return error instanceof Error && error.message ? error.message : fallback;
}
