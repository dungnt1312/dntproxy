# Phase 4 — UI Toggle & Stats Card

**Status:** pending
**Priority:** P2 (admin UX; CLI access works without UI)
**Estimated:** 0.5 day

## Context Links

- Settings screen: `ui/src/components/screens/settings-screen.tsx`
- API client: `ui/src/lib/go-api.ts`
- Existing logs API: `GET /api/logs/summary` (used elsewhere — verify path)

## Overview

Add a "Token Compression" card to the React Settings screen that toggles `compression.enabled`, optionally surfaces `minContentLength` and `logSavings` switches, and shows aggregate savings (when stats are available from the logs endpoint).

## Key Insights

- `settings-screen.tsx` already follows a card-per-section pattern — match that style; no router/global state churn.
- `mapSettings` (phase 2) maps Go `compression.*` → camelCase fields. Just consume those fields here.
- For "savings stats" we read from the existing logs endpoint and aggregate client-side (via the `/api/logs/summary` style endpoint or by `metadata_json` filtering once phase 3 lands the metadata).

## Requirements

### Functional
- Toggle: `Compression Enabled` (Switch).
- Toggle: `Log Savings to Stats DB` (Switch, gated by Compression Enabled).
- (Optional) Number input: `Min Content Length` (default 500, min 100, max 100000).
- Read-only stats panel: total bytes saved (24h), estimated tokens saved (24h), top 3 detected types.
- "Save Settings" button reuses existing pattern (`hasChanges`, `Save`, `Reset`).

### Non-Functional
- No new dependencies — use existing `@/components/ui/*` Shadcn components, `lucide-react` icons (e.g., `Sparkles`, `Database`).
- Stats panel: skeleton loader on fetch, gracefully empty when no data.

## Architecture

```
SettingsScreen
  ├── Server Configuration card        (existing)
  ├── Security card                     (existing)
  ├── Token Compression card           ← NEW
  │     ├── Switch: Enabled
  │     ├── Switch: Log Savings
  │     ├── Input:  Min Content Length (advanced, collapsible)
  │     └── StatsPanel (24h aggregate, fetched lazily)
  └── Save / Reset action row           (existing)
```

