package views

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	dbtable "github.com/orhayat/dynamodb-go/table"
	"github.com/orhayat/dynamodb-go/tui/components"
	"github.com/orhayat/dynamodb-go/tui/dynamo"
	"github.com/orhayat/dynamodb-go/tui/messages"
)

type browserMode int

const (
	modeLoading browserMode = iota // Initial state, loading schema
	modeDescribe                   // Schema loaded, choose scan or query
	modeScan
	modeQuery
	modeQueryInput
)

// indexOption represents a queryable index (table or GSI/LSI)
type indexOption struct {
	name   string // empty for table, index name for GSI/LSI
	label  string // display label e.g. "Table", "GSI: byEmail"
	pkName string // partition key attribute name
	skName string // sort key attribute name (empty if none)
	isGSI  bool
	isLSI  bool
}

// TableBrowserModel displays items in a table
type TableBrowserModel struct {
	client    *dynamo.Client
	logger    *slog.Logger
	tableName string
	schema    *dbtable.TableDefinition

	items   []map[string]any
	columns []string
	table   table.Model

	mode            browserMode
	currentStartKey dbtable.PaginationKey
	nextPageKey     dbtable.PaginationKey
	hasNextPage     bool
	pageHistory     []dbtable.PaginationKey

	// Query input
	pkInput      textinput.Model
	skInput      textinput.Model
	inputFocused int // 0 = index, 1 = pk, 2 = sk

	// Index selection
	selectedIndex     string // empty = table, otherwise index name
	showIndexDropdown bool
	indexDropdownIdx  int
	indexOptions      []indexOption // populated from schema

	// Yanked key values from selected row
	yankedKeys map[string]string // e.g. "table_pk", "table_sk", "gsi_MyIndex_pk", "lsi_MyLSI_sk"

	loading      components.Loading
	err          error
	pendingMode  int         // mode to switch to after schema loads (0=scan, 1=query, 2=describe)
	previousMode browserMode // mode to return to when pressing esc in QUERY INPUT

	width int
	height       int
	columnOffset int // for horizontal scrolling
}

