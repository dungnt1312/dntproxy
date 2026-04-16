# Dashboard Redesign Proposal
**Date:** 2026-04-16  
**Status:** Proposal  
**Goal:** Add per-connection visibility, real-time activity tracking, and usage charts similar to 9router

---

## Problem Statement

Current dashboard lacks visibility into:
1. **Which connection is handling requests** - no per-connection activity tracking
2. **Request routing flow** - can't see which connection served which request
3. **Connection-level usage breakdown** - only provider-level aggregates shown
4. **Real-time activity indicators** - no live status for connections

---

## Proposed Solution

### 1. Connection Activity Panel (Top Priority)

**Location:** Top section of dashboard, above stats cards

**Features:**
- **Live connection cards** - one card per connection showing:
  - Connection name + provider logo
  - **Live status indicator**: 
    - 🟢 Green pulse = actively processing (request in last 30s)
    - 🟡 Yellow = recently used (request in last 5min)
    - ⚪ Gray = idle (no recent requests)
    - 🔴 Red = error/rate-limited
  - **Request count** in selected time range (default 1h)
  - **Last used timestamp** (e.g., "2m ago", "just now")
  - **Success rate** mini-indicator (e.g., "98%")
  - **Token usage** (e.g., "12.5K tokens")
  - **Cooldown status** if rate-limited (e.g., "Cooldown: 45s")

**Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│ Active Connections                                          │
├─────────────────────────────────────────────────────────────┤
│ ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│ │🟢 Kiro Main  │  │🟡 OpenAI Pro │  │⚪ Kiro Backup│      │
│ │ 45 requests  │  │ 12 requests  │  │ 0 requests   │      │
│ │ Just now     │  │ 3m ago       │  │ 2h ago       │      │
│ │ 98% success  │  │ 100% success │  │ -            │      │
│ │ 8.2K tokens  │  │ 2.1K tokens  │  │ -            │      │
│ └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

**Data Source:**
- Use existing `LogConnectionSummary` API with `range=1h` filter
- Calculate "last used" from most recent `LogEntry.timestamp` per connection
- Determine live status from `timestamp` age:
  - < 30s = active (green pulse)
  - < 5min = recent (yellow)
  - else = idle (gray)
- Show cooldown from `ProviderConnection.rateLimitedUntil`

---

### 2. Per-Connection Usage Charts

**Location:** Replace current "Requests by Provider" pie chart

**Chart 1: Requests by Connection (Bar Chart)**
- X-axis: Connection names
- Y-axis: Request count
- Color-coded by provider
- Show last 24h by default
- Click to filter logs by connection

**Chart 2: Token Usage by Connection (Stacked Bar)**
- X-axis: Connection names
- Y-axis: Token count
- Stacked: input tokens (blue) + output tokens (orange)
- Show cost estimate below each bar

**Chart 3: Success Rate by Connection (Horizontal Bar)**
- X-axis: Success rate (0-100%)
- Y-axis: Connection names
- Color gradient: red (low) → yellow (medium) → green (high)
- Show error count as annotation

**Data Source:**
- Use `GET /api/logs/connections?range=24h` (already implemented)
- Returns `LogConnectionSummary[]` with per-connection aggregates

---

### 3. Real-time Request Flow (Optional Enhancement)

**Location:** New collapsible section below charts

**Features:**
- **Live request stream** - show last 10 requests with:
  - Timestamp
  - Model requested
  - Connection used (with provider logo)
  - Status (success/error)
  - Duration
  - Tokens used
- **Auto-scroll** when new requests arrive
- **SSE-powered** - reuse existing `/api/logs/stream` endpoint

**Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│ ▼ Live Request Stream                          [Pause] [▶]  │
├─────────────────────────────────────────────────────────────┤
│ 14:32:45  kr/sonnet-4.5  →  🟢 Kiro Main   ✓ 1.2s  850 tok │
│ 14:32:43  oai/gpt-4o     →  🟡 OpenAI Pro  ✓ 0.8s  420 tok │
│ 14:32:40  kr/haiku-4.5   →  🟢 Kiro Main   ✓ 0.5s  120 tok │
│ 14:32:38  kr/sonnet-4.5  →  🔴 Kiro Backup ✗ Error         │
└─────────────────────────────────────────────────────────────┘
```

---

### 4. Enhanced Connection Health Table

**Location:** Keep existing table, add columns

**New Columns:**
- **Requests (1h)** - request count in last hour
- **Last Used** - relative timestamp (e.g., "2m ago")
- **Tokens (24h)** - total tokens in last 24h
- **Cost (24h)** - estimated cost
- **Avg Latency** - average response time

**Sortable** by any column

---

## Implementation Plan

### Phase 1: Backend API Enhancements (if needed)
- ✅ `GET /api/logs/connections` - already exists
- ✅ `GET /api/logs/stream` - already exists with SSE
- ⚠️ May need to add `lastUsedAt` field to connection summary
- ⚠️ May need to add `avgLatencyMs` to connection summary

### Phase 2: Frontend Components
1. **ConnectionActivityCard** component
   - Props: `connection`, `summary`, `lastUsedAt`, `status`
   - Live status indicator with pulse animation
   - Click to filter logs by connection

2. **ConnectionUsageCharts** component
   - Recharts bar/stacked bar charts
   - Data from `getLogConnections()` API
   - Interactive - click to drill down

3. **LiveRequestStream** component (optional)
   - SSE subscription to `/api/logs/stream`
   - Auto-scroll with pause/resume
   - Collapsible section

4. **Enhanced ConnectionHealthTable**
   - Add new columns
   - Sortable headers
   - Click row to view connection details

### Phase 3: Dashboard Layout Refactor
- Reorganize dashboard-screen.tsx:
  ```
  [Connection Activity Panel]
  [Stats Cards Row]
  [Charts Row: Requests by Hour | Connection Usage Charts]
  [Connection Health Table]
  [Live Request Stream] (collapsible)
  [Recent Errors]
  ```

---

## Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Dashboard Component                                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  useEffect(() => {                                          │
│    // Initial load                                          │
│    const [connections, logConnections, logSummary] =        │
│      await Promise.all([                                    │
│        goApi.getConnections(),        // connection status  │
│        goApi.getLogConnections({      // per-conn usage     │
│          range: '1h'                                        │
│        }),                                                  │
│        goApi.getLogSummary()          // overall stats      │
│      ])                                                     │
│                                                             │
│    // Merge data                                            │
│    const enriched = connections.map(conn => ({              │
│      ...conn,                                               │
│      usage: logConnections.find(lc =>                       │
│        lc.connectionId === conn.id                          │
│      ),                                                     │
│      lastUsedAt: calculateLastUsed(conn.id, logs)           │
│    }))                                                      │
│                                                             │
│    // Auto-refresh every 30s                                │
│    setInterval(fetchData, 30000)                            │
│  }, [])                                                     │
│                                                             │
│  // Optional: SSE for real-time updates                     │
│  useEffect(() => {                                          │
│    const eventSource = new EventSource(                     │
│      '/api/logs/stream?range=1h'                            │
│    )                                                        │
│    eventSource.onmessage = (e) => {                         │
│      const { type, log } = JSON.parse(e.data)              │
│      if (type === 'delta') {                                │
│        updateConnectionActivity(log.connectionId)           │
│      }                                                      │
│    }                                                        │
│    return () => eventSource.close()                         │
│  }, [])                                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## UI/UX Considerations

### Visual Hierarchy
1. **Connection Activity Panel** - most prominent (users want to see "what's happening now")
2. **Stats Cards** - quick overview metrics
3. **Charts** - deeper analysis
4. **Tables** - detailed data

### Color Coding
- **Green** - active, healthy, success
- **Yellow** - warning, recently used, moderate
- **Red** - error, rate-limited, critical
- **Gray** - idle, inactive, neutral

### Animations
- **Pulse effect** for active connections (green dot)
- **Fade in** for new requests in live stream
- **Smooth transitions** when data updates

### Responsive Design
- **Desktop**: 3-4 connection cards per row
- **Tablet**: 2 cards per row
- **Mobile**: 1 card per row, stack charts vertically

---

## Technical Considerations

### Performance
- **Debounce** auto-refresh to avoid excessive API calls
- **Virtualize** live request stream if > 100 entries
- **Memoize** chart data transformations
- **Lazy load** charts (only render when visible)

### Error Handling
- **Graceful degradation** if API fails (show cached data)
- **Retry logic** for SSE disconnections
- **Loading states** for each section independently

### Accessibility
- **ARIA labels** for status indicators
- **Keyboard navigation** for connection cards
- **Screen reader** announcements for live updates

---

## Success Metrics

After implementation, measure:
1. **User can identify active connection** within 3 seconds
2. **User can see request routing** for last 10 requests
3. **User can compare connection usage** via charts
4. **Dashboard loads** in < 2 seconds
5. **Real-time updates** appear within 1 second of event

---

## Open Questions

1. Should we add **connection filtering** to charts (e.g., show only Kiro connections)?
2. Should we add **time range selector** for connection activity panel?
3. Should we add **export to CSV** for connection usage data?
4. Should we add **alerts/notifications** when connection goes down?
5. Should we add **connection comparison view** (side-by-side)?

---

## Next Steps

1. **Review proposal** with stakeholders
2. **Prioritize features** (MVP vs. nice-to-have)
3. **Create wireframes/mockups** for visual design
4. **Estimate effort** (backend + frontend)
5. **Create implementation plan** with tasks
