import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { ProviderConfigMeta } from '../types/provider-metadata';
import { PROVIDER_META, PROVIDER_ORDER } from './provider-registry';

function fallbackCatalog(): ProviderConfigMeta[] {
  return PROVIDER_ORDER.filter((id) => PROVIDER_META[id]?.canAdd !== false).map((id) => {
    const m = PROVIDER_META[id];
    return {
      id,
      name: m.label,
      icon: id,
      authMethods: ['apikey'],
      defaultBaseUrl: '',
      recommendedModels: [],
      format: 'openai-chat',
      supportsQuota: false,
      ui: {
        category: 'cloud',
        description: m.description ?? m.label,
        showBaseUrlField: !['openai', 'kiro', 'xai'].includes(id),
        authFlows: ['apikey'],
        formFields: [
          { name: 'name', label: 'Connection Name', type: 'text', required: false },
          { name: 'apiKey', label: 'API Key', type: 'password', required: true, secret: true },
          { name: 'baseUrl', label: 'Base URL', type: 'url', required: false },
        ],
        supportsModelSelect: true,
      },
    };
  });
}

function sortByRegistryOrder(list: ProviderConfigMeta[]): ProviderConfigMeta[] {
  const order = new Map<string, number>(PROVIDER_ORDER.map((id, i) => [id, i]));
  return [...list].sort((a, b) => (order.get(a.id) ?? 999) - (order.get(b.id) ?? 999));
}

export function useProviderCatalog() {
  const [providers, setProviders] = useState<ProviderConfigMeta[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = (await api.getProviders()) as ProviderConfigMeta[];
      setProviders(sortByRegistryOrder(data.length ? data : fallbackCatalog()));
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load providers');
      setProviders(fallbackCatalog());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { providers, loading, error, reload };
}