// NewTableBrowserModel creates a new table browser view
// initialMode: 0=scan, 1=query, 2=describe (matches messages.TableMode)
func NewTableBrowserModel(client *dynamo.Client, logger *slog.Logger, tableName string, initialMode int) TableBrowserModel {
	pkInput := textinput.New()
	pkInput.Placeholder = "Partition key value"
	pkInput.Focus()

	skInput := textinput.New()
	skInput.Placeholder = "Sort key value (optional)"

	// Initialize empty table
	t := table.New(
		table.WithColumns([]table.Column{}),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return TableBrowserModel{
		client:      client,
		logger:      logger,
		tableName:   tableName,
		loading:     components.NewLoading("Loading table schema..."),
		pkInput:     pkInput,
		skInput:     skInput,
		table:       t,
		pendingMode: initialMode,
	}
}

// SetSize sets the view dimensions
func (m *TableBrowserModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.pkInput.Width = width - 20
	m.skInput.Width = width - 20
	// Reserve space for header, info line, help text
	tableHeight := height - 12
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.table.SetHeight(tableHeight)
}

// Init starts loading table schema
func (m TableBrowserModel) Init() tea.Cmd {
	return tea.Batch(
		m.loading.Init(),
		m.client.DescribeTableCmd(m.tableName),
	)
}

// Update handles messages
func (m TableBrowserModel) Update(msg tea.Msg) (TableBrowserModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case dynamo.TableSchemaMsg:
		m.schema = &msg.Schema
		m.buildIndexOptions()
		// Apply pending mode
		switch m.pendingMode {
		case 1: // Query - came from tables list
			m.previousMode = modeLoading // sentinel: esc will go to tables list
			m.mode = modeQueryInput
			m.inputFocused = 0
			m.pkInput.Blur()
			m.skInput.Blur()
			if len(m.indexOptions) > 0 {
				opt := m.indexOptions[0]
				m.pkInput.Placeholder = opt.pkName + " value"
				if opt.skName != "" {
					m.skInput.Placeholder = opt.skName + " value (optional)"
				} else {
					m.skInput.Placeholder = "(no sort key)"
				}
			}
			return m, nil
		case 2: // Describe
			m.mode = modeDescribe
			return m, nil
		default: // Scan (0 or any other)
			m.mode = modeScan
			m.loading.SetMessage("Scanning items...")
			return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{Limit: 50})
		}

	case dynamo.ItemsLoadedMsg:
		m.items = msg.Items
		m.nextPageKey = msg.NextPage
		m.hasNextPage = msg.HasNextPage
		m.extractColumns()
		m.buildTable()
		m.err = nil

	case dynamo.ErrorMsg:
		m.err = msg.Err

	case tea.KeyMsg:
		// Handle query input mode
		if m.mode == modeQueryInput {
			return m.handleQueryInput(msg)
		}

		// Handle describe table menu
		if m.mode == modeDescribe {
			switch msg.String() {
			case "enter", "s":
				// Start scan
				m.mode = modeScan
				m.loading.SetMessage("Scanning items...")
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{Limit: 50})
			case "f":
				// Enter query mode from DESCRIBE
				m.previousMode = modeDescribe
				m.mode = modeQueryInput
				m.inputFocused = 0
				m.pkInput.Blur()
				m.skInput.Blur()
				if len(m.indexOptions) > 0 {
					opt := m.indexOptions[0]
					m.pkInput.Placeholder = opt.pkName + " value"
					if opt.skName != "" {
						m.skInput.Placeholder = opt.skName + " value (optional)"
					} else {
						m.skInput.Placeholder = "(no sort key)"
					}
				}
				return m, nil
			case "q", "esc":
				return m, func() tea.Msg { return messages.NavigateBackMsg{} }
			}
			return m, nil
		}

		switch msg.String() {
		case "enter":
			if len(m.items) > 0 && m.schema != nil {
				idx := m.table.Cursor()
				if idx < len(m.items) {
					item := m.items[idx]
					key := m.extractKey(item)
					pkName := m.schema.PrimaryKey.Name
					skName := m.schema.RangeKey.Name
					return m, func() tea.Msg {
						return messages.NavigateToItemMsg{Item: item, Key: key, PkName: pkName, SkName: skName}
					}
				}
			}
		case "ctrl+c":
			// Copy whole item as JSON to clipboard + store all keys for internal paste
			if len(m.items) > 0 && m.schema != nil {
				idx := m.table.Cursor()
				if idx < len(m.items) {
					item := m.items[idx]
					// Copy whole item as JSON to clipboard
					if jsonBytes, err := json.MarshalIndent(item, "", "  "); err == nil {
						clipboard.WriteAll(string(jsonBytes))
					}
					// Store all keys (table + GSI + LSI) for internal paste
					m.yankedKeys = m.extractAllKeys(item)
					// Debug log
					if m.logger != nil {
						m.logger.Debug("ctrl+c yankedKeys", "keys", m.yankedKeys)
					}
				}
			}
		case "n":
			// Next page
			if m.hasNextPage && m.schema != nil {
				m.pageHistory = append(m.pageHistory, m.currentStartKey)
				m.currentStartKey = m.nextPageKey
				m.loading.SetMessage("Loading next page...")
				m.items = nil
				if m.mode == modeQuery {
					return m, m.client.QueryCmd(m.schema, dbtable.QueryInput{
						Key:           dbtable.Key{PK: m.pkInput.Value(), SK: m.skInput.Value()},
						Index:         m.selectedIndex,
						PaginationKey: m.currentStartKey,
						Limit:         50,
					})
				}
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{
					PaginationKey: m.currentStartKey,
					Limit:         50,
				})
			}
		case "p":
			// Previous page
			if len(m.pageHistory) > 0 && m.schema != nil {
				m.currentStartKey = m.pageHistory[len(m.pageHistory)-1]
				m.pageHistory = m.pageHistory[:len(m.pageHistory)-1]
				m.loading.SetMessage("Loading previous page...")
				m.items = nil
				if m.mode == modeQuery {
					return m, m.client.QueryCmd(m.schema, dbtable.QueryInput{
						Key:           dbtable.Key{PK: m.pkInput.Value(), SK: m.skInput.Value()},
						Index:         m.selectedIndex,
						PaginationKey: m.currentStartKey,
						Limit:         50,
					})
				}
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{
					PaginationKey: m.currentStartKey,
					Limit:         50,
				})
			}
		case "f":
			// Enter query/find mode from SCAN or QUERY RESULTS
			m.previousMode = m.mode
			m.mode = modeQueryInput
			m.inputFocused = 0 // Start on index row
			m.pkInput.Blur()
			m.skInput.Blur()
			// Reset to table if no selection
			if m.selectedIndex == "" && len(m.indexOptions) > 0 {
				opt := m.indexOptions[0]
				m.pkInput.Placeholder = opt.pkName + " value"
				if opt.skName != "" {
					m.skInput.Placeholder = opt.skName + " value (optional)"
				} else {
					m.skInput.Placeholder = "(no sort key)"
				}
			}
			return m, nil
		case "s":
			// Switch to scan mode
			if m.schema != nil {
				m.mode = modeScan
				m.err = nil
				m.pageHistory = nil
				m.columnOffset = 0
				m.selectedIndex = "" // Reset to table
				m.loading.SetMessage("Scanning items...")
				m.items = nil
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{Limit: 50})
			}
		case "d":
			// Show describe/table info
			if m.schema != nil {
				m.mode = modeDescribe
				return m, nil
			}
		case "q", "esc":
			// QUERY RESULTS → go back to QUERY INPUT
			if m.mode == modeQuery {
				m.previousMode = modeQuery
				m.mode = modeQueryInput
				m.inputFocused = 0
				m.pkInput.Blur()
				m.skInput.Blur()
				return m, nil
			}
			// SCAN → go back to Tables List
			return m, func() tea.Msg { return messages.NavigateBackMsg{} }
		case "left", "h":
			// Scroll columns left
			if m.columnOffset > 0 {
				m.columnOffset--
				m.buildTable()
			}
			return m, nil
		case "right", "l":
			// Scroll columns right - but keep at least one column visible
			maxOffset := len(m.columns) - 1
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.columnOffset < maxOffset {
				m.columnOffset++
				m.buildTable()
			}
			return m, nil
		case "r":
			// Refresh/retry
			if m.schema != nil {
				m.err = nil
				m.loading.SetMessage("Refreshing...")
				m.items = nil
				m.pageHistory = nil
				m.columnOffset = 0
				m.currentStartKey = dbtable.PaginationKey{}
				if m.mode == modeQuery {
					return m, m.client.QueryCmd(m.schema, dbtable.QueryInput{
						Key:   dbtable.Key{PK: m.pkInput.Value(), SK: m.skInput.Value()},
						Index: m.selectedIndex,
						Limit: 50,
					})
				}
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{Limit: 50})
			}
		default:
			// Let table handle navigation (j/k/up/down/etc)
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update loading spinner
	if m.mode == modeLoading {
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m TableBrowserModel) handleQueryInput(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	key := msg.String()

	// Handle dropdown mode separately
	if m.showIndexDropdown {
		return m.handleIndexDropdown(msg)
	}

	// When typing in PK/SK fields, only esc works as special key
	if m.inputFocused == 1 || m.inputFocused == 2 {
		if key == "esc" {
			// Go back to index row
			m.pkInput.Blur()
			m.skInput.Blur()
			m.inputFocused = 0
			return m, nil
		}
		// All other keys go to text input (handled at end of function)
	}

	// Index row (inputFocused == 0) - navigation keys work
	if m.inputFocused == 0 {
		switch key {
		case "esc", "q":
			// Go back to tables list
			return m, func() tea.Msg { return messages.NavigateBackMsg{} }
		case "s":
			// Switch to scan mode
			if m.schema != nil {
				m.mode = modeScan
				m.err = nil
				m.pageHistory = nil
				m.columnOffset = 0
				m.selectedIndex = ""
				m.loading.SetMessage("Scanning items...")
				m.items = nil
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{Limit: 50})
			}
			return m, nil
		case "d":
			// Switch to describe mode
			m.mode = modeDescribe
			return m, nil
		case "f":
			// Focus PK input
			m.inputFocused = 1
			m.pkInput.Focus()
			return m, textinput.Blink
		}
	}

	switch key {
	case "tab":
		// Cycle forward: index -> pk -> sk -> index
		m.pkInput.Blur()
		m.skInput.Blur()
		m.inputFocused = (m.inputFocused + 1) % 3
		if m.inputFocused == 1 {
			m.pkInput.Focus()
		} else if m.inputFocused == 2 {
			m.skInput.Focus()
		}
		return m, textinput.Blink

	case "shift+tab":
		// Cycle backward: sk -> pk -> index -> sk
		m.pkInput.Blur()
		m.skInput.Blur()
		m.inputFocused = (m.inputFocused + 2) % 3 // +2 is same as -1 mod 3
		if m.inputFocused == 1 {
			m.pkInput.Focus()
		} else if m.inputFocused == 2 {
			m.skInput.Focus()
		}
		return m, textinput.Blink

	case "down":
		// Move down through rows
		if m.inputFocused < 2 {
			m.pkInput.Blur()
			m.skInput.Blur()
			m.inputFocused++
			if m.inputFocused == 1 {
				m.pkInput.Focus()
			} else if m.inputFocused == 2 {
				m.skInput.Focus()
			}
			return m, textinput.Blink
		}
		return m, nil

	case "up":
		// Move up through rows
		if m.inputFocused > 0 {
			m.pkInput.Blur()
			m.skInput.Blur()
			m.inputFocused--
			if m.inputFocused == 1 {
				m.pkInput.Focus()
			}
			// inputFocused == 0 means index row, no text input focused
			return m, textinput.Blink
		}
		return m, nil

	case "ctrl+d":
		// Delete/clear current input field
		if m.inputFocused == 1 {
			m.pkInput.SetValue("")
		} else if m.inputFocused == 2 {
			m.skInput.SetValue("")
		}
		return m, nil

	case "ctrl+v":
		// Paste yanked key value based on selected index
		if m.yankedKeys != nil {
			opt := m.getSelectedIndexOption()
			var pkKey, skKey string
			if opt.name == "" {
				pkKey = "table_pk"
				skKey = "table_sk"
			} else if opt.isGSI {
				pkKey = fmt.Sprintf("gsi_%s_pk", opt.name)
				skKey = fmt.Sprintf("gsi_%s_sk", opt.name)
			} else if opt.isLSI {
				pkKey = "table_pk" // LSI uses table's PK
				skKey = fmt.Sprintf("lsi_%s_sk", opt.name)
			}

			// Debug log
			if m.logger != nil {
				m.logger.Debug("ctrl+v paste",
					"index", opt.name,
					"isGSI", opt.isGSI,
					"isLSI", opt.isLSI,
					"focused", m.inputFocused,
					"pkKey", pkKey,
					"skKey", skKey,
					"yankedKeys", m.yankedKeys,
				)
			}

			if m.inputFocused == 1 {
				if v, ok := m.yankedKeys[pkKey]; ok {
					m.pkInput.SetValue(v)
				}
			} else if m.inputFocused == 2 {
				if v, ok := m.yankedKeys[skKey]; ok {
					m.skInput.SetValue(v)
				}
			}
		}
		return m, nil

	case "enter":
		// On index row, open dropdown
		if m.inputFocused == 0 {
			m.showIndexDropdown = true
			// Find current index in options
			for i, opt := range m.indexOptions {
				if opt.name == m.selectedIndex {
					m.indexDropdownIdx = i
					break
				}
			}
			return m, nil
		}

		// On PK/SK rows, execute query
		if m.pkInput.Value() == "" {
			return m, nil // PK is required
		}
		m.mode = modeQuery
		m.pageHistory = nil
		m.columnOffset = 0
		m.currentStartKey = dbtable.PaginationKey{}
		m.loading.SetMessage("Querying items...")
		m.items = nil
		input := dbtable.QueryInput{
			Key:   dbtable.Key{PK: m.pkInput.Value()},
			Index: m.selectedIndex,
			Limit: 50,
		}
		if m.skInput.Value() != "" {
			input.Key.SK = m.skInput.Value()
		}
		return m, m.client.QueryCmd(m.schema, input)
	}

	// Update the focused text input (only pk and sk rows)
	var cmd tea.Cmd
	if m.inputFocused == 1 {
		m.pkInput, cmd = m.pkInput.Update(msg)
	} else if m.inputFocused == 2 {
		m.skInput, cmd = m.skInput.Update(msg)
	}
	return m, cmd
}

