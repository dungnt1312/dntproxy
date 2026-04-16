# Dashboard Redesign - Visual Mockup

## Current Dashboard (Before)

```
┌─────────────────────────────────────────────────────────────────────┐
│ Dashboard                                                           │
│ Overview of your proxy server performance                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│ │ Activity │  │ Success  │  │   Clock  │  │   Link   │          │
│ │  1,234   │  │  98.5%   │  │  450ms   │  │    3     │          │
│ │ Requests │  │ Success  │  │ Latency  │  │ Active   │          │
│ └──────────┘  └──────────┘  └──────────┘  └──────────┘          │
│                                                                     │
│ ┌─────────────────────────────┐  ┌──────────────────────┐        │
│ │ Requests by Hour (24h)      │  │ Requests by Provider │        │
│ │                             │  │                      │        │
│ │     ▂▄▆█▆▄▂                 │  │   ◉ Kiro    45%     │        │
│ │                             │  │   ◉ OpenAI  35%     │        │
│ │                             │  │   ◉ GLM     20%     │        │
│ └─────────────────────────────┘  └──────────────────────┘        │
│                                                                     │
│ ┌─────────────────────────────────────────────────────────────────┐│
│ │ Connection Health                                               ││
│ │ Name         Provider   Status    Models              Error    ││
│ │ Kiro Main    kiro       Active    Restricted list     None     ││
│ │ OpenAI Pro   openai     Idle      All supported       None     ││
│ └─────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────┘
```

**Problems:**
- ❌ Can't see which connection is handling requests
- ❌ No per-connection usage breakdown
- ❌ No real-time activity indicators
- ❌ Charts show provider-level only, not connection-level

---

## New Dashboard (After)

```
┌─────────────────────────────────────────────────────────────────────┐
│ Dashboard                                                           │
│ Overview of your proxy server performance                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓ │
│ ┃ 🔴 Active Connections (Last 1 hour)                  [Refresh] ┃ │
│ ┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫ │
│ ┃                                                                 ┃ │
│ ┃ ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐┃ │
│ ┃ │ 🟢 Kiro Main    │  │ 🟡 OpenAI Pro   │  │ ⚪ Kiro Backup  │┃ │
│ ┃ │ ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄ │  │ ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄ │  │ ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄ │┃ │
│ ┃ │ 🎯 45 requests  │  │ 🎯 12 requests  │  │ 🎯 0 requests   │┃ │
│ ┃ │ ⏱️  Just now     │  │ ⏱️  3m ago       │  │ ⏱️  2h ago      │┃ │
│ ┃ │ ✅ 98% success  │  │ ✅ 100% success │  │ ➖ No activity  │┃ │
│ ┃ │ 🪙 8.2K tokens  │  │ 🪙 2.1K tokens  │  │ 🪙 0 tokens     │┃ │
│ ┃ │ 💰 $0.12        │  │ 💰 $0.04        │  │ 💰 $0.00        │┃ │
│ ┃ └─────────────────┘  └─────────────────┘  └─────────────────┘┃ │
│ ┃                                                                 ┃ │
│ ┃ ┌─────────────────┐  ┌─────────────────┐                      ┃ │
│ ┃ │ 🔴 GLM API      │  │ 🟠 MiniMax Pro  │                      ┃ │
│ ┃ │ ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄ │  │ ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄ │                      ┃ │
│ ┃ │ 🎯 0 requests   │  │ 🎯 8 requests   │                      ┃ │
│ ┃ │ ⏱️  Never used  │  │ ⏱️  15m ago     │                      ┃ │
│ ┃ │ ⚠️  Rate Limited│  │ ✅ 100% success │                      ┃ │
│ ┃ │ ⏳ Cooldown: 2m │  │ 🪙 1.5K tokens  │                      ┃ │
│ ┃ │ 💰 $0.00        │  │ 💰 $0.02        │                      ┃ │
│ ┃ └─────────────────┘  └─────────────────┘                      ┃ │
│ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛ │
│                                                                     │
│ ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│ │ Activity │  │ Success  │  │   Clock  │  │   Link   │          │
│ │  1,234   │  │  98.5%   │  │  450ms   │  │  5 / 5   │          │
│ │ Requests │  │ Success  │  │ Latency  │  │ Active   │          │
│ └──────────┘  └──────────┘  └──────────┘  └──────────┘          │
│                                                                     │
│ ┌─────────────────────────────┐  ┌──────────────────────┐        │
│ │ Requests by Hour (24h)      │  │ Requests by Connection│       │
│ │                             │  │                      │        │
│ │     ▂▄▆█▆▄▂                 │  │ Kiro Main    ████████│        │
│ │                             │  │ OpenAI Pro   ████     │        │
│ │                             │  │ MiniMax Pro  ██       │        │
│ │                             │  │ Kiro Backup  ▌        │        │
│ └─────────────────────────────┘  └──────────────────────┘        │
│                                                                     │
│ ┌─────────────────────────────┐  ┌──────────────────────┐        │
│ │ Token Usage by Connection   │  │ Success Rate         │        │
│ │                             │  │                      │        │
│ │ Kiro Main    ████▓▓▓▓       │  │ OpenAI Pro   ████████│        │
│ │ OpenAI Pro   ██▓▓           │  │ Kiro Main    ███████▌│        │
│ │ MiniMax Pro  █▓             │  │ MiniMax Pro  ████████│        │
│ │              ▓ Input        │  │ Kiro Backup  ▌       │        │
│ │              █ Output       │  │              0%  100%│        │
│ └─────────────────────────────┘  └──────────────────────┘        │
│                                                                     │
│ ┌─────────────────────────────────────────────────────────────────┐│
│ │ ▼ Live Request Stream                      [Pause] [Clear]     ││
│ ├─────────────────────────────────────────────────────────────────┤│
│ │ 14:32:45  kr/sonnet-4.5  →  🟢 Kiro Main   ✓ 1.2s  850 tok    ││
│ │ 14:32:43  oai/gpt-4o     →  🟡 OpenAI Pro  ✓ 0.8s  420 tok    ││
│ │ 14:32:40  kr/haiku-4.5   →  🟢 Kiro Main   ✓ 0.5s  120 tok    ││
│ │ 14:32:38  kr/sonnet-4.5  →  🔴 Kiro Backup ✗ Rate limit       ││
│ │ 14:32:35  glm/glm-4.6    →  🟠 GLM API     ✓ 2.1s  650 tok    ││
│ └─────────────────────────────────────────────────────────────────┘│
│                                                                     │
│ ┌─────────────────────────────────────────────────────────────────┐│
│ │ Connection Health                                               ││
│ │ Name         Provider  Status  Req(1h) Last    Tokens  Cost    ││
│ │ Kiro Main    kiro      🟢 Active  45    Just    8.2K   $0.12   ││
│ │ OpenAI Pro   openai    🟡 Recent  12    3m ago  2.1K   $0.04   ││
│ │ MiniMax Pro  minimax   🟡 Recent   8    15m     1.5K   $0.02   ││
│ │ Kiro Backup  kiro      ⚪ Idle     0    2h ago  0      $0.00   ││
│ │ GLM API      glm       🔴 Limited  0    Never   0      $0.00   ││
│ └─────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────┘
```

