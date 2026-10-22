import { api } from '@/api';

/** Download the given connections as a dntproxy export JSON (re-importable via "Import"). */
export async function downloadConnectionsExport(ids: string[]): Promise<void> {
  if (ids.length === 0) return;
  const data = await api.exportConnections(ids);
  const stamp = new Date().toISOString().replace(/[:T]/g, '-').slice(0, 19);
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `dntproxy-connections-${stamp}.json`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
