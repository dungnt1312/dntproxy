export interface ProviderMeta {
  id: string;
  label: string;
  shortLabel?: string;
  description?: string;
  colorClass: string;
  accentClass?: string;
  modelProviderId?: string;
  canAdd?: boolean;
}

export const PROVIDER_ORDER = [
  'kiro',
  'openai',
  'qwen',
  'xai',
  'glm',
  'minimax',
  'anthropic',
  'cline',
  'commandcode',
  'gemini',
  'openai-compatible',
] as const;

export const PROVIDER_META: Record<string, ProviderMeta> = {
  kiro: {
    id: 'kiro',
    label: 'AWS / Kiro',
    shortLabel: 'Kiro',
    description: 'Amazon CodeWhisperer / Kiro',
    colorClass: 'bg-orange-500/10 border-orange-500/20 text-orange-600',
    accentClass: 'bg-orange-500 hover:bg-orange-600',
    canAdd: true,
  },
  openai: {
    id: 'openai',
    label: 'OpenAI',
    description: 'ChatGPT, GPT-4, o-series',
    colorClass: 'bg-emerald-500/10 border-emerald-500/20 text-emerald-600',
    accentClass: 'bg-emerald-600 hover:bg-emerald-700',
    canAdd: true,
  },
  qwen: {
    id: 'qwen',
    label: 'Qwen',
    description: 'Alibaba Qwen models',
    colorClass: 'bg-indigo-500/10 border-indigo-500/20 text-indigo-600',
    accentClass: 'bg-[#6366F1] hover:bg-[#5558E6]',
    canAdd: true,
  },
  xai: {
    id: 'xai',
    label: 'Grok Build',
    shortLabel: 'Grok',
    description: 'xAI Grok Build OAuth',
    colorClass: 'bg-slate-500/10 border-slate-500/20 text-slate-600',
    accentClass: 'bg-slate-900 hover:bg-slate-800',
    modelProviderId: 'xai',
    canAdd: true,
  },
  glm: {
    id: 'glm',
    label: 'GLM (Zhipu AI)',
    shortLabel: 'GLM',
    description: 'Zhipu AI / GLM',
    colorClass: 'bg-blue-500/10 border-blue-500/20 text-blue-600',
    accentClass: 'bg-[#0066FF] hover:bg-[#0055DD]',
    canAdd: true,
  },
  minimax: {
    id: 'minimax',
    label: 'MiniMax',
    description: 'MiniMax M2 series',
    colorClass: 'bg-orange-400/10 border-orange-400/20 text-orange-500',
    accentClass: 'bg-[#FF6B35] hover:bg-[#E85A25]',
    canAdd: true,
  },
  anthropic: {
    id: 'anthropic',
    label: 'Anthropic',
    description: 'Claude models via Anthropic API',
    colorClass: 'bg-amber-600/10 border-amber-600/20 text-amber-600',
    accentClass: 'bg-amber-600 hover:bg-amber-700',
    canAdd: true,
  },
  cline: {
    id: 'cline',
    label: 'ClinePass',
    description: 'ClinePass subscription models',
    colorClass: 'bg-rose-500/10 border-rose-500/20 text-rose-600',
    accentClass: 'bg-rose-600 hover:bg-rose-700',
    canAdd: true,
  },
  commandcode: {
    id: 'commandcode',
    label: 'Command Code',
    shortLabel: 'Cmc',
    description: 'Command Code generate API',
    colorClass: 'bg-teal-500/10 border-teal-500/20 text-teal-600',
    accentClass: 'bg-teal-600 hover:bg-teal-700',
    canAdd: true,
  },
  gemini: {
    id: 'gemini',
    label: 'Gemini',
    description: 'Google Gemini models',
    colorClass: 'bg-blue-400/10 border-blue-400/20 text-blue-500',
    accentClass: 'bg-blue-500 hover:bg-blue-600',
    modelProviderId: 'gemini',
    canAdd: true,
  },
  'openai-compatible': {
    id: 'openai-compatible',
    label: 'OpenAI Compatible',
    shortLabel: 'Custom',
    description: 'OpenAI-compatible API',
    colorClass: 'bg-purple-500/10 border-purple-500/20 text-purple-600',
    accentClass: 'bg-purple-600 hover:bg-purple-700',
    canAdd: true,
  },
};

const PROVIDER_ALIASES: Record<string, string> = {
  cmc: 'commandcode',
  cmd: 'commandcode',
  grok: 'xai',
};

const PROVIDER_PUBLIC_PREFIX: Record<string, string> = {
  commandcode: 'cmc',
};

export function canonicalProviderId(provider: string): string {
  const id = provider.trim().toLowerCase();
  return PROVIDER_ALIASES[id] ?? id;
}

export function publicProviderPrefix(provider: string): string {
  const id = canonicalProviderId(provider);
  return PROVIDER_PUBLIC_PREFIX[id] ?? id;
}

export function providersMatch(a?: string, b?: string): boolean {
  if (!a || !b) return false;
  return canonicalProviderId(a) === canonicalProviderId(b) || publicProviderPrefix(a) === publicProviderPrefix(b);
}

export function getProviderMeta(provider: string): ProviderMeta {
  const id = canonicalProviderId(provider);
  return PROVIDER_META[id] ?? PROVIDER_META[provider] ?? {
    id: provider || 'unknown',
    label: provider || 'Unknown Provider',
    colorClass: 'bg-gray-500/10 border-gray-500/20 text-gray-400',
  };
}

export function getProviderLabel(provider: string): string {
  return getProviderMeta(provider).label;
}

export function getModelProviderId(provider: string): string {
  return getProviderMeta(provider).modelProviderId ?? provider;
}

/** Providers with a dedicated add-connection flow (OAuth, device code, file import). */
export const CUSTOM_ADD_CONNECTION_PROVIDERS = ['kiro', 'openai', 'xai', 'qwen', 'commandcode'] as const;

export function usesGenericApiKeyForm(provider: {
  id: string;
  ui?: { authFlows?: string[]; formFields?: { name: string }[] };
}): boolean {
  if ((CUSTOM_ADD_CONNECTION_PROVIDERS as readonly string[]).includes(provider.id)) {
    return false;
  }
  const flows = provider.ui?.authFlows ?? [];
  if (flows.includes('apikey')) {
    return true;
  }
  return (provider.ui?.formFields ?? []).some((field) => field.name === 'apiKey');
}