func (m TableBrowserModel) renderIndexDropdown() string {
	var s strings.Builder

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(0, 1).
		MarginLeft(15)

	var rows []string
	for i, opt := range m.indexOptions {
		label := opt.label
		if opt.pkName != "" {
			keyInfo := opt.pkName
			if opt.skName != "" {
				keyInfo += ", " + opt.skName
			}
			label += " (" + keyInfo + ")"
		}

		if i == m.indexDropdownIdx {
			label = components.SelectedItem.Render("● " + label)
		} else {
			label = "  " + label
		}
		rows = append(rows, label)
	}

	s.WriteString(boxStyle.Render(strings.Join(rows, "\n")))
	s.WriteString("\n")
	return s.String()
}

func (m TableBrowserModel) handleIndexDropdown(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showIndexDropdown = false
		return m, nil

	case "up", "k":
		if m.indexDropdownIdx > 0 {
			m.indexDropdownIdx--
		}
		return m, nil

	case "down", "j":
		if m.indexDropdownIdx < len(m.indexOptions)-1 {
			m.indexDropdownIdx++
		}
		return m, nil

	case "enter":
		// Select the index
		if m.indexDropdownIdx < len(m.indexOptions) {
			m.selectedIndex = m.indexOptions[m.indexDropdownIdx].name
			// Update placeholders
			opt := m.indexOptions[m.indexDropdownIdx]
			m.pkInput.Placeholder = opt.pkName + " value"
			if opt.skName != "" {
				m.skInput.Placeholder = opt.skName + " value (optional)"
			} else {
				m.skInput.Placeholder = "(no sort key)"
			}
			// Clear values since keys changed
			m.pkInput.SetValue("")
			m.skInput.SetValue("")
		}
		m.showIndexDropdown = false
		return m, nil
	}

	return m, nil
}

