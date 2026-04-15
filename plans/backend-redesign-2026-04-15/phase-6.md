---
status: pending
priority: high
effort: 1 week
phase: 6
dependencies: [phase-1, phase-2, phase-3, phase-4, phase-5]
---

# Phase 6: Dashboard UI & Deployment (Week 6)

## Goals

Refactor React dashboard UI, implement all management features, create deployment guides, and finalize documentation.

## Tasks

### 6.1 Dashboard Architecture

**Directory Structure:**
```
web/
├── src/
│   ├── components/
│   │   ├── layout/
│   │   │   ├── Header.tsx
│   │   │   ├── Sidebar.tsx
│   │   │   └── Layout.tsx
│   │   ├── apikeys/
│   │   │   ├── APIKeyList.tsx
│   │   │   ├── APIKeyForm.tsx
│   │   │   ├── APIKeyDetails.tsx
│   │   │   └── AddCreditsModal.tsx
│   │   ├── accounts/
│   │   │   ├── AccountList.tsx
│   │   │   ├── AccountForm.tsx
│   │   │   ├── AccountDetails.tsx
│   │   │   └── TestAccountModal.tsx
│   │   ├── combos/
│   │   │   ├── ComboList.tsx
│   │   │   ├── ComboForm.tsx
│   │   │   └── ComboDetails.tsx
│   │   ├── logs/
│   │   │   ├── LogList.tsx
│   │   │   ├── LogDetails.tsx
│   │   │   └── LogFilters.tsx
│   │   ├── analytics/
│   │   │   ├── UsageChart.tsx
│   │   │   ├── CostChart.tsx
│   │   │   ├── ProviderChart.tsx
│   │   │   └── Dashboard.tsx
│   │   └── settings/
│   │       ├── GeneralSettings.tsx
│   │       ├── TunnelSettings.tsx
│   │       └── SecuritySettings.tsx
│   ├── hooks/
│   │   ├── useAPIKeys.ts
│   │   ├── useAccounts.ts
│   │   ├── useCombos.ts
│   │   ├── useLogs.ts
│   │   └── useSettings.ts
│   ├── api/
│   │   └── client.ts
│   ├── types/
│   │   └── index.ts
│   └── App.tsx
├── package.json
└── vite.config.ts
```

**Tech Stack:**
- React 18
- TypeScript
- Vite
- TanStack Query (data fetching)
- TanStack Router (routing)
- Tailwind CSS
- shadcn/ui (components)
- Recharts (charts)

**Acceptance Criteria:**
- [ ] Clean component structure
- [ ] Type-safe API client
- [ ] Custom hooks for data fetching
- [ ] Responsive layout
- [ ] Dark mode support

---

### 6.2 API Client

**File:** `web/src/api/client.ts`

Implement type-safe API client:

```typescript
export class APIClient {
  private baseURL: string;
  
  constructor(baseURL: string) {
    this.baseURL = baseURL;
  }
  
  // API Keys
  async listAPIKeys(): Promise<APIKey[]>
  async createAPIKey(data: CreateAPIKeyRequest): Promise<CreateAPIKeyResponse>
  async getAPIKey(id: string): Promise<APIKey>
  async updateAPIKey(id: string, data: UpdateAPIKeyRequest): Promise<APIKey>
  async deleteAPIKey(id: string): Promise<void>
  async addCredits(id: string, amount: number): Promise<APIKey>
  
  // Accounts
  async listAccounts(): Promise<ProviderAccount[]>
  async createAccount(data: CreateAccountRequest): Promise<ProviderAccount>
  async getAccount(id: string): Promise<ProviderAccount>
  async updateAccount(id: string, data: UpdateAccountRequest): Promise<ProviderAccount>
  async deleteAccount(id: string): Promise<void>
  async testAccount(id: string): Promise<TestResult>
  async refreshToken(id: string): Promise<ProviderAccount>
  
  // Combos
  async listCombos(): Promise<Combo[]>
  async createCombo(data: CreateComboRequest): Promise<Combo>
  async getCombo(id: string): Promise<Combo>
  async updateCombo(id: string, data: UpdateComboRequest): Promise<Combo>
  async deleteCombo(id: string): Promise<void>
  
  // Logs
  async listLogs(filter: LogFilter): Promise<RequestLog[]>
  async streamLogs(): EventSource
  async clearLogs(retentionDays: number): Promise<void>
  
  // Settings
  async getSettings(): Promise<Settings>
  async updateSettings(data: UpdateSettingsRequest): Promise<Settings>
  
  // Analytics
  async getUsageStats(filter: UsageFilter): Promise<UsageStats>
  async getCostStats(filter: CostFilter): Promise<CostStats>
}
```

