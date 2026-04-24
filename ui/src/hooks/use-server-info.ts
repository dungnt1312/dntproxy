import { useCallback, useEffect, useState } from "react";

import { goApi } from "@/lib/go-api";

export interface ServerInfo {
  version: string;
  port: number;
  localUrl: string;
  tunnelUrl: string;
  tunnelRunning: boolean;
  /** Best URL for CLI tools: tunnel if running, else local */
  baseUrl: string;
}

const CACHE_TTL = 30_000; // 30s

let cached: { data: ServerInfo; ts: number } | null = null;

export function useServerInfo() {
  const [info, setInfo] = useState<ServerInfo | null>(cached?.data ?? null);
  const [loading, setLoading] = useState(!cached);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const data = await goApi.getInfo();
      cached = { data, ts: Date.now() };
      setInfo(data);
    } catch {
      // keep stale data if available
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (cached && Date.now() - cached.ts < CACHE_TTL) {
      setInfo(cached.data);
      setLoading(false);
      return;
    }
    refresh();
  }, [refresh]);

  return { info, loading, refresh };
}