func (m *TableBrowserModel) extractColumns() {
	seen := make(map[string]bool)
	var cols []string

	// Add key columns first
	if m.schema != nil {
		if m.schema.PrimaryKey.Name != "" {
			cols = append(cols, m.schema.PrimaryKey.Name)
			seen[m.schema.PrimaryKey.Name] = true
		}
		if m.schema.RangeKey.Name != "" {
			cols = append(cols, m.schema.RangeKey.Name)
			seen[m.schema.RangeKey.Name] = true
		}
	}

	// Add other columns from items
	for _, item := range m.items {
		for k := range item {
			if !seen[k] {
				cols = append(cols, k)
				seen[k] = true
			}
		}
	}
	m.columns = cols
}

func (m *TableBrowserModel) buildTable() {
	if len(m.columns) == 0 {
		return
	}

	// Ensure columnOffset is valid
	if m.columnOffset >= len(m.columns) {
		m.columnOffset = len(m.columns) - 1
	}
	if m.columnOffset < 0 {
		m.columnOffset = 0
	}

	// Calculate column widths based on content
	colWidths := make(map[string]int)
	for _, col := range m.columns {
		colWidths[col] = len(col) // Start with header width
	}

	// Check content widths
	for _, item := range m.items {
		for _, col := range m.columns {
			val := m.formatValue(item[col])
			if len(val) > colWidths[col] {
				colWidths[col] = len(val)
			}
		}
	}

	// Cap widths
	maxWidth := 30
	minWidth := 10
	totalWidth := m.width - 4 // padding
	if totalWidth < 40 {
		totalWidth = 80
	}

	// Determine which columns fit starting from columnOffset
	visibleCols := []string{}
	usedWidth := 0
	for i := m.columnOffset; i < len(m.columns); i++ {
		col := m.columns[i]
		width := colWidths[col]
		if width < minWidth {
			width = minWidth
		}
		if width > maxWidth {
			width = maxWidth
		}
		// Check if this column fits
		if usedWidth+width+3 > totalWidth && len(visibleCols) > 0 {
			break // No more room
		}
		visibleCols = append(visibleCols, col)
		usedWidth += width + 3 // +3 for separator
	}

	// Safety: ensure at least one column
	if len(visibleCols) == 0 && len(m.columns) > 0 {
		idx := m.columnOffset
		if idx >= len(m.columns) {
			idx = len(m.columns) - 1
		}
		visibleCols = []string{m.columns[idx]}
	}

	// Build columns for visible ones only
	columns := make([]table.Column, len(visibleCols))
	for i, col := range visibleCols {
		width := colWidths[col]
		if width < minWidth {
			width = minWidth
		}
		if width > maxWidth {
			width = maxWidth
		}
		columns[i] = table.Column{Title: col, Width: width}
	}

	// Build rows with only visible columns
	rows := make([]table.Row, len(m.items))
	for i, item := range m.items {
		row := make(table.Row, len(visibleCols))
		for j, col := range visibleCols {
			val := m.formatValue(item[col])
			// Truncate if needed
			maxLen := columns[j].Width
			if len(val) > maxLen {
				val = val[:maxLen-1] + "…"
			}
			row[j] = val
		}
		rows[i] = row
	}

	cursor := m.table.Cursor()
	// Clear rows first to avoid panic when column count changes
	m.table.SetRows([]table.Row{})
	m.table.SetColumns(columns)
	m.table.SetRows(rows)
	if cursor >= len(rows) {
		cursor = 0
	}
	m.table.SetCursor(cursor)
}