**Acceptance Criteria:**
- [ ] Type-safe methods
- [ ] Error handling
- [ ] Request/response types
- [ ] SSE support for log streaming
- [ ] Unit tests with mock fetch

---

### 6.3 API Key Management UI

**Files:**
- `web/src/components/apikeys/APIKeyList.tsx`
- `web/src/components/apikeys/APIKeyForm.tsx`
- `web/src/components/apikeys/APIKeyDetails.tsx`
- `web/src/components/apikeys/AddCreditsModal.tsx`

**Features:**
- List all API keys with status (active, expired, low balance)
- Create new key with form validation
- Show full key only on creation (copy to clipboard)
- Edit key (name, permissions, limits, expiry)
- Delete key (soft delete)
- Add credits modal
- View usage stats per key
- Filter/search keys

**Acceptance Criteria:**
- [ ] List with pagination
- [ ] Create form with validation
- [ ] Show full key on creation (copy button)
- [ ] Edit form (except key)
- [ ] Delete confirmation
- [ ] Add credits modal
- [ ] Usage stats chart
- [ ] Filter by status, tags
- [ ] Search by name
- [ ] Responsive design

---

### 6.4 Provider Account Management UI

**Files:**
- `web/src/components/accounts/AccountList.tsx`
- `web/src/components/accounts/AccountForm.tsx`
- `web/src/components/accounts/AccountDetails.tsx`
- `web/src/components/accounts/TestAccountModal.tsx`

**Features:**
- List all accounts with status (healthy, cooldown, unauthorized)
- Create new account (choose provider, auth method)
- OAuth flows (Builder ID, IDC, Social, Import)
- Edit account (name, priority, supported models)
- Delete account (soft delete)
- Test account (validate credentials)
- Refresh token
- View account health (cooldown, backoff level, last error)

**Acceptance Criteria:**
- [ ] List with provider filter
- [ ] Create form with provider selection
- [ ] OAuth flows (redirect to auth URL)
- [ ] Edit form (except credentials)
- [ ] Delete confirmation
- [ ] Test account button
- [ ] Refresh token button
- [ ] Health status indicators
- [ ] Cooldown countdown
- [ ] Responsive design

---

### 6.5 Combo Management UI

**Files:**
- `web/src/components/combos/ComboList.tsx`
- `web/src/components/combos/ComboForm.tsx`
- `web/src/components/combos/ComboDetails.tsx`

**Features:**
- List all combos with strategy
- Create new combo (select models, strategy)
- Edit combo (models, strategy)
- Delete combo
- Test combo (send test request)
- View combo usage stats

**Acceptance Criteria:**
- [ ] List with strategy badges
- [ ] Create form with model selection
- [ ] Drag-and-drop model ordering
- [ ] Strategy selection (fallback, round-robin)
- [ ] Edit form
- [ ] Delete confirmation
- [ ] Test combo button
- [ ] Usage stats chart
- [ ] Responsive design

---

### 6.6 Log Viewer UI

**Files:**
- `web/src/components/logs/LogList.tsx`
- `web/src/components/logs/LogDetails.tsx`
- `web/src/components/logs/LogFilters.tsx`

**Features:**
- List logs with filters (api_key_id, provider_id, date range)
- Real-time log streaming (SSE)
- Log details modal (full request/response)
- Export logs (CSV, JSON)
- Clear old logs

**Acceptance Criteria:**
- [ ] List with pagination
- [ ] Filters (api_key, provider, date range)
- [ ] Real-time streaming (SSE)
- [ ] Log details modal
- [ ] Export to CSV/JSON
- [ ] Clear logs button
- [ ] Auto-scroll on new logs
- [ ] Responsive design

