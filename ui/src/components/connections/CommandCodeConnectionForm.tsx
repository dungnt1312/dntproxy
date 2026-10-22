import { useEffect, useState } from 'react';
import { KeyRound, Loader2, Play, Search, Upload } from 'lucide-react';
import { api } from '../../api';
import { Button } from '@/components/ui/button';
import { SecretInput } from '@/components/ui/secret-input';
import type { CreateConnectionPayload } from '@/types/provider-metadata';
import { AuthMethodSelector } from './add/auth-method-selector';
import { FileDropzone } from './add/file-dropzone';
import { errorMessage, type OnSuccess } from './add/helpers';

type Mode = 'detect' | 'file' | 'manual';

type DetectResult = {
  found?: boolean;
  imported?: boolean;
  duplicate?: boolean;
  error?: string;
  path?: string;
  id?: string;
  name?: string;
};

type Props = {
  loading: boolean;
  onCreate: (payload: CreateConnectionPayload) => Promise<void>;
  onSuccess: OnSuccess;
  onError: (message: string) => void;
  onBusyChange?: (busy: boolean) => void;
};

/** Parse a Command Code CLI auth.json into a create payload. */
function parseCommandCodeAuthFile(raw: string): CreateConnectionPayload {
  let data: { apiKey?: unknown; userName?: unknown; keyName?: unknown };
  try {
    data = JSON.parse(raw);
  } catch {
    throw new Error('The selected file is not valid JSON.');
  }
  const apiKey = typeof data.apiKey === 'string' ? data.apiKey.trim() : '';
  if (!apiKey) throw new Error('auth.json is missing apiKey');
  const name =
    (typeof data.userName === 'string' && data.userName.trim()) ||
    (typeof data.keyName === 'string' && data.keyName.trim()) ||
    'Command Code';
  return { provider: 'commandcode', name, apiKey };
}

export function CommandCodeConnectionForm({ loading, onCreate, onSuccess, onError, onBusyChange }: Props) {
  const [mode, setMode] = useState<Mode>('detect');
  const [busy, setBusy] = useState(false);
  const [apiKey, setApiKey] = useState('');
  const working = loading || busy;

  useEffect(() => {
    onBusyChange?.(working || Boolean(apiKey.trim()));
    return () => onBusyChange?.(false);
  }, [working, apiKey, onBusyChange]);

  const handleDetect = async () => {
    setBusy(true);
    onError('');
    try {
      const res = (await api.detectCommandCodeAuth({ import: true })) as DetectResult;
      if (res.found && res.duplicate) {
        onError(`This Command Code account is already connected as “${res.name || 'an existing connection'}”.`);
        return;
      }
      if (res.found && res.imported) {
        onSuccess(`Imported from ${res.path || 'auth.json'}`, { id: res.id, name: res.name });
        return;
      }
      onError(res.error || 'No Command Code auth.json found on this machine.');
    } catch (e: unknown) {
      onError(errorMessage(e, 'Failed to detect Command Code auth'));
    } finally {
      setBusy(false);
    }
  };

  const handleFile = async (file: File | undefined) => {
    if (!file) return;
    setBusy(true);
    onError('');
    try {
      await onCreate(parseCommandCodeAuthFile(await file.text()));
    } catch (e: unknown) {
      onError(errorMessage(e, 'Failed to import auth.json'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-5">
      <AuthMethodSelector
        value={mode}
        onChange={(next) => {
          setMode(next);
          onError('');
        }}
        primary={[
          { id: 'detect', label: 'Auto detect', description: 'Scan local auth.json', icon: <Search size={14} />, recommended: true },
          { id: 'file', label: 'Import file', description: 'Upload auth.json', icon: <Upload size={14} /> },
        ]}
        more={[{ id: 'manual', label: 'Manual', description: 'Paste API key', icon: <Play size={14} /> }]}
      />

      <div className="border-t pt-5">
        {mode === 'detect' && (
          <div className="mx-auto max-w-sm space-y-4 text-center">
            <p className="text-sm text-muted-foreground">
              Scan for Command Code credentials in{' '}
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs">~/.commandcode/auth.json</code> on this machine.
            </p>
            <Button onClick={() => void handleDetect()} disabled={working} className="gap-2">
              {working ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />}
              {working ? 'Scanning…' : 'Scan & import'}
            </Button>
          </div>
        )}

        {mode === 'file' && (
          <div className="mx-auto max-w-sm space-y-3 text-center">
            <p className="text-sm text-muted-foreground">
              Upload <code className="rounded bg-muted px-1.5 py-0.5 text-xs">auth.json</code> from the Command Code CLI.
            </p>
            <FileDropzone
              disabled={working}
              title={working ? 'Importing…' : 'Click to select auth.json'}
              ariaLabel="Select Command Code auth.json file"
              onFiles={(files) => void handleFile(files[0])}
            />
          </div>
        )}

        {mode === 'manual' && (
          <form
            className="mx-auto max-w-md space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              if (apiKey.trim()) void onCreate({ provider: 'commandcode', apiKey: apiKey.trim() });
            }}
          >
            <div className="space-y-1.5">
              <label htmlFor="commandcode-api-key" className="text-sm font-medium">
                API key <span className="text-destructive" aria-hidden>*</span>
              </label>
              <SecretInput
                id="commandcode-api-key"
                name="commandcode-api-key"
                revealLabel="API key"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="user_…"
                className="font-mono"
                required
              />
              <p className="text-xs text-muted-foreground">From Command Code Studio or the CLI's auth.json.</p>
            </div>
            <Button type="submit" disabled={working || !apiKey.trim()} className="w-full gap-2">
              {working ? <Loader2 className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
              {working ? 'Adding…' : 'Add connection'}
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}
