package views

import (
	"fmt"
	"strings"

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
	modeScan browserMode = iota
	modeQuery
	modeQueryInput
)

// TableBrowserModel displays items in a table
type TableBrowserModel struct {
	client    *dynamo.Client
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
	inputFocused int // 0 = pk, 1 = sk

	// Yanked key values from selected row
	yankedKeys map[string]string // e.g. "table_pk", "table_sk", "gsi_MyIndex_pk", "lsi_MyLSI_sk"

	loading components.Loading
	err     error

	width        int
	height       int
	columnOffset int // for horizontal scrolling
}

// NewTableBrowserModel creates a new table browser view
func NewTableBrowserModel(client *dynamo.Client, tableName string) TableBrowserModel {
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
		client:    client,
		tableName: tableName,
		loading:   components.NewLoading("Loading table schema..."),
		pkInput:   pkInput,
		skInput:   skInput,
		table:     t,
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
		m.loading.SetMessage("Scanning items...")
		return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{Limit: 50})

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

		switch msg.String() {
		case "enter":
			if len(m.items) > 0 {
				idx := m.table.Cursor()
				if idx < len(m.items) {
					item := m.items[idx]
					key := m.extractKey(item)
					return m, func() tea.Msg {
						return messages.NavigateToItemMsg{Item: item, Key: key}
					}
				}
			}
		case "ctrl+c":
			// Copy all key values based on schema
			if len(m.items) > 0 && m.schema != nil {
				idx := m.table.Cursor()
				if idx < len(m.items) {
					m.yankedKeys = m.extractAllKeys(m.items[idx])
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
			// Enter query/find mode
			m.mode = modeQueryInput
			m.pkInput.Focus()
			return m, textinput.Blink
		case "s":
			// Switch to scan mode (also works to escape failed query)
			if (m.mode == modeQuery || m.err != nil) && m.schema != nil {
				m.mode = modeScan
				m.err = nil
				m.pageHistory = nil
				m.columnOffset = 0
				m.loading.SetMessage("Scanning items...")
				m.items = nil
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{Limit: 50})
			}
		case "q", "esc":
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
	if m.items == nil && m.err == nil {
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m TableBrowserModel) handleQueryInput(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to previous mode (scan or query results) without clearing inputs
		if m.items != nil {
			m.mode = modeQuery // Go back to query results if we have them
		} else {
			m.mode = modeScan
		}
		return m, nil
	case "tab", "shift+tab":
		// Toggle between inputs
		if m.inputFocused == 0 {
			m.inputFocused = 1
			m.pkInput.Blur()
			m.skInput.Focus()
		} else {
			m.inputFocused = 0
			m.skInput.Blur()
			m.pkInput.Focus()
		}
		return m, textinput.Blink
	case "down":
		// Move to SK
		if m.inputFocused == 0 {
			m.inputFocused = 1
			m.pkInput.Blur()
			m.skInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "up":
		// Move to PK
		if m.inputFocused == 1 {
			m.inputFocused = 0
			m.skInput.Blur()
			m.pkInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "ctrl+v":
		// Paste yanked key value (for now: table keys only, later: index-aware)
		if m.yankedKeys != nil {
			if m.inputFocused == 0 {
				if v, ok := m.yankedKeys["table_pk"]; ok {
					m.pkInput.SetValue(v)
				}
			} else {
				if v, ok := m.yankedKeys["table_sk"]; ok {
					m.skInput.SetValue(v)
				}
			}
		}
		return m, nil
	case "enter":
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
			Limit: 50,
		}
		if m.skInput.Value() != "" {
			input.Key.SK = m.skInput.Value()
		}
		return m, m.client.QueryCmd(m.schema, input)
	}

	// Update the focused input
	var cmd tea.Cmd
	if m.inputFocused == 0 {
		m.pkInput, cmd = m.pkInput.Update(msg)
	} else {
		m.skInput, cmd = m.skInput.Update(msg)
	}
	return m, cmd
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
	modeStr := "SCAN"
	if m.mode == modeQuery || m.mode == modeQueryInput {
		modeStr = "QUERY"
	}
	title := fmt.Sprintf("%s [%s]", m.tableName, modeStr)
	s.WriteString(components.Title.Render(title) + "\n")

	// Query input mode
	if m.mode == modeQueryInput {
		s.WriteString("\n")
		s.WriteString("Partition Key: " + m.pkInput.View() + "\n")
		s.WriteString("Sort Key:      " + m.skInput.View() + "\n")
		s.WriteString("\n")
		s.WriteString(components.MutedStyle.Render("Tab to switch fields, Enter to query, Esc to cancel"))
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
	help := "↑/↓: rows | ←/→: columns | Enter: view | /: query | s: scan | r: refresh | Esc: back"
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
