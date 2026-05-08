import { useState } from 'react'
import { Download, Upload, AlertTriangle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Label } from '@/components/ui/label'

interface BackupStats {
  connections: number
  combos: number
  aliases: number
  apiKeys: number
}

function toJsonBlob(data: unknown) {
  return new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
}

export default function BackupScreen() {
  const [loading, setLoading] = useState(false)
  const [importMode, setImportMode] = useState<'merge' | 'replace'>('merge')
  const [lastExport, setLastExport] = useState<BackupStats | null>(null)
  const [lastImport, setLastImport] = useState<{ imported: number; skipped: number; mode: string } | null>(null)

  async function handleExport(mask: boolean) {
    setLoading(true)
    setLastImport(null)
    try {
      const data = await goApi.exportBackup(mask)

      const blob = toJsonBlob(data)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `dntproxy-backup-${new Date().toISOString().slice(0, 19).replace(/[:-]/g, '')}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)

      setLastExport({
        connections: Array.isArray((data as any)?.providerConnections) ? (data as any).providerConnections.length : 0,
        combos: Array.isArray((data as any)?.combos) ? (data as any).combos.length : 0,
        aliases: Object.keys((data as any)?.modelAliases || {}).length,
        apiKeys: Array.isArray((data as any)?.apiKeys) ? (data as any).apiKeys.length : 0,
      })

      toast.success(mask ? 'Masked backup exported' : 'Backup exported')
    } catch {
      toast.error('Export failed')
    } finally {
      setLoading(false)
    }
  }

  async function handleImportFile(file: File) {
    setLoading(true)
    setLastExport(null)
    try {
      const text = await file.text()
      const parsed = JSON.parse(text)
      const result = await goApi.importBackup(parsed, importMode)

      setLastImport({
        imported: Number((result as any)?.imported || 0),
        skipped: Number((result as any)?.skipped || 0),
        mode: String((result as any)?.mode || importMode),
      })
      toast.success('Backup imported')
    } catch {
      toast.error('Import failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Backup & Restore</h1>
        <p className="text-sm text-muted-foreground">Export full configuration and restore later on the same or another instance.</p>
      </div>

      <div className="rounded-lg border p-4 space-y-3">
        <h2 className="text-sm font-semibold">Export</h2>
        <p className="text-xs text-muted-foreground">Download current settings, connections, combos, aliases, and keys.</p>
        <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
          <Button
            onClick={() => handleExport(false)}
            disabled={loading}
            className="h-9"
          >
            {loading ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : <Download className="mr-1 h-4 w-4" />}
            Export
          </Button>
          <Button
            onClick={() => handleExport(true)}
            disabled={loading}
            variant="outline"
            className="h-9"
          >
            <Download className="mr-1 h-4 w-4" /> Export (masked)
          </Button>
        </div>

        {lastExport && (
          <div className="rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
            {lastExport.connections} connections · {lastExport.combos} combos · {lastExport.aliases} aliases · {lastExport.apiKeys} keys
          </div>
        )}
      </div>

      <div className="rounded-lg border p-4 space-y-3">
        <h2 className="text-sm font-semibold">Import</h2>
        <p className="text-xs text-muted-foreground">Upload a `.json` backup and choose merge or replace mode.</p>

        <RadioGroup
          value={importMode}
          onValueChange={(value) => setImportMode(value as 'merge' | 'replace')}
          className="flex flex-row items-center gap-4"
        >
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="merge" id="import-merge" />
            <Label htmlFor="import-merge">Merge</Label>
          </div>
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="replace" id="import-replace" />
            <Label htmlFor="import-replace">Replace</Label>
          </div>
        </RadioGroup>

        <label className="inline-flex h-9 cursor-pointer items-center rounded-md border px-3 text-sm hover:bg-accent">
          <Upload className="mr-1 h-4 w-4" /> Select backup file
          <input
            type="file"
            accept=".json"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) handleImportFile(file)
              e.currentTarget.value = ''
            }}
          />
        </label>

        {lastImport && (
          <div className="rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
            Imported {lastImport.imported}, skipped {lastImport.skipped}, mode {lastImport.mode}
          </div>
        )}
      </div>

      <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-4 text-xs text-muted-foreground">
        <div className="mb-1 flex items-center gap-2 text-amber-700 dark:text-amber-400">
          <AlertTriangle className="h-4 w-4" /> Security Note
        </div>
        Backup files may contain sensitive credentials. Store them securely and do not commit them.
      </div>
    </div>
  )
}
