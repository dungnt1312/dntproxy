# Combo Pinned Accounts

## Overview

Combos now support **per-step account pinning**, allowing you to specify which connection should be used for each model in a combo chain.

## Format

### Model String Format

```
provider/model@connectionId
```

**Examples:**
- `kr/claude-opus-4.6@conn-123` - Pin to specific connection
- `kr/claude-opus-4.6@auto` - Explicit auto-select (same as no suffix)
- `kr/claude-opus-4.6` - Auto-select (default behavior)

### Combo Configuration

```json
{
  "name": "my-combo",
  "models": [
    "kr/claude-opus-4.6@conn-abc-123",
    "oai/gpt-4",
    "kr/claude-sonnet-4.5@conn-xyz-456"
  ]
}
```

**Behavior:**
1. **Step 1:** Use Kiro connection `conn-abc-123` for `claude-opus-4.6`
2. **Step 2:** Auto-select any available OpenAI connection for `gpt-4`
3. **Step 3:** Use Kiro connection `conn-xyz-456` for `claude-sonnet-4.5`

## Account Selection Logic

### Auto-Select (Default)

When no `@connectionId` is specified:
- ✅ Weighted random selection from available connections
- ✅ Respects rate limits and cooldowns
- ✅ Auto-retry with different connections on failure
- ✅ Filtered by `combo.connectionIds` if specified

### Pinned Connection

When `@connectionId` is specified:
- ✅ Uses only the specified connection
- ✅ Still respects rate limits and model locks
- ✅ Fails immediately if connection unavailable (no retry)
- ❌ Does NOT fall back to other connections
- ❌ Ignores `combo.connectionIds` global filter

## API Examples

### Create Combo with Pinned Accounts

```bash
POST /api/combos
{
  "name": "production-chain",
  "models": [
    "kr/claude-opus-4.6@conn-primary",
    "kr/claude-sonnet-4.5@conn-backup",
    "oai/gpt-4"
  ]
}
```

### Use Combo

```bash
POST /v1/chat/completions
{
  "model": "production-chain",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

**Flow:**
1. Try `claude-opus-4.6` on `conn-primary`
   - If success → return response
   - If fail → move to step 2 (no retry on other connections)
2. Try `claude-sonnet-4.5` on `conn-backup`
   - If success → return response
   - If fail → move to step 3
3. Try `gpt-4` on any available OpenAI connection (auto-select)
   - Weighted random selection
   - Auto-retry on failure

## Error Handling

### Pinned Connection Errors

| Error | Status | Behavior |
|-------|--------|----------|
| Connection not found | 404 | Move to next model |
| Connection rate limited | 429 | Move to next model |
| Model not supported | 400 | Move to next model |
| Connection inactive | 503 | Move to next model |

### Auto-Select Errors

| Error | Status | Behavior |
|-------|--------|----------|
| No active connections | 404 | Move to next model |
| All connections rate limited | 429 | Move to next model |
| Model not supported | 400 | Move to next model |
| All connections failed | 503 | Move to next model |

## Use Cases

### 1. Primary + Backup Strategy

```json
{
  "name": "primary-backup",
  "models": [
    "kr/opus@conn-primary",
    "kr/opus@conn-backup"
  ]
}
```

Always try primary first, fall back to backup only if primary fails.

### 2. Cost Optimization

```json
{
  "name": "cost-optimized",
  "models": [
    "kr/haiku@conn-cheap",
    "kr/sonnet@conn-standard",
    "kr/opus@conn-premium"
  ]
}
```

Try cheaper models first, escalate to more expensive ones only if needed.

### 3. Geographic Routing

```json
{
  "name": "geo-routing",
  "models": [
    "kr/opus@conn-us-east",
    "kr/opus@conn-eu-west",
    "kr/opus@conn-ap-south"
  ]
}
```

Try region-specific connections in order.

### 4. Mixed Strategy

```json
{
  "name": "mixed",
  "models": [
    "kr/opus@conn-vip",
    "kr/sonnet",
    "oai/gpt-4"
  ]
}
```

- Step 1: Pin to VIP connection
- Step 2: Auto-select from any Kiro connection
- Step 3: Auto-select from any OpenAI connection

## Backward Compatibility

✅ **Fully backward compatible**

Old combos without `@connectionId` continue to work exactly as before:

```json
{
  "name": "old-combo",
  "models": ["kr/opus", "oai/gpt-4"]
}
```

This still uses auto-select for all models.

## Implementation Details

### Parsing

```go
// ParseModelString parses "provider/model@connectionId"
parsed, err := ParseModelString("kr/opus@conn-123")
// → {Provider: "kiro", Model: "opus", ConnectionID: "conn-123"}
```

### Account Selection

```go
// SelectCredentialsForModel handles both pinned and auto-select
creds, err := accountSelector.SelectCredentialsForModel(
    "kr/opus@conn-123",  // model string with optional @connectionId
    excludeIDs,          // already-failed connections
    allowedConnectionIDs // global filter from combo.connectionIds
)
```

### Execution Flow

```
Request → ResolveRouting → HandleCombo → Loop models:
  ├─ ParseModelString("kr/opus@conn-123")
  ├─ SelectCredentialsForModel
  │   ├─ If @connectionId → selectPinnedConnection
  │   └─ Else → SelectCredentials (weighted random)
  ├─ Execute
  │   ├─ Success → return
  │   └─ Fail → mark unavailable, next model
  └─ Next model
```

## Testing

Run tests:

```bash
go test ./internal/service/ -run TestParseModelString -v
go test ./internal/service/ -run TestNormalizeModelStr -v
```

## UI Integration

Frontend should:

1. **Parse** combo models on load:
   ```javascript
   const parsed = parseModelString("kr/opus@conn-123");
   // → {provider: "kiro", model: "opus", connectionId: "conn-123"}
   ```

2. **Build** UI state:
   ```javascript
   const steps = combo.models.map(parseModelString);
   ```

3. **Serialize** on save:
   ```javascript
   const models = steps.map(step => {
     const base = `${step.provider}/${step.model}`;
     return step.connectionId ? `${base}@${step.connectionId}` : base;
   });
   ```

See UI design doc for detailed component specifications.
