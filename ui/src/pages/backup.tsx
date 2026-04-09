import { useState, useRef } from 'react'
import { api } from '../api'
import { Download, Upload, Database, AlertTriangle, CheckCircle } from 'lucide-react'

interface BackupStats {
  connections: number
  combos: number
  aliases: number
  apiKeys: number
}

export default function Backup() {
  const [exportStats, setExportStats] = useState<BackupStats | null>(null)
  const [importResult, setImportResult] = useState<{ imported: number; skipped: number; mode: string } | null>(null)
  const [importMode, setImportMode] = useState<'merge' | 'replace'>('merge')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  async function handleExport(mask: boolean) {
    setLoading(true)
    setError(null)
    setExportStats(null)
    setImportResult(null)

    try {
      const data = await api.exportBackup(mask)
      
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `dntproxy-backup-${new Date().toISOString().slice(0, 19).replace(/[:-]/g, '')}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)

      setExportStats({
        connections: data.providerConnections?.length || 0,
        combos: data.combos?.length || 0,
        aliases: Object.keys(data.modelAliases || {}).length || 0,
        apiKeys: data.apiKeys?.length || 0,
      })
    } catch (e: any) {
      setError(e.message || 'Export failed')
    } finally {
      setLoading(false)
    }
  }

  async function handleImport() {
    fileInputRef.current?.click()
  }

  async function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return

    setLoading(true)
    setError(null)
    setExportStats(null)
    setImportResult(null)

    try {
      const text = await file.text()
      const data = JSON.parse(text)
      
      const result = await api.importBackup(data, importMode)
      setImportResult(result)
    } catch (e: any) {
      setError(e.message || 'Import failed')
    } finally {
      setLoading(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  return (
    <div className="max-w-2xl">
      <h2 className="text-2xl font-bold mb-6 flex items-center gap-2">
        <Database size={24} />
        Backup & Restore
      </h2>

      <div className="space-y-6">
        {/* Export Section */}
        <div className="bg-[var(--bg-card)] rounded-lg border border-[var(--border)] p-5">
          <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Download size={18} />
            Export Backup
          </h3>
          <p className="text-sm text-[var(--text-muted)] mb-4">
            Download all your connections, combos, aliases, and settings as a JSON file.
          </p>
          <div className="flex gap-3">
            <button
              onClick={() => handleExport(false)}
              disabled={loading}
              className="flex-1 bg-[var(--accent)] text-white px-4 py-2 rounded-lg hover:opacity-90 disabled:opacity-50 text-sm font-medium"
            >
              Export (with tokens)
            </button>
            <button
              onClick={() => handleExport(true)}
              disabled={loading}
              className="flex-1 bg-[var(--bg-hover)] text-[var(--text)] px-4 py-2 rounded-lg hover:bg-[var(--border)] disabled:opacity-50 text-sm font-medium border border-[var(--border)]"
            >
              Export (masked)
            </button>
          </div>

          {exportStats && (
            <div className="mt-4 p-3 bg-green-900/20 rounded-lg border border-green-800">
              <div className="flex items-center gap-2 text-green-400 text-sm font-medium mb-2">
                <CheckCircle size={16} />
                Export successful
              </div>
              <div className="grid grid-cols-4 gap-2 text-xs text-[var(--text-muted)]">
                <span>{exportStats.connections} connections</span>
                <span>{exportStats.combos} combos</span>
                <span>{exportStats.aliases} aliases</span>
                <span>{exportStats.apiKeys} API keys</span>
              </div>
            </div>
          )}
        </div>

        {/* Import Section */}
        <div className="bg-[var(--bg-card)] rounded-lg border border-[var(--border)] p-5">
          <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Upload size={18} />
            Import Backup
          </h3>
          <p className="text-sm text-[var(--text-muted)] mb-4">
            Restore from a previously exported backup file.
          </p>

          <div className="mb-4">
            <label className="text-sm text-[var(--text-muted)] mb-2 block">Import mode:</label>
            <div className="flex gap-4">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="radio"
                  name="importMode"
                  value="merge"
                  checked={importMode === 'merge'}
                  onChange={() => setImportMode('merge')}
                  className="accent-[var(--accent)]"
                />
                <span>Merge</span>
                <span className="text-[var(--text-muted)] text-xs">(add to existing)</span>
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="radio"
                  name="importMode"
                  value="replace"
                  checked={importMode === 'replace'}
                  onChange={() => setImportMode('replace')}
                  className="accent-[var(--accent)]"
                />
                <span>Replace</span>
                <span className="text-[var(--text-muted)] text-xs">(overwrite all)</span>
              </label>
            </div>
          </div>

          <input
            ref={fileInputRef}
            type="file"
            accept=".json"
            onChange={handleFileSelect}
            className="hidden"
          />

          <button
            onClick={handleImport}
            disabled={loading}
            className="w-full bg-[var(--accent)] text-white px-4 py-2 rounded-lg hover:opacity-90 disabled:opacity-50 text-sm font-medium"
          >
            Select Backup File
          </button>

          {importResult && (
            <div className="mt-4 p-3 bg-green-900/20 rounded-lg border border-green-800">
              <div className="flex items-center gap-2 text-green-400 text-sm font-medium mb-2">
                <CheckCircle size={16} />
                Import successful
              </div>
              <div className="text-xs text-[var(--text-muted)]">
                {importResult.imported} items imported, {importResult.skipped} skipped (mode: {importResult.mode})
              </div>
            </div>
          )}
        </div>

        {/* Warning */}
        <div className="bg-yellow-900/20 rounded-lg border border-yellow-800 p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle size={18} className="text-yellow-400 shrink-0 mt-0.5" />
            <div className="text-sm">
              <p className="text-yellow-400 font-medium mb-1">Security Notice</p>
              <p className="text-[var(--text-muted)]">
                Backup files contain sensitive data (API keys, tokens). Store them securely and never share publicly.
                When exporting with "masked" option, tokens and API keys will be redacted.
              </p>
            </div>
          </div>
        </div>

        {/* Error Display */}
        {error && (
          <div className="bg-red-900/20 rounded-lg border border-red-800 p-4">
            <div className="flex items-center gap-2 text-red-400 text-sm font-medium">
              <AlertTriangle size={16} />
              {error}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
