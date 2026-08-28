import { useEffect, useState } from 'react';
import { KeyRound, Loader2, Play, Search, Upload } from 'lucide-react';
import { api } from '../../api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { SecretInput } from '@/components/ui/secret-input';
import { ProviderLogoIcon } from './helpers';
import type { CreateConnectionPayload } from '@/types/provider-metadata';
import { parseCommandCodeAuthFile } from './CommandCodeAuthImport';
import { AuthMethodSelector } from './add/auth-method-selector';
import { FileDropzone } from './add/file-dropzone';

type Mode = 'detect' | 'file' | 'manual';

type DetectResult = {
  found?: boolean;
  imported?: boolean;
  duplicate?: boolean;
  error?: string;
  path?: string;
  name?: string;
};

type Props = {
  loading: boolean;
  onCreate: (payload: CreateConnectionPayload) => Promise<void>;
  onSuccess: (message: string) => void;
  onError: (message: string) => void;
  onBusyChange?: (busy: boolean) => void;
};

export function CommandCodeConnectionForm({
  loading,
  onCreate,
  onSuccess,
  onError,
  onBusyChange,
}: Props) {
  const [mode, setMode] = useState<Mode>('detect');
  const [busy, setBusy] = useState(false);
  const [apiKey, setApiKey] = useState('');
  const [name, setName] = useState('');
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
        onSuccess(`Command Code already connected as ${res.name || 'existing account'}`);
        return;
      }
      if (res.found && res.imported) {
        onSuccess(`Command Code imported from ${res.path || 'auth.json'}`);
        return;
      }
      onError(res.error || 'No Command Code auth.json found.');
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : 'Failed to detect Command Code auth');
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
      onError(e instanceof Error ? e.message : 'Failed to import auth.json');
    } finally {
      setBusy(false);
    }
  };

  const handleManual = async () => {
    if (!apiKey.trim()) return;
    await onCreate({
      provider: 'commandcode',
      name: name.trim() || undefined,
      apiKey: apiKey.trim(),
    });
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
          { id: 'detect', label: 'Auto Detect', description: 'Scan local auth.json', icon: <Search size={13} />, recommended: true },
          { id: 'file', label: 'Import File', description: 'Upload auth.json', icon: <Upload size={13} /> },
        ]}
        more={[{ id: 'manual', label: 'Manual', description: 'Paste API key', icon: <Play size={13} /> }]}
      />

      <div className="border-t pt-5">
        {mode === 'detect' && (
          <div className="text-center space-y-4 max-w-sm mx-auto">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 mx-auto">
              <Search size={20} className="text-primary" />
            </div>
            <div>
              <p className="font-medium text-sm mb-1">Auto Detect & Import</p>
              <p className="text-xs text-muted-foreground">
                Scan for Command Code credentials from{' '}
                <code className="bg-muted px-1.5 py-0.5 rounded text-[11px]">~/.commandcode/auth.json</code> on this
                machine.
              </p>
            </div>
            <Button onClick={() => void handleDetect()} disabled={working} size="sm" className="gap-2">
              {working ? <Loader2 size={13} className="animate-spin" /> : <Search size={13} />}
              {working ? 'Detecting…' : 'Scan & Import'}
            </Button>
          </div>
        )}

        {mode === 'file' && (
          <div className="text-center space-y-4 max-w-sm mx-auto">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 mx-auto">
              <Upload size={20} className="text-primary" />
            </div>
            <div>
              <p className="font-medium text-sm mb-1">Import from File</p>
              <p className="text-xs text-muted-foreground">
                Upload your <code className="bg-muted px-1.5 py-0.5 rounded text-[11px]">auth.json</code> from the
                Command Code CLI.
              </p>
            </div>
            <FileDropzone
              disabled={working}
              title={working ? 'Importing…' : 'Click to select auth.json'}
              ariaLabel="Select Command Code auth.json file"
              onFiles={(files) => void handleFile(files[0])}
            />
          </div>
        )}

        {mode === 'manual' && (
          <div className="space-y-4 max-w-md mx-auto">
            <div className="text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-muted mx-auto">
                <ProviderLogoIcon provider="commandcode" size={20} />
              </div>
              <p className="font-medium text-sm mt-3 mb-1">Manual API Key</p>
              <p className="text-xs text-muted-foreground">Paste the Command Code key from Studio or auth.json.</p>
            </div>
            <div className="space-y-1">
              <label htmlFor="commandcode-api-key" className="text-xs font-medium">
                API Key <span className="text-destructive">*</span>
              </label>
              <SecretInput
                id="commandcode-api-key"
                name="commandcode-api-key"
                revealLabel="API key"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="user_…"
                className="text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <label htmlFor="commandcode-name" className="text-xs font-medium">
                Display Name{' '}
                <span className="font-normal text-muted-foreground">(optional)</span>
              </label>
              <Input
                id="commandcode-name"
                autoComplete="off"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="My Command Code account…"
                className="text-xs"
              />
            </div>
            <Button onClick={() => void handleManual()} disabled={working || !apiKey.trim()} size="sm" className="w-full gap-2">
              {working ? <Loader2 size={13} className="animate-spin" /> : <KeyRound size={13} />}
              {working ? 'Adding…' : 'Add Command Code Connection'}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
