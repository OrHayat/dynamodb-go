# DynamoDB TUI - Implementation Plan

## Overview

Terminal UI for browsing and managing DynamoDB tables. Built with Bubble Tea framework, using the parent `dynamodb-go` package for DynamoDB operations.

## Architecture

```
tui/
├── main.go                 # Entry, AWS config, flags
├── go.work                 # Link to ../dynamodb-go
├── app/
│   └── app.go              # Root model, view routing
├── views/
│   ├── tables_list.go      # List DynamoDB tables
│   ├── table_browser.go    # Scan/Query items
│   └── item_detail.go      # View item as JSON
├── components/
│   ├── statusbar.go        # Current table/profile + keys
│   ├── loading.go          # Spinner for async ops
│   └── styles.go           # Shared lipgloss styles
└── dynamo/
    ├── client.go           # Async wrapper for parent pkg
    └── messages.go         # tea.Msg types for async results
```

## Navigation Flow

```
┌──────────────┐     Enter     ┌────────────────┐    Enter    ┌─────────────┐
│ Tables List  │ ──────────▶   │ Table Browser  │ ─────────▶  │ Item Detail │
│              │               │ (Scan/Query)   │             │ (JSON view) │
└──────────────┘    Esc        └────────────────┘    Esc      └─────────────┘
                 ◀──────────                      ◀─────────
```

## AWS Config

Uses AWS SDK default credential chain (same as AWS CLI):
- No flags: uses `[default]` profile
- `--profile <name>`: uses specified profile
- `--region <region>`: override region
- Respects `AWS_PROFILE`, `AWS_REGION` env vars

---

# Phase 1: Read-Only MVP

## 1. Project Setup

- [ ] 1.1. Delete existing tutorial/example files:
  - Delete `tutorial1.go`
  - Delete `components/` folder (contains broken `table/table.go`)
- [ ] 1.2. Create `go.work` to link parent `dynamodb-go` package
- [ ] 1.3. Update `go.mod` - add parent package as dependency
- [ ] 1.4. Create folder structure (`app/`, `views/`, `components/`, `dynamo/`)
- [ ] 1.5. Rewrite `main.go` as minimal entry point (placeholder until step 8)

## 2. DynamoDB Client Layer

- [ ] 2.1. Create `dynamo/client.go`
  - Wrap parent `table.Client`
  - Add `ListTables()` (not in parent pkg - call SDK directly)
  - Add async wrappers returning `tea.Cmd`:
    - `ListTablesCmd()`
    - `DescribeTableCmd(tableName)`
    - `ScanCmd(table, input)`
    - `QueryCmd(table, input)`
    - `GetItemCmd(table, key)`

- [ ] 2.2. Create `dynamo/messages.go`
  - Define message types for async results:
    - `TablesListMsg`
    - `TableSchemaMsg`
    - `ItemsLoadedMsg`
    - `ItemDetailMsg`
    - `ErrorMsg`

## 3. Shared Components

- [ ] 3.1. Create `components/styles.go`
  - Color palette (primary, secondary, error, muted)
  - Base styles (title, selected row, normal row)
  - Table styles (header, cell, border)

- [ ] 3.2. Create `components/statusbar.go`
  - Model with: current context (profile, region, table)
  - Render keybindings for current view
  - Accept `SetContext()` and `SetKeys()` methods

- [ ] 3.3. Create `components/loading.go`
  - Simple spinner component for async operations
  - Reuse `bubbles/spinner`

## 4. App Shell

- [ ] 4.1. Create `app/state.go`
  - Define view enum: `ViewTablesList`, `ViewTableBrowser`, `ViewItemDetail`
  - Define `AppState` struct holding current view + shared data

- [ ] 4.2. Create `app/app.go`
  - Root `Model` implementing `tea.Model`
  - Hold child view models
  - Route `Update()` to active view
  - Route `View()` to active view + statusbar
  - Handle view transitions (push/pop navigation stack)

## 5. Tables List View

- [ ] 5.1. Create `views/tables_list.go`
  - Model with: `[]string` tables, `cursor`, `loading`, `error`
  - `Init()`: trigger `ListTablesCmd`
  - `Update()`:
    - Handle `TablesListMsg` → populate list
    - Handle `ErrorMsg` → show error
    - Handle keys: `j/k` navigate, `Enter` select, `q` quit
  - `View()`: render table list with cursor highlight