---

### 6.7 Analytics Dashboard

**Files:**
- `web/src/components/analytics/Dashboard.tsx`
- `web/src/components/analytics/UsageChart.tsx`
- `web/src/components/analytics/CostChart.tsx`
- `web/src/components/analytics/ProviderChart.tsx`

**Features:**
- Overview cards (total requests, total cost, avg latency, error rate)
- Usage chart (requests over time)
- Cost chart (cost over time)
- Provider distribution chart (pie chart)
- Model distribution chart (bar chart)
- Top API keys by usage
- Top API keys by cost

**Acceptance Criteria:**
- [ ] Overview cards with stats
- [ ] Usage chart (line chart, 7/30/90 days)
- [ ] Cost chart (line chart, 7/30/90 days)
- [ ] Provider distribution (pie chart)
- [ ] Model distribution (bar chart)
- [ ] Top keys table (usage, cost)
- [ ] Date range selector
- [ ] Responsive design

---

### 6.8 Settings UI

**Files:**
- `web/src/components/settings/GeneralSettings.tsx`
- `web/src/components/settings/TunnelSettings.tsx`
- `web/src/components/settings/SecuritySettings.tsx`

**Features:**
- General settings (port, require_api_key)
- Tunnel settings (enable/disable, show URL)
- Security settings (API key requirements, rate limits)
- Backup/restore config

**Acceptance Criteria:**
- [ ] General settings form
- [ ] Tunnel enable/disable toggle
- [ ] Tunnel URL display (copy button)
- [ ] Security settings form
- [ ] Backup export button
- [ ] Restore import button
- [ ] Save confirmation
- [ ] Responsive design

---

### 6.9 Deployment: Docker

**File:** `Dockerfile`

Create production Dockerfile:

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o dntproxy ./cmd/dntproxy

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/dntproxy .
COPY --from=builder /app/web/dist ./web/dist

EXPOSE 20199
VOLUME ["/root/.dntproxy"]

CMD ["./dntproxy", "serve"]
```

**File:** `docker-compose.yml`

```yaml
version: '3.8'

services:
  dntproxy:
    build: .
    ports:
      - "20199:20199"
    volumes:
      - ./data:/root/.dntproxy
    environment:
      - DNTPROXY_PORT=20199
      - DNTPROXY_REQUIRE_API_KEY=true
    restart: unless-stopped
```

**Acceptance Criteria:**
- [ ] Multi-stage build (small image)
- [ ] Volume for data persistence
- [ ] Environment variables
- [ ] Health check
- [ ] Docker Compose support
- [ ] Documentation (README.md)

---

### 6.10 Deployment: Systemd Service

**File:** `install/dntproxy.service`

Create systemd service file:

```ini
[Unit]
Description=DNTProxy - Multi-Provider AI Gateway
After=network.target

[Service]
Type=simple
User=dntproxy
Group=dntproxy
WorkingDirectory=/opt/dntproxy
ExecStart=/opt/dntproxy/dntproxy serve
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

**File:** `install/install.sh`

Create install script:

```bash
#!/bin/bash
set -e

# Download binary
wget https://github.com/yourusername/dntproxy/releases/latest/download/dntproxy-linux-amd64 -O dntproxy
chmod +x dntproxy

# Create user
sudo useradd -r -s /bin/false dntproxy

# Install
sudo mkdir -p /opt/dntproxy
sudo mv dntproxy /opt/dntproxy/
sudo chown -R dntproxy:dntproxy /opt/dntproxy

# Install service
sudo cp dntproxy.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable dntproxy
sudo systemctl start dntproxy

echo "DNTProxy installed successfully!"
```

**Acceptance Criteria:**
- [ ] Systemd service file
- [ ] Install script (Linux)
- [ ] Install script (macOS)
- [ ] Install script (Windows)
- [ ] Uninstall script
- [ ] Documentation (INSTALL.md)

---

### 6.11 Documentation

**Files:**
- `README.md` — Overview, quick start, features
- `docs/INSTALL.md` — Installation guide (binary, Docker, systemd)
- `docs/API.md` — API documentation (OpenAPI spec)
- `docs/ARCHITECTURE.md` — Architecture overview
- `docs/DEVELOPMENT.md` — Development guide
- `docs/MIGRATION.md` — Migration guide from old version

