import { useState, useRef } from 'react'
import { api } from '../api'
import { Download, Upload, AlertTriangle, CheckCircle, Loader2 } from 'lucide-react'

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
      <div className="page-header">
        <div>
          <h2 className="page-title">Backup & Restore</h2>
          <p className="page-subtitle">Export or import your proxy configuration.</p>
        </div>
      </div>

      <div className="space-y-5">
        {/* Export Section */}
        <div className="glass p-5">
          <h3 className="text-base font-semibold mb-3 flex items-center gap-2" style={{ fontFamily: 'var(--font-heading)' }}>
            <Download size={16} className="text-[var(--accent)]" />
            Export Backup
          </h3>
          <p className="text-xs text-[var(--text-muted)] mb-4">
            Download all your connections, combos, aliases, and settings as a JSON file.
          </p>
          <div className="flex gap-3">
            <button
              onClick={() => handleExport(false)}
              disabled={loading}
              className="btn-primary flex-1"
            >
              {loading && <Loader2 size={14} className="animate-spin" />}
              Export (with tokens)
            </button>
            <button
              onClick={() => handleExport(true)}
              disabled={loading}
              className="btn-ghost flex-1"
            >
              Export (masked)
            </button>
          </div>

          {exportStats && (
            <div className="mt-4 glass-sm p-3 border-[var(--success)]/20 animate-slide-up">
              <div className="flex items-center gap-2 text-[var(--success)] text-xs font-medium mb-2">
                <CheckCircle size={14} />
                Export successful
              </div>
              <div className="grid grid-cols-4 gap-2 text-[10px] text-[var(--text-muted)]">
                <span>{exportStats.connections} connections</span>
                <span>{exportStats.combos} combos</span>
                <span>{exportStats.aliases} aliases</span>
                <span>{exportStats.apiKeys} API keys</span>
              </div>
            </div>
          )}
        </div>

        {/* Import Section */}
        <div className="glass p-5">
          <h3 className="text-base font-semibold mb-3 flex items-center gap-2" style={{ fontFamily: 'var(--font-heading)' }}>
            <Upload size={16} className="text-[var(--purple)]" />
            Import Backup
          </h3>
          <p className="text-xs text-[var(--text-muted)] mb-4">
            Restore from a previously exported backup file.
          </p>

          <div className="mb-4">
            <label className="text-xs text-[var(--text-muted)] mb-2 block font-medium">Import mode:</label>
            <div className="flex gap-4">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="radio"
                  name="importMode"
                  value="merge"
                  checked={importMode === 'merge'}
                  onChange={() => setImportMode('merge')}
                  className="accent-[var(--accent)] cursor-pointer"
                />
                <span>Merge</span>
                <span className="chip chip-muted text-[10px]">add to existing</span>
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="radio"
                  name="importMode"
                  value="replace"
                  checked={importMode === 'replace'}
                  onChange={() => setImportMode('replace')}
                  className="accent-[var(--accent)] cursor-pointer"
                />
                <span>Replace</span>
                <span className="chip chip-danger text-[10px]">overwrite all</span>
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
            className="btn-primary w-full"
          >
            {loading && <Loader2 size={14} className="animate-spin" />}
            Select Backup File
          </button>

          {importResult && (
            <div className="mt-4 glass-sm p-3 border-[var(--success)]/20 animate-slide-up">
              <div className="flex items-center gap-2 text-[var(--success)] text-xs font-medium mb-1">
                <CheckCircle size={14} />
                Import successful
              </div>
              <div className="text-[10px] text-[var(--text-muted)]">
                {importResult.imported} items imported, {importResult.skipped} skipped (mode: {importResult.mode})
              </div>
            </div>
          )}
        </div>

        {/* Warning */}
        <div className="glass-sm p-4 border-[var(--warning)]/15">
          <div className="flex items-start gap-3">
            <AlertTriangle size={16} className="text-[var(--warning)] shrink-0 mt-0.5" />
            <div className="text-xs">
              <p className="text-[var(--warning)] font-medium mb-1">Security Notice</p>
              <p className="text-[var(--text-muted)] leading-relaxed">
                Backup files contain sensitive data (API keys, tokens). Store them securely and never share publicly.
                When exporting with "masked" option, tokens and API keys will be redacted.
              </p>
            </div>
          </div>
        </div>

        {/* Error Display */}
        {error && (
          <div className="glass-sm p-4 border-[var(--danger)]/20 animate-slide-up">
            <div className="flex items-center gap-2 text-[var(--danger)] text-xs font-medium">
              <AlertTriangle size={14} />
              {error}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