- [ ] 5.2. Add `ListTables` pagination (DynamoDB returns max 100)

## 6. Table Browser View

- [ ] 6.1. Create `views/table_browser.go` - Model struct
  - `tableSchema` (from DescribeTable)
  - `items []map[string]any`
  - `columns []string` (attribute names)
  - `cursor` (selected row)
  - `paginationKey` (for next/prev)
  - `mode` (scan vs query)
  - `queryInput` (PK/SK values when in query mode)
  - `loading`, `error`

- [ ] 6.2. Implement `Init()`
  - Trigger `DescribeTableCmd`

- [ ] 6.3. Implement `Update()` - messages
  - `TableSchemaMsg` → store schema, trigger initial `ScanCmd`
  - `ItemsLoadedMsg` → populate items, extract column names
  - `ErrorMsg` → show error

- [ ] 6.4. Implement `Update()` - navigation keys
  - `j/k` navigate rows
  - `h/l` scroll columns horizontally (if many)
  - `Enter` select item → transition to detail view
  - `n` next page (if `paginationKey` exists)
  - `p` previous page (need to track page history)
  - `/` enter query mode
  - `Esc` back to tables list

- [ ] 6.5. Implement query mode
  - Show input for PK value
  - Optional SK value/condition
  - `Enter` execute query
  - `Esc` cancel, back to scan mode

- [ ] 6.6. Implement `View()`
  - Header: table name, mode (scan/query), item count
  - Data table with columns
  - Handle wide tables (truncate cells, horizontal scroll)
  - Pagination indicator

## 7. Item Detail View

- [ ] 7.1. Create `views/item_detail.go` - Model struct
  - `item map[string]any`
  - `viewport` (for scrolling)
  - `jsonString` (formatted)

- [ ] 7.2. Implement `Init()`
  - Format item as indented JSON
  - Initialize viewport

- [ ] 7.3. Implement `Update()`
  - `j/k` scroll viewport
  - `y` copy JSON to clipboard
  - `Esc` back to browser

- [ ] 7.4. Implement `View()`
  - Header: PK/SK values
  - JSON in viewport

## 8. Main Entry Point

- [ ] 8.1. Update `main.go`
  - Parse flags: `--profile`, `--region`
  - Load AWS config with profile/region
  - Create DynamoDB client
  - Create app model
  - Run `tea.NewProgram()`

- [ ] 8.2. Error handling
  - Invalid profile → show error, list available profiles
  - No credentials → helpful error message
  - Network error → show in UI, allow retry

## 9. Testing & Polish

- [ ] 9.1. Manual testing against real DynamoDB
- [ ] 9.2. Handle edge cases:
  - Empty tables
  - Tables with no sort key
  - Large items (truncate in browser, full in detail)
  - Binary attributes (show as base64)
- [ ] 9.3. Responsive layout (handle terminal resize)

---

## Keybindings (Phase 1)

| Key | Context | Action |
|-----|---------|--------|
| `j` / `↓` | All | Move down |
| `k` / `↑` | All | Move up |
| `Enter` | All | Select / Confirm |
| `Esc` | All | Back / Cancel |
| `q` | Tables List | Quit app |
| `h` / `←` | Browser | Scroll columns left |
| `l` / `→` | Browser | Scroll columns right |
| `n` | Browser | Next page |
| `p` | Browser | Previous page |
| `/` | Browser | Enter query mode |
| `y` | Item Detail | Copy JSON to clipboard |
| `?` | All | Show help |

---

# Phase 2: Enhanced Read (Future)

- Client-side filtering (`/` filter in browser)
- Column visibility toggle
- Column reordering
- Export page/all results to JSON file
- Refresh data (`r` key)
- Better JSON syntax highlighting

---

# Phase 3: Write Operations (Future)

- Create item (`c` key → JSON editor)
- Edit item (`e` key → JSON editor with current values)
- Delete item (`d` key → confirmation prompt)
- JSON editor component (textarea with validation)
- Optimistic UI updates
- Undo support

---

# Phase 4: Polish & Advanced (Future)

- In-app profile/region picker
- Index selection (GSI/LSI) for queries
- Saved/recent queries
- Table info view (schema, indexes, capacity)
- Batch operations UI
- Keyboard shortcuts help overlay
- Config file for preferences
- Themes (light/dark)