The new card lives at the same nesting depth as the existing Security card, between Server Configuration and Security (or at the bottom — author's choice; follow visual flow of "Server → Security → Optimisation").

## Related Code Files

### Files to modify
- `ui/src/components/screens/settings-screen.tsx` — add card + state fields.
- `ui/src/lib/go-api.ts` — already extended in phase 2 for round-trip fields; this phase only consumes them.

### Files to create
- (Optional) `ui/src/components/screens/compression-stats-panel.tsx` — only if the card grows past ~80 lines, extract to keep the parent under 200 lines.

## Implementation Steps

1. **Extend `SettingsData` type** in `settings-screen.tsx`:
   ```ts
   interface SettingsData {
     id: string;
     serverPort: number;
     apiKeyAuthEnabled: boolean;
     defaultRoutingStrategy: string;
     compressionEnabled: boolean;
     compressionMinLength: number;
     compressionLogSavings: boolean;
   }
   const DEFAULT_SETTINGS: SettingsData = {
     // existing defaults …
     compressionEnabled: false,
     compressionMinLength: 500,
     compressionLogSavings: true,
   };
   ```
2. **Add card markup** under existing motion.div pattern. Example skeleton:
   ```tsx
   <motion.div variants={itemVariants}>
     <Card>
       <CardHeader>
         <div className="flex items-center gap-2">
           <Sparkles className="size-5 text-violet-600" />
           <CardTitle className="text-base">Token Compression</CardTitle>
         </div>
         <CardDescription>
           Detect verbose command output (git/test/ls/log/json) and compress before forwarding to provider. Reduces token cost on tool-heavy agent loops.
         </CardDescription>
       </CardHeader>
       <CardContent className="space-y-4 p-6 pt-0">
         <div className="flex items-center justify-between gap-4">
           <div className="space-y-0.5">
             <Label htmlFor="compEnabled">Enable compression</Label>
             <p className="text-xs text-muted-foreground">
               Rewrites tool result content using compact, semantically-equivalent forms.
             </p>
           </div>
           <Switch id="compEnabled"
             checked={settings.compressionEnabled}
             onCheckedChange={(v) => updateField('compressionEnabled', v)} />
         </div>

         <div className="flex items-center justify-between gap-4">
           <div className="space-y-0.5">
             <Label htmlFor="compLog">Log savings</Label>
             <p className="text-xs text-muted-foreground">
               Track per-request original/compressed bytes in the logs database.
             </p>
           </div>
           <Switch id="compLog" disabled={!settings.compressionEnabled}
             checked={settings.compressionLogSavings}
             onCheckedChange={(v) => updateField('compressionLogSavings', v)} />
         </div>

         {/* Stats panel */}
         <CompressionStatsPanel enabled={settings.compressionEnabled} />
       </CardContent>
     </Card>
   </motion.div>
   ```
3. **Wire `handleSave`** to include compression fields in the `goApi.updateSettings` payload (already accepted by phase-2 `updateSettings`):
   ```ts
   const json = await goApi.updateSettings({
     serverPort: settings.serverPort,
     apiKeyAuthEnabled: settings.apiKeyAuthEnabled,
     defaultRoutingStrategy: settings.defaultRoutingStrategy,
     compressionEnabled: settings.compressionEnabled,
     compressionMinLength: settings.compressionMinLength,
     compressionLogSavings: settings.compressionLogSavings,
   });
   ```
4. **Stats panel.** Two strategies — pick one:
   - **A. Defer to v2.** Render the panel as a placeholder with copy "Stats appear here once you enable Logging" — zero extra work this phase.
   - **B. Inline aggregation.** Call `goApi.getLogs?.({ search: 'compression' })` (if exists) or read `/api/logs?range=1d`, filter rows whose `metadataJson` contains a `compression` key, sum `orig - comp`. Cap to top 3 detection types by count.

   **Recommendation:** Ship A in this phase to keep scope tight; add B in a follow-up once metadata is reliably populated by phase 3.

## Todo

- [ ] Extend `SettingsData` type and `DEFAULT_SETTINGS`.
- [ ] Add the new card with two switches.
- [ ] (Optional) Add `Min Content Length` input under an "Advanced" `<details>`.
- [ ] Update `handleSave` payload.
- [ ] Add stats panel placeholder (or implement aggregation).
- [ ] `cd ui && npm run build` clean.
- [ ] Visual smoke test against running backend with `compression.enabled` toggle round-trip.

## Success Criteria

- Toggle persists across page reload.
- Toggling triggers `hasChanges = true` and Save button enables.
- API roundtrip: toggle on → save → reload → checkbox stays on.
- Save followed by a chat request shows compression-applied evidence in server logs (matches phase 3 success).

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Existing `mapSettings`/`updateSettings` payload merge silently drops new fields | Confirmed in phase 2 step 3 — fields explicitly listed in payload. |
| Stats panel queries before metadata exists | Render placeholder copy gracefully; do not block the card. |
| Visual regression on the settings page | Reuse existing `Card`/`Switch`/`Label` components — no new styles. |

## Security Considerations

None — settings page already protected by admin middleware.

## Next Steps

- Phase 5 covers automated tests for the backend; UI is exercised manually for v1.
- Future enhancement: full compression analytics dashboard on the Logs page.

## Unresolved Questions

1. Should we expose `Min Content Length` in v1 UI? **Recommendation:** Hide behind an "Advanced" disclosure; default 500 is fine for nearly all users.
2. Where does the savings number come from in v1? **Recommendation:** Defer (placeholder copy) until phase-3 metadata is stable.
