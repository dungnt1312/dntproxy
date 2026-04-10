# Connections Page UX Improvements - Option A Implementation

## Summary
Implemented **Option A: Compact Card với Collapsible Sections** to reduce visual clutter and improve information hierarchy.

## Changes Made

### 1. **Compact Header Layout**
- Reduced toggle switch size: `w-9 h-5` → `w-8 h-4`
- Smaller status dot and badges
- Combined name + provider + auth method in single row
- Email moved to subtitle (11px, truncated)
- Reduced padding: `p-4` → `p-3`

### 2. **Collapsible Sections**
Added 3 new collapsible panels:

#### **Quota Panel** (collapsed by default)
- Click BarChart2 icon to toggle
- Shows quota bars only when expanded
- Inline refresh button
- Auto-loads on first expand

#### **Models Panel** (collapsed by default)
- Click Settings2 icon to toggle
- Edit models in textarea
- Fetch from API button
- Save/Cancel actions

#### **Details Panel** (NEW)
- Click Details icon to toggle
- Shows:
  - Token expiry bar (full width)
  - Model list with test buttons (first 10 + count)
  - Base URL (if available)
- Compact model chips (10px font)

### 3. **Compact Status Line**
Replaced verbose status components with single line:
```
Token: 2h 15m • 3 models • ⏱ Rate limit: 30s • Backoff: 2/7 • 🔒 1 locked • Reset
```

- All status info in one row (11px text)
- Inline reset button (no icon, just "Reset" link)
- Conditional rendering (only show if active)

### 4. **Action Buttons**
Reduced from 5-6 buttons to 5 compact icons:
- Test (TestTube)
- Quota (BarChart2) - toggles panel
- Models (Settings2) - toggles panel
- Details (Settings2) - toggles panel
- Delete (Trash2)

Removed:
- Fetch Models button (moved to model edit panel)
- Separate quota check (now toggle)

### 5. **State Management**
Added new state:
```tsx
const [expandedQuota, setExpandedQuota] = useState<Record<string, boolean>>({})
const [expandedDetails, setExpandedDetails] = useState<Record<string, boolean>>({})
```

New toggle functions:
```tsx
const toggleQuota = (id: string) => {
  setExpandedQuota(prev => ({ ...prev, [id]: !prev[id] }))
  if (!expandedQuota[id] && !quotaResult[id]) {
    handleCheckQuota(id) // Auto-load on first expand
  }
}

const toggleDetails = (id: string) => {
  setExpandedDetails(prev => ({ ...prev, [id]: !prev[id] }))
}
```

### 6. **Removed Components**
- `<TokenBar />` - moved to Details panel
- `<StatusRow />` - replaced with compact inline status
- Verbose model chips display - moved to Details panel

## Visual Comparison

### Before (Old Layout)
```
┌─────────────────────────────────────────────────────────────┐
│ [Toggle] ● Name • Provider • Auth • API Key • Expired      │
│           email@example.com                                  │
│                                                              │
│           Token: ████████░░ 2h 15m (45%)                    │
│           ✓ No rate limit • Backoff: 2/7 • 🔒 1 locked     │
│           [Reset cooldown]                                   │
│                                                              │
│           Models (12):                                       │
│           [model1] [model2] [model3] ... [model12]          │
│                                                              │
│           [Fetch] [Quota] [Models] [Test] [Delete]         │
├─────────────────────────────────────────────────────────────┤
│ Quota Panel (always visible if checked)                     │
│   Requests: 450/500 (90%) ████████░░                        │
│   Tokens: 2.5M/3M (83%) ███████░░░                          │
└─────────────────────────────────────────────────────────────┘
```

### After (New Compact Layout)
```
┌─────────────────────────────────────────────────────────────┐
│ [T] ● Name • Provider • Auth                                │
│       email@example.com                                      │
│       Token: 2h 15m • 3 models • ⏱ 30s • Backoff: 2/7 • Reset│
│       [Test] [Quota▼] [Models▼] [Details▼] [Delete]        │
├─────────────────────────────────────────────────────────────┤
│ ▼ Quota (click to expand/collapse)                          │
│   Requests: 450/500 (90%) ████████░░                        │
│   Tokens: 2.5M/3M (83%) ███████░░░                          │
├─────────────────────────────────────────────────────────────┤
│ ▼ Details (click to expand/collapse)                        │
│   Token: ████████░░ 2h 15m (45%)                            │
│   Models (12): ✓ model1 • ✗ model2 • model3 • +9 more      │
│   Base URL: https://api.example.com                         │
└─────────────────────────────────────────────────────────────┘
```

## Benefits

### Space Savings
- **~40% height reduction** per card (collapsed state)
- Can see 2-3x more connections on screen
- Less scrolling required

### Cognitive Load
- **Progressive disclosure**: show summary → expand details
- Clear visual hierarchy
- Important info (name, status, actions) always visible
- Details hidden until needed

### Interaction
- **Fewer clicks**: toggle panels vs separate pages
- **Faster scanning**: compact status line
- **Better mobile**: smaller touch targets, less wrapping

### Maintainability
- **Less code**: removed redundant components
- **Cleaner state**: centralized toggle logic
- **Easier to extend**: add new panels without cluttering

## Testing Checklist

- [ ] Toggle switches work (enable/disable)
- [ ] Quota panel expands/collapses
- [ ] Models panel expands/collapses
- [ ] Details panel expands/collapses
- [ ] Auto-load quota on first expand
- [ ] Test connection button works
- [ ] Delete connection works
- [ ] Inline rename works
- [ ] Reset cooldown works
- [ ] Model test (individual + batch) works
- [ ] Fetch models from API works
- [ ] Save models works
- [ ] Responsive layout (mobile/tablet)

## Future Enhancements (Phase 2)

1. **Dropdown Menu** for actions (replace 5 buttons with ⋮ menu)
2. **Keyboard shortcuts** (e/d to expand/collapse, t to test)
3. **Bulk actions** (select multiple → test all, delete all)
4. **Connection grouping** by provider
5. **Drag-drop reordering** for priority
6. **Search/filter** connections
7. **Export/import** connection configs

## Rollback Plan

If issues arise, revert these commits:
- Compact header layout
- Collapsible sections
- Compact status line
- Action button changes

Original components (`TokenBar`, `StatusRow`) are still in codebase, just not used.