func (m TableBrowserModel) extractKey(item map[string]any) dbtable.Key {
	var key dbtable.Key
	if m.schema != nil {
		if pk, ok := item[m.schema.PrimaryKey.Name]; ok {
			key.PK = pk
		}
		if m.schema.RangeKey.Name != "" {
			if sk, ok := item[m.schema.RangeKey.Name]; ok {
				key.SK = sk
			}
		}
	}
	return key
}

// extractAllKeys extracts all key values from an item based on schema
// Keys are stored as: "table_pk", "table_sk", "gsi_IndexName_pk", "gsi_IndexName_sk", "lsi_IndexName_sk"
func (m TableBrowserModel) extractAllKeys(item map[string]any) map[string]string {
	keys := make(map[string]string)
	if m.schema == nil {
		return keys
	}

	// Table keys
	if v, ok := item[m.schema.PrimaryKey.Name]; ok {
		keys["table_pk"] = fmt.Sprintf("%v", v)
	}
	if m.schema.RangeKey.Name != "" {
		if v, ok := item[m.schema.RangeKey.Name]; ok {
			keys["table_sk"] = fmt.Sprintf("%v", v)
		}
	}

	// GSI keys
	for _, gsi := range m.schema.GSI {
		if v, ok := item[gsi.PrimaryKey.Name]; ok {
			keys[fmt.Sprintf("gsi_%s_pk", gsi.IndexName)] = fmt.Sprintf("%v", v)
		}
		if gsi.RangeKey.Name != "" {
			if v, ok := item[gsi.RangeKey.Name]; ok {
				keys[fmt.Sprintf("gsi_%s_sk", gsi.IndexName)] = fmt.Sprintf("%v", v)
			}
		}
	}

	// LSI keys (inherit table PK, have their own SK)
	for _, lsi := range m.schema.LSI {
		if v, ok := item[lsi.RangeKey.Name]; ok {
			keys[fmt.Sprintf("lsi_%s_sk", lsi.IndexName)] = fmt.Sprintf("%v", v)
		}
	}

	return keys
}