**Improvements:**
- ✅ **Connection Activity Panel** - see which connections are active at a glance
- ✅ **Live status indicators** - 🟢 active, 🟡 recent, ⚪ idle, 🔴 error
- ✅ **Per-connection metrics** - requests, last used, success rate, tokens, cost
- ✅ **Connection-level charts** - requests, tokens, success rate by connection
- ✅ **Live request stream** - see real-time request routing
- ✅ **Enhanced health table** - more actionable data

---

## Key Features Explained

### 1. Connection Activity Cards

**Status Indicators:**
- 🟢 **Green pulse** = Active (request within last 30 seconds)
- 🟡 **Yellow** = Recently used (request within last 5 minutes)
- ⚪ **Gray** = Idle (no recent requests)
- 🔴 **Red** = Error or rate-limited

**Metrics Shown:**
- 🎯 **Request count** in selected time range (default: 1 hour)
- ⏱️ **Last used** timestamp (relative: "just now", "3m ago", "2h ago")
- ✅ **Success rate** percentage
- 🪙 **Token usage** (input + output combined)
- 💰 **Estimated cost** based on pricing profiles
- ⏳ **Cooldown timer** if rate-limited (e.g., "Cooldown: 2m 15s")

**Interactions:**
- **Click card** → filter logs by this connection
- **Hover** → show tooltip with detailed stats
- **Right-click** → quick actions (test, reset cooldown, view details)

### 2. Per-Connection Usage Charts

**Chart 1: Requests by Connection (Bar Chart)**
- Shows request distribution across connections
- Color-coded by provider (Kiro = red, OpenAI = green, etc.)
- Click bar to filter logs by connection
- Tooltip shows exact count + percentage

**Chart 2: Token Usage by Connection (Stacked Bar)**
- Blue = input tokens
- Orange = output tokens
- Shows cost estimate below each bar
- Helps identify expensive connections

**Chart 3: Success Rate by Connection (Horizontal Bar)**
- Green gradient = high success rate (95-100%)
- Yellow gradient = medium (80-95%)
- Red gradient = low (<80%)
- Shows error count as annotation

### 3. Live Request Stream

