# Code Standards & Structure

## Go Conventions

### Formatting & Linting
- **Strict Formatting**: Standard `gofmt` compliance is required for all Go files.
- **Naming Conventions**: 
  - File names should use `kebab-case.go` (e.g., `chat-handler.go`, `request-translator.go`).
  - Structs & Interfaces map to `PascalCase` when public, `camelCase` when private.

### Architecture Guidelines (Clean Architecture)
The application strictly segments responsibilities into 4 isolation layers inside the `internal/` directory:

1. **Domain (`domain/`)**: 
   - No external imports (except standard library).
   - Only data structures representing the pure business forms.
2. **Ports (`port/`)**: 
   - Go interfaces that define behaviors the `service` layer requires.
3. **Adapters (`adapter/`)**: 
   - Strict technical implementations hooking into libraries and network layers.
   - Example: Gin routes, HTTP clients, AWS payload mappers.
4. **Services (`service/`)**: 
   - Pure business logic functions orchestrating the adapters to solve a problem.

### Size Limitations
- Maintain smaller file constraints by keeping individual files conceptually targeted.
- **Component modules**: Target ≤200 lines per file.
- **Screen orchestrators**: ≤400 lines acceptable (coordinate multiple components).
- **Modularization strategy**: When files exceed limits, extract to subdirectories with focused responsibilities.
- Example: `Playground.tsx` (800+ lines) → `playground/` directory with `ModelSelector.tsx`, `ParameterControls.tsx`, `MessageList.tsx`, `InputArea.tsx`.

### Error Handling
- Wrap errors with additional context when passing up the call stack: `fmt.Errorf("do action: %w", err)`.
- Fallback strategies should leverage structured logging for visibility.

## Frontend UI Guidelines (React)
- **Framework**: Vite + React + TypeScript.
- **Formatting**: ESLint and Prettier used per standard `eslint.config.js`.
- **Component Design**: Organize cleanly into isolated component folders.
- **Dependencies**: Use `bun` to lock and install dependencies.
- **Component Library**: Use shadcn/ui components consistently (Button, Switch, RadioGroup, Dialog, Card, etc.).
- **Theming**: Use CSS variables for colors, avoid hardcoded theme values.
- **API Client**: Use unified `goApi` client for all backend requests.
- **File Organization**: 
  - Screen-level components in `src/screens/`
  - Shared components in `src/components/`
  - Screen-specific components in subdirectories (e.g., `src/screens/playground/`, `src/screens/logs/`)
- **Modularization**: Extract components when screen files exceed 400 lines.

## State Management and Persistence
- State configuration runs on a `db.json` approach leveraging file-locking via `gofrs/flock` to guarantee atomicity. No race conditions should occur during auth refreshes.
