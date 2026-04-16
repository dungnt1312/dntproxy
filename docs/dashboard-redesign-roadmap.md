# Dashboard Redesign - Implementation Roadmap

**Created:** 2026-04-16  
**Status:** Planning  
**Estimated Duration:** 10-15 days  

---

## Overview

This roadmap breaks down the dashboard redesign into actionable tasks with clear dependencies and estimates.

---

## Phase 1: MVP - Connection Activity Panel (Days 1-3)

### Backend Tasks

#### Task 1.1: Enhance LogConnectionSummary API
**File:** `internal/adapter/storage/sqlite-log-store-query.go`  
**Estimate:** 2 hours  
**Description:** Add `lastUsedAt` and `avgLatencyMs` to connection summaries

```go
// Add to ConnectionSummaries query
SELECT 
  COALESCE(connection_id, ''), 
  COALESCE(connection_name, ''), 
  provider,
  COUNT(CASE WHEN direction = 'response' THEN 1 END),
  COUNT(CASE WHEN level = 'ERROR' THEN 1 END),
  COALESCE(SUM(total_tokens), 0),
  COALESCE(SUM(cost_total), 0),
  MAX(timestamp_ms) as last_used_ms,  -- NEW
  AVG(duration_ms) as avg_latency_ms  -- NEW
FROM request_logs
WHERE timestamp_ms >= ?
GROUP BY connection_id, connection_name, provider
```

**Changes:**
- Update `domain.LogConnectionSummary` struct
- Update `scanConnectionSummary()` function
- Update API response in `log-handler.go`

#### Task 1.2: Add Connection Status Calculation
**File:** `internal/service/connection-status-calculator.go` (new)  
**Estimate:** 3 hours  
**Description:** Calculate live connection status based on last activity

```go
type ConnectionStatus string

const (
    StatusActive      ConnectionStatus = "active"       // < 30s
    StatusRecent      ConnectionStatus = "recent"       // < 5min
    StatusIdle        ConnectionStatus = "idle"         // > 5min
    StatusError       ConnectionStatus = "error"        // has error
    StatusRateLimited ConnectionStatus = "rate_limited" // cooldown active
)

func CalculateStatus(conn *domain.ProviderConnection, lastUsedMs int64) ConnectionStatus {
    now := time.Now().UnixMilli()
    
    // Check rate limit first
    if conn.RateLimitedUntil != nil && conn.RateLimitedUntil.After(time.Now()) {
        return StatusRateLimited
    }
    
    // Check error state
    if conn.LastError != "" {
        return StatusError
    }
    
    // Check activity
    if lastUsedMs == 0 {
        return StatusIdle
    }
    
    ageMs := now - lastUsedMs
    if ageMs < 30000 { // 30 seconds
        return StatusActive
    }
    if ageMs < 300000 { // 5 minutes
        return StatusRecent
    }
    
    return StatusIdle
}
```

### Frontend Tasks

#### Task 1.3: Create ConnectionActivityCard Component
**File:** `ui/src/components/dashboard/ConnectionActivityCard.tsx` (new)  
**Estimate:** 4 hours  
**Description:** Reusable card component for connection activity

