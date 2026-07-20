import type { ReactNode } from 'react';

export interface Connection {
  id: string;
  name: string;
  provider: string;
  isActive: boolean;
  priority?: number;
  weight?: number;
  email?: string;
  baseUrl?: string;
  providerName?: string;
  authMethod?: string;
  authType?: string;
  apiKey?: string;
  supportedModels?: string[];
  routePrefix?: string;
  modelPrefix?: string;
  rateLimitedUntil?: string;
  expiresAt?: string;
  backoffLevel?: number;
  lastError?: string;
  modelLocks?: Record<string, string>;
  // Computed from provider config by the backend
  supportsQuota?: boolean;
  // Multi-tenancy: which tenant owns this connection (empty = admin/global)
  tenantId?: string;
}

export interface QuotaBucket {
  key: string;
  label: string;
  used: number;
  total: number;
  remaining: number;
  pct: number;
  resetAt?: string;
  unlimited: boolean;
}

export interface BillingHistoryEntry {
  year: number;
  month: number;
  includedUsed: number;
  onDemandUsed: number;
  totalUsed: number;
}

export interface UsageData {
  provider?: string;
  plan?: string;
  limitReached?: boolean;
  message?: string;
  quotas?: QuotaBucket[];
  overages?: {
    used: number;
    cap: number;
    remaining: number;
    status?: string;
    charge?: number;
    rate?: number;
  };
  history?: BillingHistoryEntry[];
  error?: string;
}

export interface TestConnectionResult {
  status?: string;
  message?: string;
  loading?: boolean;
}

export interface ConnectionGroup {
  id: string;
  label: string;
  items: Connection[];
  icon: ReactNode;
  colorClass: string;
}
