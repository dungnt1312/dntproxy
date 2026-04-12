# Playground UX/UI Redesign

## Overview
Complete redesign of the Playground screen with improved model selection, parameter controls, and request/response logging.

## Key Improvements

### 1. Enhanced Model Selection ✅
**Before:** Simple flat dropdown with all models
**After:** Intelligent grouping and filtering with auto-update

- **Connection Filter Dropdown**: Filter models by specific connection or view all
  - Shows connection name
  - Only shows active connections
  - **Auto-selects first model when connection changes**
  - Models filtered by connection's supported models
  
- **Grouped Model Selector**: Models grouped by provider
  - Visual grouping with provider labels
  - Shows only models from selected connection (if filtered)
  - Better organization when multiple providers exist (Kiro, OpenAI, etc.)
  - Provider badge shown next to selected model

### 2. Parameter Controls - Dedicated Tab ✅
**Before:** No parameter controls
**After:** Full-featured parameters tab with clean layout

- **System Prompt**: Large textarea (120px min-height) for system instructions
  - Font-mono for better readability
  - Clear description and placeholder
- **Temperature**: Slider (0-2, step 0.1) with live value display
  - Individual reset button
  - Labels: "Deterministic" ↔ "Creative"
- **Top P**: Slider (0-1, step 0.01) with live value display
  - Individual reset button
  - Labels: "Focused" ↔ "Diverse"
- **Reset All** button at bottom
- **Tab-based layout** - no hidden collapsible panels

### 3. Request/Response Log Panel - Dedicated Tab ✅
**Before:** No request logging
**After:** Full log history in dedicated tab

**Features:**
- **Tab-based layout** - Chat, Parameters, Logs all accessible via tabs
- **Real-time Request Tracking**:
  - Timestamp, model, connection used
  - Status badges (pending/success/error)
  - Duration and token usage
  
- **Expandable Details**:
  - Full request body (model, messages, parameters)
  - Response content and usage stats
  - Error messages with styling
  
- **Log Management**:
  - Clear all logs button
  - Auto-scroll to latest log
  - Visual status indicators
  - Connection name display

### 4. Improved Visual Design ✅

**Layout Changes:**
- **Tab-based navigation** - Chat, Parameters, Logs
- Clean header with controls bar
- Full-screen content area for each tab
- Better spacing and typography

**Visual Enhancements:**
- Provider badges on selected model
- Better loading states
- Improved button styling
- Consistent color scheme
- Better dark mode support
- Log count badge on tab

**UX Improvements:**
- Auto-select model when connection changes
- Auto-scroll for messages and logs
- Better error feedback
- More informative empty states
- Clearer visual hierarchy
- Reset buttons for each parameter

## Files Modified

### Core Component
- `ui/src/components/screens/playground-screen.tsx` - Complete rewrite (280 → 700 lines)

### New UI Components
- `ui/src/components/ui/slider.tsx` - Slider component for parameters
- `ui/src/components/ui/collapsible.tsx` - Collapsible panel for parameters
- `ui/src/components/ui/separator.tsx` - Visual separator component

## Technical Details

### State Management
```typescript
- models: Model[]                    // Available models
- connections: Connection[]          // Active connections
- selectedModelId: string            // Current model
- filterConnection: string           // Connection filter ('all' or ID)
- messages: Message[]                // Chat messages
- chatParams: ChatParams             // System prompt, temperature, etc.
- requestLogs: RequestLog[]          // Request/response history
- expandedLog: string | null         // Currently expanded log
```

### Data Flow
1. Load models + connections on mount (parallel)
2. User selects connection filter → models filtered
3. User selects model from grouped dropdown
4. User sends message → request logged
5. Stream response → update message + log
6. Extract usage stats → update log with tokens
7. User expands log → see full request/response

### API Integration
- `goApi.getModels()` - Load available models
- `goApi.getConnections()` - Load active connections
- `fetch(/v1/chat/completions)` - Send chat requests
- Extract delta content from SSE stream
- Parse usage metadata from response

## Usage Examples

### Test Specific Connection
1. Select connection from "Filter by connection" dropdown
2. Models automatically filter to that connection
3. Select model from grouped dropdown
4. Send test message
5. View request/response in log panel

### Compare Models
1. Send request with Model A
2. Check log panel for response
3. Switch to Model B
4. Send same request
5. Compare responses in logs

### Debug Issues
1. Check error status in log panel
2. Expand failed request
3. View full request body and error message
4. Verify parameters and connection

## Benefits

### For Users
✅ Test models by provider/connection easily
✅ See all request/response details
✅ Debug issues without checking browser dev tools
✅ Fine-tune parameters before sending
✅ Track conversation history

### For Developers
✅ Clear request/response logging
✅ Visible parameter values
✅ Error details with context
✅ Connection-specific testing
✅ Better debugging workflow

## Future Enhancements

Potential improvements:
- [ ] Export logs as JSON
- [ ] Compare multiple responses side-by-side
- [ ] Save favorite configurations
- [ ] Token usage visualization
- [ ] Response time charts
- [ ] Multi-model parallel testing
- [ ] Conversation branching
- [ ] Prompt templates

## Testing Checklist

Before deployment:
- [ ] Load models and connections successfully
- [ ] Filter by connection works correctly
- [ ] Grouped model selector displays properly
- [ ] Parameters panel opens/closes
- [ ] Sliders update values correctly
- [ ] System prompt included in requests
- [ ] Request logs appear after sending
- [ ] Log expand/collapse works
- [ ] Error logging shows error details
- [ ] Token usage extracted and displayed
- [ ] Auto-scroll works for messages and logs
- [ ] Clear chat button works
- [ ] Clear logs button works
- [ ] Responsive layout on mobile
