import type { LogFilters } from "@/types/logs";

export { StatusBadge, formatDateTime, formatLatency } from "./helpers.tsx";

export function buildFilterParams(filters: LogFilters): URLSearchParams {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value && value !== "all") params.set(key, value);
  });
  return params;
}

export const DEFAULT_FILTERS: LogFilters = {
  range: "24h",
  connectionId: "all",
  provider: "all",
  level: "all",
  q: "",
};