// View renders the table browser
func (m TableBrowserModel) View() string {
	var s strings.Builder

	// Header
	var modeStr string
	switch m.mode {
	case modeLoading:
		modeStr = "LOADING"
	case modeDescribe:
		modeStr = "DESCRIBE"
	case modeScan:
		modeStr = "SCAN"
	case modeQuery, modeQueryInput:
		if m.selectedIndex != "" {
			modeStr = "QUERY:" + m.selectedIndex
		} else {
			modeStr = "QUERY"
		}
	}
	title := fmt.Sprintf("%s [%s]", m.tableName, modeStr)
	s.WriteString(components.Title.Render(title) + "\n")

	// Loading state
	if m.mode == modeLoading {
		s.WriteString("\n" + m.loading.View() + "\n")
		return components.Container.Render(s.String())
	}

	// Describe table menu
	if m.mode == modeDescribe {
		s.WriteString("\n")
		if m.schema != nil {
			// Show table info
			s.WriteString(components.HelpKey.Render("Partition Key: ") + m.schema.PrimaryKey.Name + "\n")
			if m.schema.RangeKey.Name != "" {
				s.WriteString(components.HelpKey.Render("Sort Key:      ") + m.schema.RangeKey.Name + "\n")
			}
			if len(m.schema.GSI) > 0 {
				s.WriteString(components.HelpKey.Render("GSIs:          "))
				for i, gsi := range m.schema.GSI {
					if i > 0 {
						s.WriteString(", ")
					}
					s.WriteString(gsi.IndexName)
				}
				s.WriteString("\n")
			}
			if len(m.schema.LSI) > 0 {
				s.WriteString(components.HelpKey.Render("LSIs:          "))
				for i, lsi := range m.schema.LSI {
					if i > 0 {
						s.WriteString(", ")
					}
					s.WriteString(lsi.IndexName)
				}
				s.WriteString("\n")
			}
		}
		s.WriteString("\n")
		s.WriteString(components.MutedStyle.Render("enter/s: scan | f: query | esc: back"))
		return components.Container.Render(s.String())
	}

	// Query input mode
	if m.mode == modeQueryInput {
		s.WriteString("\n")

		// Get current index option for display
		opt := m.getSelectedIndexOption()

		// Index row
		indexLabel := opt.label
		if opt.pkName != "" {
			keyInfo := opt.pkName
			if opt.skName != "" {
				keyInfo += ", " + opt.skName
			}
			indexLabel += " (" + keyInfo + ")"
		}

		indexRowStyle := components.MutedStyle
		if m.inputFocused == 0 {
			indexRowStyle = lipgloss.NewStyle().Foreground(components.Primary).Bold(true)
		}
		s.WriteString(indexRowStyle.Render("Index:         "+indexLabel+" ▼") + "\n")

		// Show dropdown if open
		if m.showIndexDropdown {
			s.WriteString(m.renderIndexDropdown())
		}

		// PK row
		pkLabel := "Partition Key: "
		if m.inputFocused == 1 {
			pkLabel = components.HelpKey.Render("Partition Key: ")
		}
		s.WriteString(pkLabel + m.pkInput.View() + "\n")

		// SK row
		skLabel := "Sort Key:      "
		if m.inputFocused == 2 {
			skLabel = components.HelpKey.Render("Sort Key:      ")
		}
		s.WriteString(skLabel + m.skInput.View() + "\n")

		s.WriteString("\n")
		s.WriteString(components.MutedStyle.Render("↑/↓/tab: switch fields | enter: select index / query | esc: cancel"))
		return components.Container.Render(s.String())
	}

	if m.err != nil {
		s.WriteString("\n" + components.ErrorStyle.Render("Error: "+m.err.Error()) + "\n")
		s.WriteString(components.MutedStyle.Render("r: retry | s: scan | Esc: back") + "\n")
		return components.Container.Render(s.String())
	}

	if m.items == nil {
		s.WriteString("\n" + m.loading.View() + "\n")
		return components.Container.Render(s.String())
	}

	if len(m.items) == 0 {
		s.WriteString("\n" + components.MutedStyle.Render("No items found") + "\n")
		return components.Container.Render(s.String())
	}

	// Info line
	pageNum := len(m.pageHistory) + 1
	info := fmt.Sprintf("%d items | Page %d", len(m.items), pageNum)
	if m.hasNextPage {
		info += " | n: next"
	}
	if len(m.pageHistory) > 0 {
		info += " | p: prev"
	}
	// Column scroll indicator
	if len(m.columns) > 0 {
		info += fmt.Sprintf(" | Cols: %d-%d/%d", m.columnOffset+1, min(m.columnOffset+5, len(m.columns)), len(m.columns))
	}
	s.WriteString(components.MutedStyle.Render(info) + "\n\n")

	// Table
	s.WriteString(m.table.View() + "\n")

	// Help
	help := "↑/↓: rows | ←/→: columns | enter: view | d: describe | f: query | s: scan | r: refresh | esc: back"
	s.WriteString("\n" + components.MutedStyle.Render(help))

	return components.Container.Render(s.String())
}

