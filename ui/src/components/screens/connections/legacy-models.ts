import { api } from '@/api';
import { getModelProviderId } from '@/lib/provider-registry';
import type { Connection } from '@/types/connections';

/**
 * Legacy/outdated model detection: flags connection models whose stripped
 * value no longer exists in the provider's current model registry/catalog.
 */

export type KnownModelIndex = Record<string, Set<string>>;

function connectionPrefix(conn: Connection): string {
  if (conn.provider === 'openai-compatible' && conn.routePrefix) return conn.routePrefix;
  return getModelProviderId(conn.provider);
}

/** Strip any "prefix/" (the connection's own prefix wins) so values compare equal. */
export function stripPrefix(model: string, conn: Connection): string {
  const trimmed = model.trim();
  const prefix = connectionPrefix(conn);
  if (trimmed.startsWith(prefix + '/')) return trimmed.slice(prefix.length + 1);
  const slash = trimmed.indexOf('/');
  return slash >= 0 ? trimmed.slice(slash + 1) : trimmed;
}

/**
 * Build provider-keyed sets of *stripped* registry model ids so they can be
 * compared against the unprefixed values stored on connections.
 */
export function buildKnownModelIndex(registry: unknown): KnownModelIndex {
  const index: KnownModelIndex = {};
  const models = (registry as { models?: Record<string, { provider?: string; id?: string }> })?.models ?? {};
  const stubConnCache = new Map<string, Connection>();

  for (const [, def] of Object.entries(models)) {
    const provider = def?.provider;
    const id = def?.id;
    if (!provider || !id) continue;

    let stub = stubConnCache.get(provider);
    if (!stub) {
      stub = { id: 'stub', name: 'stub', provider, isActive: true };
      stubConnCache.set(provider, stub);
    }
    const stripped = stripPrefix(id, stub);
    (index[provider] ??= new Set()).add(stripped.toLowerCase());
  }
  return index;
}

export function findLegacyModels(conn: Connection, index: KnownModelIndex): string[] {
  const known = index[getModelProviderId(conn.provider)];
  if (!known) return [];
  return (conn.supportedModels ?? []).filter((m) => !known.has(stripPrefix(m, conn).toLowerCase()));
}

let registryPromise: Promise<KnownModelIndex> | null = null;

/** Cached registry loader shared across renders/screens. */
export function loadKnownModelIndex(): Promise<KnownModelIndex> {
  registryPromise ??= api
    .getModelRegistry()
    .then(buildKnownModelIndex)
    .catch(() => ({}) as KnownModelIndex);
  return registryPromise;
}
