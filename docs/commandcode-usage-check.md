# Command Code API Usage Check

How to check Command Code (commandcode.ai) credits and usage programmatically — reverse-engineered from the CLI bundle (`dist/cli.mjs`).

## Why

The Command Code CLI exposes usage only via:

- `/usage` — interactive overlay inside a chat session
- `https://commandcode.ai/usage` — web page (requires login)

There is no official public API or CLI flag for reading credits/usage headlessly. The endpoints below are what the CLI itself calls, discovered by reverse-engineering the bundle.

## Endpoints

Base URL: `https://api.commandcode.ai` (staging: `https://staging-api.commandcode.ai`)

| Endpoint | Purpose |
|----------|---------|
| `GET /alpha/whoami` | Current user + org (`org.id` is needed by the other calls) |
| `GET /alpha/billing/credits?orgId=<id>` | Credits + 5-hour/weekly window limits |
| `GET /alpha/billing/subscriptions?orgId=<id>` | Plan id, status, billing period |
| `GET /alpha/usage/summary?orgId=<id>&since=<periodStart>` | Token/cost/request totals since period start |

## Authentication

- API key comes from `~/.commandcode/auth.json` (field `apiKey`), or env var `COMMAND_CODE_API_KEY`.
- Sent as header: `Authorization: Bearer <apiKey>`.

On Windows the auth file is at `C:\Users\<user>\.commandcode\auth.json`.

## Example (Node.js)

```js
const fs = require('fs');
const os = require('os');
const path = require('path');

const apiKey = JSON.parse(fs.readFileSync(path.join(os.homedir(), '.commandcode', 'auth.json'), 'utf8')).apiKey;
const base = 'https://api.commandcode.ai';
const headers = { Authorization: 'Bearer ' + apiKey };

async function get(endpoint, params = {}) {
  const url = new URL(base + endpoint);
  for (const [k, v] of Object.entries(params)) if (v != null) url.searchParams.set(k, v);
  const res = await fetch(url, { headers });
  if (!res.ok) throw new Error(endpoint + ' -> ' + res.status + ' ' + await res.text());
  return res.json();
}

(async () => {
  const whoami = await get('/alpha/whoami');
  const orgId = whoami.org?.id ?? null;

  const [credits, subscription, summary] = await Promise.all([
    get('/alpha/billing/credits', { orgId }),
    get('/alpha/billing/subscriptions', { orgId }),
    get('/alpha/usage/summary', { orgId, since: null }),
  ]);

  console.log(JSON.stringify({ whoami, credits, subscription, summary }, null, 2));
})();
```

Note: `fetch` requires Node 18+. The `since` param for `/alpha/usage/summary` is the subscription's `currentPeriodStart` when known.

## Response Shapes (observed 2026-08)

### `/alpha/billing/credits`

```json
{
  "credits": {
    "belowThreshold": false,
    "creditThreshold": 0,
    "monthlyCredits": 3.59,
    "purchasedCredits": 0,
    "freeCredits": 0
  },
  "windowLimits": {
    "limited": true,
    "exceeded": null,
    "fiveHour": { "used": 0.22, "cap": 3, "exceeded": false, "resetAt": 1787313905620 },
    "weekly":   { "used": 0.77, "cap": 6, "exceeded": false, "resetAt": 1787892128076 }
  }
}
```

### `/alpha/billing/subscriptions`

```json
{
  "success": true,
  "data": {
    "status": "active",
    "planId": "individual-go",
    "currentPeriodStart": "2026-08-04T07:00:41.000Z",
    "currentPeriodEnd": "2026-09-04T07:00:41.000Z",
    "cancelAtPeriodEnd": false
  }
}
```

### `/alpha/usage/summary`

```json
{
  "totalCount": 2451,
  "totalCost": 6.33,
  "successRate": 99.96,
  "completedCount": 2450,
  "failedCount": 1,
  "totalTokensIn": 257714692,
  "totalTokensOut": 1412767,
  "totalTokens": 259127459,
  "totalCredits": 6.33,
  "periodBasis": "billing-period"
}
```

## Plan → Monthly Credits (from CLI constants)

| planId | Monthly credits |
|--------|-----------------|
| individual-go | 10 |
| individual-goat | 70 |
| individual-pro / pro-v1 | 30 / 80 |
| individual-provider | 15 |
| individual-max | 150 |
| individual-ultra | 300 |
| teams-pro | 40 |

## Headless Check One-Liner

```bash
node -e "const fs=require('fs'),os=require('os'),p=require('path');const k=JSON.parse(fs.readFileSync(p.join(os.homedir(),'.commandcode','auth.json'),'utf8')).apiKey;(async()=>{const g=async e=>{const r=await fetch('https://api.commandcode.ai'+e,{headers:{Authorization:'Bearer '+k}});return r.json()};const w=await g('/alpha/whoami');const c=await g('/alpha/billing/credits?orgId='+(w.org?.id||''));console.log('Plan credits left: $'+c.credits.monthlyCredits.toFixed(2));console.log('5h: '+c.windowLimits.fiveHour.used.toFixed(2)+'/'+c.windowLimits.fiveHour.cap+' | weekly: '+c.windowLimits.weekly.used.toFixed(2)+'/'+c.windowLimits.weekly.cap)})()"
```

## Caveats

- These are **undocumented internal endpoints** — they may change without notice.
- `planId` values and credit caps come from CLI constants and can drift from live billing.
- Use at your own risk; prefer the official UI when you need authoritative numbers.
