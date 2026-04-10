import type { LogEntry } from '../types/logs'

interface LogTableProps {
  logs: LogEntry[]
}

interface LogMetadata {
  responsePreview?: string
  truncated?: boolean
  source?: string
}

function parseMetadata(value?: string): LogMetadata {
  if (!value) return {}
  try {
    const parsed = JSON.parse(value) as Record<string, unknown>
    return {
      responsePreview: typeof parsed.responsePreview === 'string' ? parsed.responsePreview : undefined,
      truncated: typeof parsed.truncated === 'boolean' ? parsed.truncated : false,
      source: typeof parsed.source === 'string' ? parsed.source : undefined,
    }
  } catch {
    return {}
  }
}

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value || '-'
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function formatCost(value = 0, currency = 'USD') {
  if (!value) return '-'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    maximumFractionDigits: 4,
  }).format(value)
}

function statusClass(status?: number, level?: string) {
  if (level === 'ERROR' || (status && status >= 400)) return 'border-red-500/30 bg-red-500/10 text-[var(--danger)]'
  if (status && status >= 200 && status < 300) return 'border-emerald-500/30 bg-emerald-500/10 text-[var(--success)]'
  return 'border-[var(--border)] bg-[var(--bg-hover)] text-[var(--text-muted)]'
}

function titleFor(log: LogEntry) {
  if (log.direction === 'inbound') return 'Client request'
  if (log.direction === 'outbound') return 'Provider request'
  if (log.direction === 'response') return log.level === 'ERROR' ? 'Provider error response' : 'Provider response'
  if (log.direction === 'usage') return 'Usage captured'
  if (log.direction === 'payload') return 'Response payload'
  return 'System event'
}

function previewFor(log: LogEntry, metadata: LogMetadata) {
  if (metadata.responsePreview) return metadata.responsePreview
  if (log.error) return log.error
  if (log.direction === 'usage') {
    const input = log.inputTokens || 0
    const output = log.outputTokens || 0
    return `${input.toLocaleString()} input + ${output.toLocaleString()} output tokens`
  }
  return log.message
}

function metricLine(log: LogEntry) {
  const parts: string[] = []
  if (log.durationMs) parts.push(`${log.durationMs}ms`)
  if (log.bodySize) parts.push(`${log.bodySize.toLocaleString()}B`)
  if (log.totalTokens) parts.push(`${log.totalTokens.toLocaleString()} tok`)
  if (log.costTotal) parts.push(formatCost(log.costTotal, log.currency || 'USD'))
  return parts
}

function LogEvent({ log }: { log: LogEntry }) {
  const metadata = parseMetadata(log.metadataJson)
  const preview = previewFor(log, metadata)
  const metrics = metricLine(log)

  return (
    <article className={`border border-[var(--border)] bg-[var(--bg-card)] rounded-lg p-3 ${log.level === 'ERROR' ? 'border-red-500/40' : ''}`}>
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold">{titleFor(log)}</span>
            <span className="text-xs text-[var(--text-muted)] font-mono">{formatTime(log.timestamp)}</span>
            <span className={`text-xs border rounded px-2 py-0.5 font-mono ${statusClass(log.statusCode, log.level)}`}>
              {log.statusCode || log.level}
            </span>
            <span className="text-xs border border-[var(--border)] rounded px-2 py-0.5 text-[var(--text-muted)]">
              {log.provider} / {log.direction}
            </span>
          </div>

          <div className="mt-2 grid gap-1 text-xs text-[var(--text-muted)] lg:grid-cols-2">
            <div className="truncate">Connection: {log.connectionName || log.connectionId || '-'}</div>
            <div className="truncate">Model: {log.model || '-'}</div>
          </div>
        </div>

        {metrics.length > 0 && (
          <div className="flex flex-wrap gap-1 lg:justify-end">
            {metrics.map(item => (
              <span key={item} className="text-xs font-mono border border-[var(--border)] rounded px-2 py-1 text-[var(--text-muted)]">
                {item}
              </span>
            ))}
          </div>
        )}
      </div>

      {preview && (
        <div className="mt-3 rounded border border-[var(--border)] bg-[var(--bg)] p-3">
          <div className="mb-2 text-xs uppercase text-[var(--text-muted)]">
            {metadata.responsePreview ? 'Response payload' : 'Summary'}
            {metadata.truncated ? ' (truncated)' : ''}
          </div>
          <pre className="max-h-44 overflow-auto whitespace-pre-wrap break-words text-xs leading-5 text-[var(--text-primary)]">{preview}</pre>
        </div>
      )}

      <details className="mt-3 text-xs text-[var(--text-muted)]">
        <summary className="cursor-pointer select-none">Details</summary>
        <dl className="mt-2 grid gap-2 lg:grid-cols-2">
          <div className="truncate">Request ID: <span className="font-mono">{log.requestId || '-'}</span></div>
          <div className="truncate">Path: <span className="font-mono">{log.path || '-'}</span></div>
          <div>Message: {log.message || '-'}</div>
          <div>Usage source: {log.usageSource || metadata.source || '-'}</div>
          {log.error && <div className="lg:col-span-2 text-[var(--danger)]">Error: {log.error}</div>}
        </dl>
      </details>
    </article>
  )
}

export default function LogTable({ logs }: LogTableProps) {
  const safeLogs = Array.isArray(logs) ? logs : []

  if (safeLogs.length === 0) {
    return (
      <div className="flex h-80 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--bg-card)] text-[var(--text-muted)]">
        No logs for this filter.
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {safeLogs.map(log => <LogEvent key={log.id} log={log} />)}
    </div>
  )
}
