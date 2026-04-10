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

function prettyJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
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

function statusChipClass(status?: number, level?: string) {
  if (level === 'ERROR' || (status && status >= 400)) return 'chip-danger'
  if (status && status >= 200 && status < 300) return 'chip-success'
  return 'chip-muted'
}

function directionColor(direction?: string) {
  if (direction === 'inbound') return 'var(--accent)'
  if (direction === 'outbound') return 'var(--purple)'
  if (direction === 'response') return 'var(--success)'
  if (direction === 'usage') return 'var(--warning)'
  return 'var(--text-dim)'
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
    <article className={`glass-sm relative overflow-hidden ${log.level === 'ERROR' ? 'border-[var(--danger)]/20' : ''}`}>
      {/* Direction indicator bar */}
      <div className="absolute left-0 top-0 bottom-0 w-[3px] rounded-l-xl" style={{ backgroundColor: directionColor(log.direction) }} />

      <div className="p-3 pl-4">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="font-semibold text-sm" style={{ fontFamily: 'var(--font-heading)' }}>{titleFor(log)}</span>
              <span className="text-[10px] text-[var(--text-dim)] font-mono">{formatTime(log.timestamp)}</span>
              <span className={`chip text-[10px] ${statusChipClass(log.statusCode, log.level)}`}>
                {log.statusCode || log.level}
              </span>
              <span className="chip chip-muted text-[10px]">
                {log.provider} / {log.direction}
              </span>
            </div>

            <div className="mt-1.5 grid gap-1 text-[11px] text-[var(--text-dim)] lg:grid-cols-2">
              <div className="truncate">Conn: {log.connectionName || log.connectionId || '-'}</div>
              <div className="truncate">Model: {log.model || '-'}</div>
            </div>
          </div>

          {metrics.length > 0 && (
            <div className="flex flex-wrap gap-1 lg:justify-end">
              {metrics.map(item => (
                <span key={item} className="chip chip-muted text-[10px] font-mono">
                  {item}
                </span>
              ))}
            </div>
          )}
        </div>

        {preview && (
          <div className="mt-3 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-3">
            <div className="mb-1.5 text-[10px] uppercase font-bold tracking-wider text-[var(--text-dim)]">
              {metadata.responsePreview ? 'Response payload' : 'Summary'}
              {metadata.truncated ? ' (truncated)' : ''}
            </div>
            <pre className="max-h-44 overflow-auto whitespace-pre-wrap break-words text-xs leading-5 text-[var(--text-muted)]">{preview}</pre>
          </div>
        )}

        <details className="mt-3 text-xs text-[var(--text-dim)]">
          <summary className="cursor-pointer select-none hover:text-[var(--text-muted)] transition-colors">Details</summary>
          <dl className="mt-2 grid gap-2 lg:grid-cols-2 text-[11px]">
            <div className="truncate">Request ID: <span className="font-mono text-[var(--text-muted)]">{log.requestId || '-'}</span></div>
            <div className="truncate">Path: <span className="font-mono text-[var(--text-muted)]">{log.path || '-'}</span></div>
            <div>Message: {log.message || '-'}</div>
            <div>Usage source: {log.usageSource || metadata.source || '-'}</div>
            {log.error && <div className="lg:col-span-2 text-[var(--danger)]">Error: {log.error}</div>}
          </dl>

          {log.requestBody && (
            <details className="mt-3">
              <summary className="cursor-pointer select-none font-semibold text-[var(--text-muted)] hover:text-[var(--text)] transition-colors">Request Body</summary>
              <pre className="mt-2 max-h-96 overflow-auto rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-3 whitespace-pre-wrap break-all leading-5 text-[var(--text-muted)] font-mono text-[10px]">
                {prettyJSON(log.requestBody)}
              </pre>
            </details>
          )}

          {log.responseBody && (
            <details className="mt-2">
              <summary className="cursor-pointer select-none font-semibold text-[var(--text-muted)] hover:text-[var(--text)] transition-colors">Response Body</summary>
              <pre className="mt-2 max-h-96 overflow-auto rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-3 whitespace-pre-wrap break-all leading-5 text-[var(--text-muted)] font-mono text-[10px]">
                {prettyJSON(log.responseBody)}
              </pre>
            </details>
          )}
        </details>
      </div>
    </article>
  )
}

export default function LogTable({ logs }: LogTableProps) {
  const safeLogs = Array.isArray(logs) ? logs : []

  if (safeLogs.length === 0) {
    return (
      <div className="flex h-80 items-center justify-center glass text-[var(--text-dim)]">
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