**README.md Structure:**
```markdown
# DNTProxy

Multi-provider AI gateway with API key management, rate limiting, and credit tracking.

## Features
- 7 providers (Kiro, OpenAI, Anthropic, GLM, MiniMax, Qwen, OpenAI-Compatible)
- API key management (credits, rate limits, permissions)
- Multi-account fallback with cooldown
- Combo strategies (fallback, round-robin)
- OAuth flows (Builder ID, IDC, Social, Import)
- Cloudflare tunnel
- Web dashboard

## Quick Start
```bash
# Download binary
wget https://github.com/yourusername/dntproxy/releases/latest/download/dntproxy-linux-amd64
chmod +x dntproxy-linux-amd64
./dntproxy-linux-amd64 serve

# Or use Docker
docker run -p 20199:20199 -v ./data:/root/.dntproxy yourusername/dntproxy
```

## Usage
```bash
# Create API key
curl -X POST http://localhost:20199/admin/api-keys \
  -H "Content-Type: application/json" \
  -d '{"name":"My Key","credit_limit":100}'

# Use API
curl -X POST http://localhost:20199/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"kr/sonnet","messages":[{"role":"user","content":"Hello"}]}'
```

## Documentation
- [Installation Guide](docs/INSTALL.md)
- [API Documentation](docs/API.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Development Guide](docs/DEVELOPMENT.md)
```

**Acceptance Criteria:**
- [ ] README with quick start
- [ ] Installation guide (all platforms)
- [ ] API documentation (OpenAPI spec)
- [ ] Architecture overview
- [ ] Development guide
- [ ] Migration guide
- [ ] Screenshots of dashboard

---

### 6.12 Final Testing

**Test Checklist:**
- [ ] All providers work (Kiro, OpenAI, Anthropic, GLM, MiniMax, Qwen)
- [ ] Multi-account fallback works
- [ ] Combo strategies work (fallback, round-robin)
- [ ] API key authentication works
- [ ] Rate limiting works (RPM, TPM)
- [ ] Credit deduction works
- [ ] OAuth flows work (Builder ID, IDC, Social, Import)
- [ ] Token refresh works
- [ ] Cloudflare tunnel works
- [ ] File watcher works (auto-reload config)
- [ ] Dashboard UI works (all features)
- [ ] Docker deployment works
- [ ] Systemd service works
- [ ] Performance targets met (<50ms p99)
- [ ] No memory leaks
- [ ] No race conditions

---

## Dependencies

- Phase 1-5 (all backend components)
- React 18
- TypeScript
- Vite
- TanStack Query
- TanStack Router
- Tailwind CSS
- shadcn/ui
- Recharts

---

## Testing Strategy

### Unit Tests
- Component tests (React Testing Library)
- Hook tests
- API client tests
- 90%+ coverage

### Integration Tests
- E2E tests (Playwright)
- Test all user flows
- Test all API endpoints

### Manual Testing
- Test on all platforms (Windows, macOS, Linux)
- Test all browsers (Chrome, Firefox, Safari)
- Test mobile responsive design

---

## Deliverables

- [ ] React dashboard UI (all features)
- [ ] API client (type-safe)
- [ ] Custom hooks (data fetching)
- [ ] Analytics dashboard
- [ ] Docker deployment
- [ ] Systemd service
- [ ] Install scripts (all platforms)
- [ ] Documentation (README, INSTALL, API, ARCHITECTURE, DEVELOPMENT, MIGRATION)
- [ ] Screenshots
- [ ] Final testing checklist
- [ ] Release notes

---

## Estimated Effort

- Dashboard architecture: 4 hours
- API client: 4 hours
- API key management UI: 8 hours
- Account management UI: 8 hours
- Combo management UI: 6 hours
- Log viewer UI: 6 hours
- Analytics dashboard: 8 hours
- Settings UI: 4 hours
- Docker deployment: 4 hours
- Systemd service: 4 hours
- Install scripts: 6 hours
- Documentation: 8 hours
- Final testing: 8 hours
- Screenshots: 2 hours

**Total:** 80 hours (2 weeks)
