import { useMemo, useState, useId } from 'react';
import { Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { getProviderMeta } from '@/lib/provider-registry';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';
import { DynamicFormField } from './DynamicFormField';

function parseSupportedModels(str: string): string[] {
  return str
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean);
}

type Props = {
  provider: ProviderConfigMeta;
  loading: boolean;
  onSubmit: (payload: CreateConnectionPayload) => Promise<void>;
};

export function ApiKeyConnectionForm({ provider, loading, onSubmit }: Props) {
  const fields = provider.ui.formFields;
  const initial = useMemo(() => {
    const v: Record<string, string> = { supportedModels: '' };
    for (const f of fields) {
      if (f.name === 'baseUrl' && !f.defaultValue && provider.defaultBaseUrl) {
        v[f.name] = '';
      } else {
        v[f.name] = f.defaultValue ?? '';
      }
    }
    return v;
  }, [fields, provider.defaultBaseUrl]);

  const [values, setValues] = useState(initial);

  const set = (name: string, val: string) => setValues((prev) => ({ ...prev, [name]: val }));

  const canSubmit = useMemo(() => {
    if (provider.id === 'openai-compatible') {
      return Boolean(values.baseUrl?.trim());
    }
    const apiKeyField = fields.find((f) => f.name === 'apiKey');
    if (apiKeyField?.required) {
      return Boolean(values.apiKey?.trim());
    }
    return fields.filter((f) => f.required).every((f) => Boolean(values[f.name]?.trim()));
  }, [fields, provider.id, values]);

  const handleSubmit = async () => {
    const models = parseSupportedModels(values.supportedModels ?? '');
    const payload: CreateConnectionPayload = {
      provider: provider.id,
      name: values.name?.trim() || undefined,
      apiKey: values.apiKey?.trim() || undefined,
      baseUrl: values.baseUrl?.trim() || undefined,
      routePrefix: values.routePrefix?.trim() || undefined,
      modelPrefix: values.modelPrefix?.trim() || undefined,
      supportedModels: models.length > 0 ? models : undefined,
    };
    await onSubmit(payload);
  };

  const accent = getProviderMeta(provider.id).accentClass;
  const showModels = provider.ui.supportsModelSelect;
  const modelsInputId = useId();

  const ordered = fields.filter((f) => f.name !== 'supportedModels');

  return (
    <form
      className="space-y-3 max-w-lg mx-auto"
      onSubmit={(e) => {
        e.preventDefault();
        if (!loading && canSubmit) void handleSubmit();
      }}
    >
      {ordered.map((f) => (
        <DynamicFormField key={f.name} field={f} value={values[f.name] ?? ''} onChange={(v) => set(f.name, v)} />
      ))}
      {showModels && (
        <div className="space-y-1">
          <label htmlFor={modelsInputId} className="text-xs font-medium">
            Supported Models{' '}
            <span className="font-normal text-muted-foreground">(one per line, optional)</span>
          </label>
          <Textarea
            id={modelsInputId}
            name="supportedModels"
            value={values.supportedModels ?? ''}
            onChange={(e) => set('supportedModels', e.target.value)}
            placeholder="Defaults from provider when empty…"
            autoComplete="off"
            spellCheck={false}
            rows={3}
            className="text-xs font-mono"
          />
        </div>
      )}
      <Button type="submit" disabled={loading || !canSubmit} size="sm" className={cn('w-full', accent)}>
        {loading ? (
          <>
            <Loader2 size={13} className="animate-spin mr-2" />
            Adding…
          </>
        ) : (
          `Add ${provider.name} Connection`
        )}
      </Button>
    </form>
  );
}