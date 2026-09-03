/** Bulk OpenAI auto-login job types (mirrors internal/service/autologin). */

export type AutoLoginAccountResult = {
    email: string;
    /** "success" | "error" | "skipped" | "stopped" */
    status: string;
    plan?: string;
    connectionId?: string;
    /** true = refreshed an existing connection instead of creating one */
    replaced: boolean;
    error?: string;
};

export type AutoLoginStatus = {
    running: boolean;
    stopped: boolean;
    total: number;
    done: number;
    failed: number;
    cancelled: number;
    /** healthy accounts left untouched (skip-existing) */
    skipped: number;
    workers: number;
    headless: boolean;
    active: string[];
    results: AutoLoginAccountResult[];
};

export type AutoLoginStartResponse = {
    started: boolean;
    total: number;
    workers: number;
    headless: boolean;
};