func (m TableBrowserModel) formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		return fmt.Sprintf("[%d]", len(val))
	case map[string]any:
		return fmt.Sprintf("{%d}", len(val))
	default:
		return fmt.Sprintf("%v", val)
	}
}

// IsInQueryInput returns true if browser is in query input mode
func (m TableBrowserModel) IsInQueryInput() bool {
	return m.mode == modeQueryInput
}

// IsInDescribeMode returns true if browser is showing the describe table menu
func (m TableBrowserModel) IsInDescribeMode() bool {
	return m.mode == modeDescribe
}

// buildIndexOptions creates the list of queryable indexes from schema
func (m *TableBrowserModel) buildIndexOptions() {
	if m.schema == nil {
		return
	}

	m.indexOptions = []indexOption{
		{
			name:   "",
			label:  "Table",
			pkName: m.schema.PrimaryKey.Name,
			skName: m.schema.RangeKey.Name,
		},
	}

	for _, gsi := range m.schema.GSI {
		m.indexOptions = append(m.indexOptions, indexOption{
			name:   gsi.IndexName,
			label:  "GSI: " + gsi.IndexName,
			pkName: gsi.PrimaryKey.Name,
			skName: gsi.RangeKey.Name,
			isGSI:  true,
		})
	}

	for _, lsi := range m.schema.LSI {
		m.indexOptions = append(m.indexOptions, indexOption{
			name:   lsi.IndexName,
			label:  "LSI: " + lsi.IndexName,
			pkName: m.schema.PrimaryKey.Name, // LSI uses table's PK
			skName: lsi.RangeKey.Name,
			isLSI:  true,
		})
	}
}

// getSelectedIndexOption returns the currently selected index option
func (m TableBrowserModel) getSelectedIndexOption() indexOption {
	for _, opt := range m.indexOptions {
		if opt.name == m.selectedIndex {
			return opt
		}
	}
	// fallback to table
	if len(m.indexOptions) > 0 {
		return m.indexOptions[0]
	}
	return indexOption{label: "Table"}
}
