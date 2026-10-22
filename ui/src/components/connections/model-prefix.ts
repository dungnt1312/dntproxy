import { canonicalProviderId, getModelProviderId, publicProviderPrefix } from '@/lib/provider-registry';
import type { Connection } from '@/types/connections';

type PrefixConn = Pick<Connection, 'provider' | 'routePrefix'>;

/** Short aliases accepted by the backend router (mirrors domain.ProviderAliasToID). */
const ALIASES_BY_PROVIDER: Record<string, string[]> = {
  kiro: ['kr'],
  openai: ['oai'],
  'openai-compatible': ['oaic'],
  qwen: ['qw'],
  cline: ['cl'],
  commandcode: ['cmc', 'cmd'],
  xai: ['grok'],
  deepseek: ['ds'],
};

/** The route prefix a connection's models are addressed with ("kiro", or a custom routePrefix). */
export function modelPrefixFor(conn: PrefixConn): string {
  return conn.provider === 'openai-compatible' && conn.routePrefix ? conn.routePrefix : getModelProviderId(conn.provider);
}

/** Every prefix that unambiguously refers to *this* connection's provider. */
function knownPrefixes(conn: PrefixConn): string[] {
  const id = canonicalProviderId(conn.provider);
  const set = new Set<string>([modelPrefixFor(conn), conn.provider, id, publicProviderPrefix(id), getModelProviderId(id)]);
  if (conn.provider === 'openai-compatible' && conn.routePrefix) set.add(conn.routePrefix);
  for (const a of ALIASES_BY_PROVIDER[id] ?? []) set.add(a);
  set.delete('');
  // Longest first so "openai-compatible/" wins over "openai/".
  return [...set].sort((a, b) => b.length - a.length);
}

/**
 * Strips this connection's provider prefix from a "prefix/model" string so it
 * can be stored in supportedModels. Only known prefixes are removed: upstream
 * ids that themselves contain a slash (e.g. "deepseek/deepseek-v4-pro",
 * "meta-llama/Llama-3") are kept intact.
 */
export function stripModelForConnection(model: string, conn: PrefixConn): string {
  const trimmed = model.trim();
  for (const prefix of knownPrefixes(conn)) {
    if (trimmed.startsWith(prefix + '/')) return trimmed.slice(prefix.length + 1);
  }
  return trimmed;
}

/** Inverse for display: give each stored model the connection's prefix (once). */
export function withModelPrefix(models: string[], conn: PrefixConn): string[] {
  const prefix = modelPrefixFor(conn);
  const known = knownPrefixes(conn);
  return models.map((m) => (known.some((p) => m.startsWith(p + '/')) ? m : `${prefix}/${m}`));
}