```tsx
interface ConnectionActivityCardProps {
  connection: Connection
  summary: LogConnectionSummary
  status: 'active' | 'recent' | 'idle' | 'error' | 'rate_limited'
  onClick?: () => void
}

export function ConnectionActivityCard({ 
  connection, 
  summary, 
  status,
  onClick 
}: ConnectionActivityCardProps) {
  const statusConfig = {
    active: { color: 'emerald', icon: '🟢', label: 'Active', pulse: true },
    recent: { color: 'amber', icon: '🟡', label: 'Recent', pulse: false },
    idle: { color: 'gray', icon: '⚪', label: 'Idle', pulse: false },
    error: { color: 'red', icon: '🔴', label: 'Error', pulse: false },
    rate_limited: { color: 'orange', icon: '🔴', label: 'Rate Limited', pulse: false },
  }
  
  const config = statusConfig[status]
  const lastUsed = formatRelativeTime(summary.lastUsedAt)
  const successRate = summary.requests > 0 
    ? ((summary.requests - summary.errors) / summary.requests * 100).toFixed(0)
    : 0
  
  return (
    <Card 
      className="cursor-pointer hover:shadow-lg transition-all"
      onClick={onClick}
    >
      <CardContent className="p-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <div className={cn(
              "w-3 h-3 rounded-full",
              `bg-${config.color}-500`,
              config.pulse && "animate-pulse"
            )} />
            <h3 className="font-semibold">{connection.name}</h3>
          </div>
          <Badge variant="outline">{config.label}</Badge>
        </div>
        
        <div className="space-y-2 text-sm">
          <div className="flex items-center gap-2">
            <Activity className="w-4 h-4 text-muted-foreground" />
            <span>{summary.requests} requests</span>
          </div>
          
          <div className="flex items-center gap-2">
            <Clock className="w-4 h-4 text-muted-foreground" />
            <span>{lastUsed}</span>
          </div>
          
          <div className="flex items-center gap-2">
            <CheckCircle className="w-4 h-4 text-muted-foreground" />
            <span>{successRate}% success</span>
          </div>
          
          <div className="flex items-center gap-2">
            <Zap className="w-4 h-4 text-muted-foreground" />
            <span>{formatTokens(summary.totalTokens)} tokens</span>
          </div>
          
          <div className="flex items-center gap-2">
            <DollarSign className="w-4 h-4 text-muted-foreground" />
            <span>${summary.costTotal.toFixed(4)}</span>
          </div>
          
          {status === 'rate_limited' && connection.rateLimitedUntil && (
            <div className="flex items-center gap-2 text-orange-600">
              <AlertTriangle className="w-4 h-4" />
              <span>Cooldown: {formatCooldown(connection.rateLimitedUntil)}</span>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
```

#### Task 1.4: Create ConnectionActivityPanel Component
**File:** `ui/src/components/dashboard/ConnectionActivityPanel.tsx` (new)  
**Estimate:** 2 hours  
**Description:** Container for connection activity cards

```tsx
interface ConnectionActivityPanelProps {
  connections: Connection[]
  summaries: LogConnectionSummary[]
  onConnectionClick?: (connectionId: string) => void
}

export function ConnectionActivityPanel({
  connections,
  summaries,
  onConnectionClick
}: ConnectionActivityPanelProps) {
  const enrichedConnections = connections.map(conn => {
    const summary = summaries.find(s => s.connectionId === conn.id)
    const status = calculateStatus(conn, summary?.lastUsedAt)
    
    return { connection: conn, summary, status }
  })
  
  // Sort: active > recent > idle > error
  const sorted = enrichedConnections.sort((a, b) => {
    const order = { active: 0, recent: 1, idle: 2, error: 3, rate_limited: 4 }
    return order[a.status] - order[b.status]
  })
  
  return (
    <Card className="border-2 border-primary/20">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Activity className="w-5 h-5 text-primary" />
            Active Connections (Last 1 hour)
          </CardTitle>
          <Button variant="outline" size="sm">
            <RefreshCw className="w-4 h-4" />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {sorted.map(({ connection, summary, status }) => (
            <ConnectionActivityCard
              key={connection.id}
              connection={connection}
              summary={summary || createEmptySummary()}
              status={status}
              onClick={() => onConnectionClick?.(connection.id)}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
```

#### Task 1.5: Integrate into Dashboard
**File:** `ui/src/components/screens/dashboard-screen.tsx`  
**Estimate:** 2 hours  
**Description:** Add connection activity panel to dashboard

```tsx
export default function DashboardScreen() {
  const [connections, setConnections] = useState<Connection[]>([])
  const [logSummaries, setLogSummaries] = useState<LogConnectionSummary[]>([])
  
  useEffect(() => {
    async function fetchData() {
      const [conns, s
