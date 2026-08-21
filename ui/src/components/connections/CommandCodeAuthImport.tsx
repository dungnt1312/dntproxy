import { useState } from 'react';
import { Upload } from 'lucide-react';
import type { CreateConnectionPayload } from '@/types/provider-metadata';

type Props = {
  loading: boolean;
  onImport: (payload: CreateConnectionPayload) => Promise<void>;
  onError: (message: string) => void;
};

export function parseCommandCodeAuthFile(raw: string): CreateConnectionPayload {
  const data = JSON.parse(raw) as { apiKey?: unknown; userName?: unknown; keyName?: unknown };
  const apiKey = typeof data.apiKey === 'string' ? data.apiKey.trim() : '';
  if (!apiKey) {
    throw new Error('auth.json is missing apiKey');
  }
  const name =
    (typeof data.userName === 'string' && data.userName.trim()) ||
    (typeof data.keyName === 'string' && data.keyName.trim()) ||
    'Command Code';
  return { provider: 'commandcode', name, apiKey };
}

export function CommandCodeAuthImport({ loading, onImport, onError }: Props) {
  const [busy, setBusy] = useState(false);

  const handleFile = async (file: File | undefined) => {
    if (!file) return;
    setBusy(true);
    try {
      const payload = parseCommandCodeAuthFile(await file.text());
      await onImport(payload);
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : 'Failed to import Command Code auth.json');
    } finally {
      setBusy(false);
    }
  };

  return (
    <label className="flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed p-5 cursor-pointer hover:border-primary hover:bg-muted/40 transition-all">
      <Upload size={18} className="text-muted-foreground" />
      <div className="text-center">
        <p className="text-xs font-medium">{busy || loading ? 'Importing…' : 'Import ~/.commandcode/auth.json'}</p>
        <p className="text-[11px] text-muted-foreground mt-0.5">Uses the same API key as the Command Code CLI</p>
      </div>
      <input
        type="file"
        accept=".json,application/json"
        className="hidden"
        disabled={busy || loading}
        onChange={(e) => {
          const file = e.target.files?.[0];
          e.target.value = '';
          void handleFile(file);
        }}
      />
    </label>
  );
}