**Real-time request flow:**
- Auto-updates via SSE (Server-Sent Events)
- Shows last 10-20 requests
- Format: `timestamp  model  →  connection  status  duration  tokens`
- Color-coded status: ✓ success (green), ✗ error (red)
- Auto-scroll with pause/resume button
- Collapsible to save space

**Use cases:**
- Debug routing issues
- Monitor which connection is handling requests
- See request patterns in real-time
- Identify performance bottlenecks

### 4. Enhanced Connection Health Table

**New columns:**
- **Req (1h)** - request count in last hour (sortable)
- **Last Used** - relative timestamp (sortable)
- **Tokens (24h)** - total tokens in last 24 hours
- **Cost (24h)** - estimated cost
- **Avg Latency** - average response time

**Interactions:**
- **Sort by any column** - click header to sort
- **Click row** - view connection details
- **Status indicator** - live status with color coding

---

## Responsive Design

### Desktop (1920x1080)
- 4 connection cards per row
- Charts side-by-side (2 columns)
- Full table visible

### Tablet (768x1024)
- 2 connection cards per row
- Charts stacked vertically
- Table scrollable horizontally

### Mobile (375x667)
- 1 connection card per row
- Charts stacked vertically
- Table simplified (fewer columns)
- Live stream hidden by default

---

## Color Palette

### Status Colors
- 🟢 **Active Green**: `#10b981` (emerald-500)
- 🟡 **Recent Yellow**: `#f59e0b` (amber-500)
- ⚪ **Idle Gray**: `#6b7280` (gray-500)
- 🔴 **Error Red**: `#ef4444` (red-500)

### Provider Colors
- **Kiro**: `#ef4444` (red-500)
- **OpenAI**: `#10b981` (emerald-500)
- **Anthropic**: `#f59e0b` (amber-500)
- **GLM**: `#8b5cf6` (violet-500)
- **MiniMax**: `#ec4899` (pink-500)
- **Qwen**: `#3b82f6` (blue-500)

### Chart Colors
- **Input tokens**: `#3b82f6` (blue-500)
- **Output tokens**: `#f97316` (orange-500)
- **Success**: `#10b981` (emerald-500)
- **Error**: `#ef4444` (red-500)

---

## Animation & Transitions

### Pulse Animation (Active Status)
```css
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.status-active {
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
```

### Card Hover Effect
```css
.connection-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
  transition: all 0.2s ease;
}
```

### Live Stream Entry Animation
```css
@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateX(-20px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

.request-entry {
  animation: slideIn 0.3s ease;
}
```

---

## Accessibility

### ARIA Labels
- Connection cards: `aria-label="Kiro Main connection, active, 45 requests"`
- Status indicators: `aria-label="Active status"`
- Charts: `role="img"` with descriptive `aria-label`

### Keyboard Navigation
- Tab through connection cards
- Enter to select/filter
- Arrow keys to navigate table
- Escape to close modals

### Screen Reader Support
- Announce live updates: `aria-live="polite"`
- Descriptive button labels
- Table headers properly associated

---

## Performance Optimizations

### Data Fetching
- **Initial load**: Parallel fetch (connections + logs + summary)
- **Auto-refresh**: Every 30 seconds (configurable)
- **SSE**: Only for live stream (optional feature)
- **Debounce**: User interactions (filter, sort)

### Rendering
- **Memoize** chart data transformations
- **Virtualize** live stream if > 100 entries
- **Lazy load** charts (only render when visible)
- **Throttle** SSE updates (max 1 update per second)

### Caching
- Cache connection list for 30s
- Cache log summaries for 10s
- Invalidate on user action (refresh, filter)

---

## Implementation Complexity

### Easy (1-2 days)
- ✅ Connection Activity Cards (basic version)
- ✅ Enhanced Connection Health Table
- ✅ Per-Connection Bar Chart

### Medium (3-5 days)
- ⚠️ Live status calculation (last used, active detection)
- ⚠️ Token Usage Stacked Bar Chart
- ⚠️ Success Rate Horizontal Bar Chart
- ⚠️ Auto-refresh logic

### Hard (5-7 days)
- 🔴 Live Request Stream with SSE
- 🔴 Real-time status updates
- 🔴 Advanced filtering/sorting
- 🔴 Responsive design polish

**Total Estimate:** 10-15 days for full implementation

---

## Phased Rollout

### Phase 1: MVP (Week 1)
- Connection Activity Cards (basic)
- Per-Connection Bar Chart
- Enhanced Health Table

### Phase 2: Charts (Week 2)
- Token Usage Chart
- Success Rate Chart
- Auto-refresh

### Phase 3: Real-time (Week 3)
- Live Request Stream
- SSE integration
- Real-time status updates

### Phase 4: Polish (Week 4)
- Responsive design
- Animations
- Performance optimization
- Accessibility audit
