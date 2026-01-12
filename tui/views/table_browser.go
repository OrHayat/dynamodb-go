package views

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	dbtable "github.com/orhayat/dynamodb-go/table"
	"github.com/orhayat/dynamodb-go/tui/components"
	"github.com/orhayat/dynamodb-go/tui/dynamo"
	"github.com/orhayat/dynamodb-go/tui/messages"
	"github.com/orhayat/dynamodb-go/tui/storage"
)

type browserMode int

const (
	modeLoading browserMode = iota // Initial state, loading schema
	modeDescribe                   // Schema loaded, choose scan or query
	modeScan
	modeQuery
	modeQueryInput
)

// Cache configuration - adjust these as needed
const (
	LogicalPageSize = 50 // items per logical page (display)
)

// cachedPage represents a fetched DynamoDB page
type cachedPage struct {
	items     []map[string]any
	fetchedAt time.Time
	nextKey   dbtable.PaginationKey
	hasMore   bool
}

// age returns how long ago this page was fetched
func (p cachedPage) age() time.Duration {
	return time.Since(p.fetchedAt)
}

// clearExportMsgMsg is sent to clear export status message
type clearExportMsgMsg struct{}

// indexOption represents a queryable index (table or GSI/LSI)
type indexOption struct {
	name   string // empty for table, index name for GSI/LSI
	label  string // display label e.g. "Table", "GSI: byEmail"
	pkName string // partition key attribute name
	skName string // sort key attribute name (empty if none)
	isGSI  bool
	isLSI  bool
}

// filterOperator represents a filter comparison operator
type filterOperator struct {
	symbol string // display symbol
	label  string // full name for help
}

var filterOperators = []filterOperator{
	{"=", "equals"},
	{"<>", "not equals"},
	{"<", "less than"},
	{">", "greater than"},
	{"<=", "less or equal"},
	{">=", "greater or equal"},
	{"begins_with", "begins with"},
	{"contains", "contains"},
}

// filterType represents a DynamoDB attribute type for filtering
var filterTypes = []string{"String", "Number", "Boolean"}

// validOperatorsForType returns indices of valid operators for a given type
func validOperatorsForType(typeIdx int) []int {
	switch filterTypes[typeIdx] {
	case "Boolean":
		return []int{0, 1} // = and <>
	case "Number":
		return []int{0, 1, 2, 3, 4, 5} // =, <>, <, >, <=, >= (no begins_with, contains)
	default: // String
		return []int{0, 1, 6, 7} // =, <>, begins_with, contains (no numeric comparisons)
	}
}

// TableBrowserModel displays items in a table
type TableBrowserModel struct {
	client        *dynamo.Client
	logger        *slog.Logger
	filterStorage storage.FilterStorage
	tableName     string
	schema        *dbtable.TableDefinition

	// Cached data
	cache          []cachedPage     // physical DynamoDB pages
	allItems       []map[string]any // flattened items from all cached pages
	logicalPageIdx int              // current logical page (0-indexed)
	fetchingMore   bool             // true when fetching next physical page

	columns []string
	table   table.Model

	mode browserMode

	// Export
	showExport      bool
	exportPath      textinput.Model
	exportMsg       string
	exportOverwrite bool

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

	// Filter input
	filterField        components.Autocomplete
	filterValue        textinput.Model
	filterTypeIdx      int  // index into filterTypes (0=String, 1=Number, 2=Boolean)
	filterOpIdx        int  // index into filterOperators
	showFilterTypeDrop bool // show type dropdown
	showFilterOpDrop   bool // show operator dropdown
	showFilter         bool // filter row visible in scan mode
	filterInputFocus   int  // 0=field, 1=type, 2=operator, 3=value

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
func NewTableBrowserModel(client *dynamo.Client, logger *slog.Logger, filterStorage storage.FilterStorage, tableName string, initialMode int) TableBrowserModel {
	pkInput := textinput.New()
	pkInput.Placeholder = "Partition key value"
	pkInput.Focus()

	skInput := textinput.New()
	skInput.Placeholder = "Sort key value (optional)"

	// Initialize filter components
	filterField := components.NewAutocomplete("field name")
	filterValue := textinput.New()
	filterValue.Placeholder = "value"

	// Initialize export input
	exportPath := textinput.New()
	exportPath.Placeholder = "filename.jsonl"
	exportPath.CharLimit = 256

	// Load saved filter state
	var filterTypeIdx, filterOpIdx int
	if savedFilter, ok := filterStorage.Get(tableName); ok {
		filterField.SetValue(savedFilter.Field)
		filterValue.SetValue(savedFilter.Value)
		filterTypeIdx = savedFilter.Type
		filterOpIdx = savedFilter.Operator
	}

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
		client:        client,
		logger:        logger,
		filterStorage: filterStorage,
		tableName:     tableName,
		loading:       components.NewLoading("Loading table schema..."),
		pkInput:       pkInput,
		skInput:       skInput,
		filterField:   filterField,
		filterValue:   filterValue,
		filterTypeIdx: filterTypeIdx,
		filterOpIdx:   filterOpIdx,
		exportPath:    exportPath,
		table:         t,
		pendingMode:   initialMode,
	}
}

// SetSize sets the view dimensions
func (m *TableBrowserModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.pkInput.Width = width - 20
	m.skInput.Width = width - 20
	m.filterField.SetWidth(20)
	m.filterValue.Width = width - 50 // leave room for field + operator
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
			return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{})
		}

	case dynamo.ItemsLoadedMsg:
		// Add to cache
		page := cachedPage{
			items:     msg.Items,
			fetchedAt: time.Now(),
			nextKey:   msg.NextPage,
			hasMore:   msg.HasNextPage,
		}
		m.cache = append(m.cache, page)
		m.fetchingMore = false // Allow next fetch
		m.rebuildAllItems()
		m.extractColumns()
		m.buildTableForLogicalPage()
		m.updateFilterSuggestions()
		m.err = nil
		// Log cache state
		m.logger.Info("Cache updated",
			"physicalPages", len(m.cache),
			"totalItems", len(m.allItems),
			"logicalPages", (len(m.allItems)+LogicalPageSize-1)/LogicalPageSize,
		)

	case clearExportMsgMsg:
		m.exportMsg = ""
		return m, nil

	case dynamo.ErrorMsg:
		m.err = msg.Err
		m.fetchingMore = false

	case tea.KeyMsg:
		// Handle export input mode
		if m.showExport {
			return m.handleExportInput(msg)
		}

		// Handle query input mode
		if m.mode == modeQueryInput {
			return m.handleQueryInput(msg)
		}

		// Handle scan filter input when filter is visible
		if m.mode == modeScan && m.showFilter {
			return m.handleScanFilterInput(msg)
		}

		// Handle describe table menu
		if m.mode == modeDescribe {
			switch msg.String() {
			case "enter", "s":
				// Start scan
				m.mode = modeScan
				m.loading.SetMessage("Scanning items...")
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{})
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
			visibleItems := m.getVisibleItems()
			if len(visibleItems) > 0 && m.schema != nil {
				idx := m.table.Cursor()
				if idx < len(visibleItems) {
					item := visibleItems[idx]
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
			visibleItems := m.getVisibleItems()
			if len(visibleItems) > 0 && m.schema != nil {
				idx := m.table.Cursor()
				if idx < len(visibleItems) {
					item := visibleItems[idx]
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
			// Next logical page
			if m.schema != nil && !m.fetchingMore {
				nextStart := (m.logicalPageIdx + 1) * LogicalPageSize
				if nextStart < len(m.allItems) {
					// Have cached data, just move to next logical page
					m.logicalPageIdx++
					m.buildTableForLogicalPage()
					return m, nil
				} else if m.hasMorePhysicalPages() {
					// Need to fetch next physical page
					m.fetchingMore = true
					m.loading.SetMessage("Loading more items...")
					return m, m.fetchNextPhysicalPage()
				}
			}
			return m, nil
		case "p":
			// Previous logical page (instant, from cache)
			if m.logicalPageIdx > 0 {
				m.logicalPageIdx--
				m.buildTableForLogicalPage()
			}
			return m, nil
		case "ctrl+e":
			// Export items
			if len(m.allItems) > 0 && m.schema != nil {
				m.showExport = true
				m.exportMsg = ""
				m.exportOverwrite = false
				timestamp := time.Now().Format("20060102_150405")
				defaultPath := fmt.Sprintf("%s_%s.jsonl", m.tableName, timestamp)
				m.exportPath.SetValue(defaultPath)
				m.exportPath.Focus()
				m.exportPath.CursorEnd()
				return m, textinput.Blink
			}
		case "/":
			// Toggle filter in scan mode
			if m.mode == modeScan {
				m.showFilter = true
				m.filterInputFocus = 0
				return m, m.filterField.Focus()
			}
			return m, nil
		case "f":
			// Enter query/find mode from SCAN or QUERY RESULTS
			m.previousMode = m.mode
			m.mode = modeQueryInput
			m.inputFocused = 0 // Start on index row
			m.showFilter = false
			m.blurAllInputs()
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
				m.clearCache()
				m.columnOffset = 0
				m.selectedIndex = "" // Reset to table
				// Keep filter visible if there's an active filter
				if !m.hasActiveFilter() {
					m.showFilter = false
				}
				m.blurAllInputs()
				m.loading.SetMessage("Scanning items...")
				input := dbtable.ScanInput{} // No limit - fetch full 1MB page
				if m.hasActiveFilter() {
					input.FilterExpression = m.buildFilterExpression()
				}
				return m, m.client.ScanCmd(m.schema, input)
			}
		case "d":
			// Show describe/table info
			if m.schema != nil {
				m.mode = modeDescribe
				m.showFilter = false
				m.blurAllInputs()
				return m, nil
			}
		case "q", "esc":
			// QUERY RESULTS → go back to QUERY INPUT
			if m.mode == modeQuery {
				m.previousMode = modeQuery
				m.mode = modeQueryInput
				m.inputFocused = 0
				m.blurAllInputs()
				return m, nil
			}
			// SCAN → go back to Tables List
			return m, func() tea.Msg { return messages.NavigateBackMsg{} }
		case "left", "h":
			// Scroll columns left
			if m.columnOffset > 0 {
				m.columnOffset--
				m.buildTableForLogicalPage()
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
				m.buildTableForLogicalPage()
			}
			return m, nil
		case "r":
			// Refresh/retry
			if m.schema != nil {
				m.err = nil
				m.loading.SetMessage("Refreshing...")
				m.clearCache()
				m.columnOffset = 0
				if m.mode == modeQuery {
					input := dbtable.QueryInput{
						Key:   dbtable.Key{PK: m.pkInput.Value(), SK: m.skInput.Value()},
						Index: m.selectedIndex,
					}
					if m.hasActiveFilter() {
						input.FilterExpression = m.buildFilterExpression()
					}
					return m, m.client.QueryCmd(m.schema, input)
				}
				input := dbtable.ScanInput{}
				if m.hasActiveFilter() {
					input.FilterExpression = m.buildFilterExpression()
				}
				return m, m.client.ScanCmd(m.schema, input)
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

	// Handle filter operator dropdown
	if m.showFilterOpDrop {
		return m.handleFilterOpDropdown(msg)
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

	// Handle filter row input
	if m.inputFocused == 3 {
		return m.handleFilterInput(msg)
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
				m.clearCache()
				m.columnOffset = 0
				m.selectedIndex = ""
				m.loading.SetMessage("Scanning items...")
				return m, m.client.ScanCmd(m.schema, dbtable.ScanInput{})
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
		// Cycle forward: index -> pk -> sk -> filter -> index
		m.blurAllInputs()
		m.inputFocused = (m.inputFocused + 1) % 4
		return m, m.focusCurrentInput()

	case "shift+tab":
		// Cycle backward: filter -> sk -> pk -> index -> filter
		m.blurAllInputs()
		m.inputFocused = (m.inputFocused + 3) % 4 // +3 is same as -1 mod 4
		return m, m.focusCurrentInput()

	case "down":
		// Move down through rows
		if m.inputFocused < 3 {
			m.blurAllInputs()
			m.inputFocused++
			return m, m.focusCurrentInput()
		}
		return m, nil

	case "up":
		// Move up through rows
		if m.inputFocused > 0 {
			m.blurAllInputs()
			m.inputFocused--
			return m, m.focusCurrentInput()
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
		m.clearCache()
		m.columnOffset = 0
		m.loading.SetMessage("Querying items...")
		input := dbtable.QueryInput{
			Key:   dbtable.Key{PK: m.pkInput.Value()},
			Index: m.selectedIndex,
		}
		if m.skInput.Value() != "" {
			input.Key.SK = m.skInput.Value()
		}
		// Add filter if set
		if m.hasActiveFilter() {
			input.FilterExpression = m.buildFilterExpression()
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

// renderFilterRow renders the filter input row
func (m TableBrowserModel) renderFilterRow(focused bool) string {
	var s strings.Builder

	// Label
	filterLabel := "Filter:        "
	if focused {
		filterLabel = components.HelpKey.Render("Filter:        ")
	}
	s.WriteString(filterLabel)

	// Field autocomplete
	fieldStyle := components.MutedStyle
	if focused && m.filterInputFocus == 0 {
		fieldStyle = lipgloss.NewStyle().Foreground(components.Primary)
	}
	fieldView := m.filterField.View()
	// Only show dropdown part if focused on field
	if focused && m.filterInputFocus == 0 && m.filterField.DropdownVisible() {
		s.WriteString(fieldView + "\n")
		return s.String() // dropdown takes over
	}
	// Just show the text input part (first line)
	fieldLines := strings.Split(fieldView, "\n")
	s.WriteString(fieldStyle.Render(fieldLines[0]))

	// Type dropdown
	s.WriteString(" ")
	typeStyle := components.MutedStyle
	if focused && m.filterInputFocus == 1 {
		typeStyle = lipgloss.NewStyle().Foreground(components.Primary).Bold(true)
	}
	typeLabel := "[" + filterTypes[m.filterTypeIdx] + " ▼]"
	s.WriteString(typeStyle.Render(typeLabel))

	// Show type dropdown if open
	if m.showFilterTypeDrop {
		s.WriteString("\n")
		s.WriteString(m.renderTypeDropdown())
		return s.String()
	}

	// Operator dropdown
	s.WriteString(" ")
	opStyle := components.MutedStyle
	if focused && m.filterInputFocus == 2 {
		opStyle = lipgloss.NewStyle().Foreground(components.Primary).Bold(true)
	}
	opLabel := "[" + filterOperators[m.filterOpIdx].symbol + " ▼]"
	s.WriteString(opStyle.Render(opLabel))

	// Show operator dropdown if open
	if m.showFilterOpDrop {
		s.WriteString("\n")
		s.WriteString(m.renderOperatorDropdown())
		return s.String()
	}

	// Value
	s.WriteString(" ")
	valueStyle := components.MutedStyle
	if focused && m.filterInputFocus == 3 {
		valueStyle = lipgloss.NewStyle().Foreground(components.Primary)
	}
	_ = valueStyle // value input has its own styling
	s.WriteString(m.filterValue.View())

	s.WriteString("\n")
	return s.String()
}

// renderOperatorDropdown renders the operator selection dropdown
func (m TableBrowserModel) renderOperatorDropdown() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(0, 1).
		MarginLeft(20)

	validOps := validOperatorsForType(m.filterTypeIdx)
	var rows []string
	for _, idx := range validOps {
		op := filterOperators[idx]
		label := op.symbol + " (" + op.label + ")"
		if idx == m.filterOpIdx {
			label = components.SelectedItem.Render("● " + label)
		} else {
			label = "  " + label
		}
		rows = append(rows, label)
	}

	return boxStyle.Render(strings.Join(rows, "\n")) + "\n"
}

// renderTypeDropdown renders the type selection dropdown
func (m TableBrowserModel) renderTypeDropdown() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(0, 1).
		MarginLeft(20)

	var rows []string
	for i, t := range filterTypes {
		label := t
		if i == m.filterTypeIdx {
			label = components.SelectedItem.Render("● " + label)
		} else {
			label = "  " + label
		}
		rows = append(rows, label)
	}

	return boxStyle.Render(strings.Join(rows, "\n")) + "\n"
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

// handleFilterOpDropdown handles keys when filter operator dropdown is open
func (m TableBrowserModel) handleFilterOpDropdown(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	validOps := validOperatorsForType(m.filterTypeIdx)

	// Find current position in valid ops list
	currentPos := 0
	for i, idx := range validOps {
		if idx == m.filterOpIdx {
			currentPos = i
			break
		}
	}

	switch msg.String() {
	case "esc":
		m.showFilterOpDrop = false
		return m, nil

	case "up", "k":
		if currentPos > 0 {
			m.filterOpIdx = validOps[currentPos-1]
		}
		return m, nil

	case "down", "j":
		if currentPos < len(validOps)-1 {
			m.filterOpIdx = validOps[currentPos+1]
		}
		return m, nil

	case "enter":
		m.showFilterOpDrop = false
		return m, nil
	}

	return m, nil
}

func (m TableBrowserModel) handleFilterTypeDrop(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showFilterTypeDrop = false
		return m, nil

	case "up", "k":
		if m.filterTypeIdx > 0 {
			m.filterTypeIdx--
			m.ensureValidOperator()
		}
		return m, nil

	case "down", "j":
		if m.filterTypeIdx < len(filterTypes)-1 {
			m.filterTypeIdx++
			m.ensureValidOperator()
		}
		return m, nil

	case "enter":
		m.showFilterTypeDrop = false
		return m, nil
	}

	return m, nil
}

// ensureValidOperator resets operator to first valid one if current is invalid for type
func (m *TableBrowserModel) ensureValidOperator() {
	validOps := validOperatorsForType(m.filterTypeIdx)
	for _, idx := range validOps {
		if idx == m.filterOpIdx {
			return // current operator is valid
		}
	}
	// Current operator not valid, reset to first valid
	m.filterOpIdx = validOps[0]
}

// handleFilterInput handles keys when filter row is focused
func (m TableBrowserModel) handleFilterInput(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	key := msg.String()

	// If autocomplete dropdown is visible, let it handle keys first
	if m.filterInputFocus == 0 && m.filterField.DropdownVisible() {
		switch key {
		case "enter", "tab":
			// Let autocomplete handle selection
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case "up", "down":
			// Let autocomplete handle navigation
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case "esc":
			// Close dropdown
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		}
	}

	// Handle type dropdown
	if m.showFilterTypeDrop {
		return m.handleFilterTypeDrop(msg)
	}

	switch key {
	case "esc":
		// Go back to index row
		m.blurAllInputs()
		m.inputFocused = 0
		return m, nil

	case "tab":
		// Cycle within filter: field -> type -> op -> value -> next row
		m.filterField.Blur()
		m.filterValue.Blur()
		m.filterInputFocus = (m.filterInputFocus + 1) % 4
		if m.filterInputFocus == 0 {
			// Moved past value, go to next main row
			m.inputFocused = 0
			return m, nil
		}
		return m, m.focusFilterInput()

	case "shift+tab":
		// Cycle backward within filter
		if m.filterInputFocus == 0 {
			// At field, go to previous main row
			m.blurAllInputs()
			m.inputFocused = 2 // SK row
			return m, m.focusCurrentInput()
		}
		m.filterField.Blur()
		m.filterValue.Blur()
		m.filterInputFocus--
		return m, m.focusFilterInput()

	case "left":
		// Smart left: on text input at cursor=0 → prev field, otherwise move cursor
		switch m.filterInputFocus {
		case 0: // field - let text input handle cursor
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case 1: // type dropdown - go to field
			m.filterInputFocus = 0
			return m, m.focusFilterInput()
		case 2: // operator dropdown - go to type
			m.filterInputFocus = 1
			return m, nil
		case 3: // value - at cursor=0 go to operator, else move cursor
			if m.filterValue.Position() == 0 {
				m.filterValue.Blur()
				m.filterInputFocus = 2
				return m, nil
			}
			var cmd tea.Cmd
			m.filterValue, cmd = m.filterValue.Update(msg)
			return m, cmd
		}

	case "right":
		// Smart right: on text input at end → next field, otherwise move cursor
		switch m.filterInputFocus {
		case 0: // field - at end go to type, else move cursor
			if m.filterField.Position() >= len(m.filterField.Value()) {
				m.filterField.Blur()
				m.filterInputFocus = 1
				return m, nil
			}
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case 1: // type dropdown - go to operator
			m.filterInputFocus = 2
			return m, nil
		case 2: // operator dropdown - go to value
			m.filterInputFocus = 3
			return m, m.focusFilterInput()
		case 3: // value - let text input handle cursor
			var cmd tea.Cmd
			m.filterValue, cmd = m.filterValue.Update(msg)
			return m, cmd
		}

	case "up":
		// Go to SK row
		m.blurAllInputs()
		m.inputFocused = 2
		return m, m.focusCurrentInput()

	case "down":
		// If on field, open dropdown
		if m.filterInputFocus == 0 {
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		}
		return m, nil

	case "enter":
		// On type, open type dropdown
		if m.filterInputFocus == 1 {
			m.showFilterTypeDrop = true
			return m, nil
		}
		// On operator, open operator dropdown
		if m.filterInputFocus == 2 {
			m.showFilterOpDrop = true
			return m, nil
		}
		// On field or value, execute query (same as PK/SK enter)
		if m.pkInput.Value() == "" {
			return m, nil // PK is required
		}
		m.mode = modeQuery
		m.clearCache()
		m.columnOffset = 0
		m.loading.SetMessage("Querying items...")
		input := dbtable.QueryInput{
			Key:   dbtable.Key{PK: m.pkInput.Value()},
			Index: m.selectedIndex,
		}
		if m.skInput.Value() != "" {
			input.Key.SK = m.skInput.Value()
		}
		// Add filter if set
		if m.hasActiveFilter() {
			input.FilterExpression = m.buildFilterExpression()
		}
		return m, m.client.QueryCmd(m.schema, input)

	case "ctrl+d":
		// Clear filter
		m.filterField.SetValue("")
		m.filterValue.SetValue("")
		m.filterTypeIdx = 0
		m.filterOpIdx = 0
		return m, nil
	}

	// Forward to focused filter component
	var cmd tea.Cmd
	if m.filterInputFocus == 0 {
		m.filterField, cmd = m.filterField.Update(msg)
	} else if m.filterInputFocus == 3 {
		m.filterValue, cmd = m.filterValue.Update(msg)
	}
	return m, cmd
}

// blurAllInputs removes focus from all text inputs
func (m *TableBrowserModel) blurAllInputs() {
	m.pkInput.Blur()
	m.skInput.Blur()
	m.filterField.Blur()
	m.filterValue.Blur()
}

// focusCurrentInput focuses the appropriate input for inputFocused state
func (m *TableBrowserModel) focusCurrentInput() tea.Cmd {
	switch m.inputFocused {
	case 1:
		return m.pkInput.Focus()
	case 2:
		return m.skInput.Focus()
	case 3:
		m.filterInputFocus = 0 // Start at field
		return m.filterField.Focus()
	}
	return nil
}

// focusFilterInput focuses the appropriate filter sub-input
func (m *TableBrowserModel) focusFilterInput() tea.Cmd {
	switch m.filterInputFocus {
	case 0:
		return m.filterField.Focus()
	case 3:
		return m.filterValue.Focus()
	}
	return nil // type and operator dropdowns don't need focus
}

// handleScanFilterInput handles keys when filter is visible in scan mode
func (m TableBrowserModel) handleScanFilterInput(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	key := msg.String()

	// Handle dropdowns first
	if m.showFilterTypeDrop {
		return m.handleFilterTypeDrop(msg)
	}
	if m.showFilterOpDrop {
		return m.handleFilterOpDropdown(msg)
	}

	// If autocomplete dropdown is visible, let it handle keys first
	if m.filterInputFocus == 0 && m.filterField.DropdownVisible() {
		switch key {
		case "enter", "tab":
			// Let autocomplete handle selection
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case "up", "down":
			// Let autocomplete handle navigation
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case "esc":
			// Close dropdown
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		}
	}

	switch key {
	case "esc":
		// Close filter and return to table navigation
		m.filterField.Blur()
		m.filterValue.Blur()
		m.showFilter = false
		return m, nil

	case "tab":
		// Cycle within filter: field -> type -> op -> value
		m.filterField.Blur()
		m.filterValue.Blur()
		m.filterInputFocus = (m.filterInputFocus + 1) % 4
		return m, m.focusFilterInput()

	case "shift+tab":
		// Cycle backward within filter
		m.filterField.Blur()
		m.filterValue.Blur()
		m.filterInputFocus = (m.filterInputFocus + 3) % 4 // +3 is same as -1 mod 4
		return m, m.focusFilterInput()

	case "left":
		// Smart left: on text input at cursor=0 → prev field, otherwise move cursor
		switch m.filterInputFocus {
		case 0: // field - let text input handle cursor
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case 1: // type dropdown - go to field
			m.filterInputFocus = 0
			return m, m.focusFilterInput()
		case 2: // operator dropdown - go to type
			m.filterInputFocus = 1
			return m, nil
		case 3: // value - at cursor=0 go to operator, else move cursor
			if m.filterValue.Position() == 0 {
				m.filterValue.Blur()
				m.filterInputFocus = 2
				return m, nil
			}
			var cmd tea.Cmd
			m.filterValue, cmd = m.filterValue.Update(msg)
			return m, cmd
		}

	case "right":
		// Smart right: on text input at end → next field, otherwise move cursor
		switch m.filterInputFocus {
		case 0: // field - at end go to type, else move cursor
			if m.filterField.Position() >= len(m.filterField.Value()) {
				m.filterField.Blur()
				m.filterInputFocus = 1
				return m, nil
			}
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		case 1: // type dropdown - go to operator
			m.filterInputFocus = 2
			return m, nil
		case 2: // operator dropdown - go to value
			m.filterInputFocus = 3
			return m, m.focusFilterInput()
		case 3: // value - let text input handle cursor
			var cmd tea.Cmd
			m.filterValue, cmd = m.filterValue.Update(msg)
			return m, cmd
		}

	case "down":
		// If on field, open dropdown
		if m.filterInputFocus == 0 {
			m.filterField, _ = m.filterField.Update(msg)
			return m, nil
		}
		return m, nil

	case "enter":
		// On type, open type dropdown
		if m.filterInputFocus == 1 {
			m.showFilterTypeDrop = true
			return m, nil
		}
		// On operator, open operator dropdown
		if m.filterInputFocus == 2 {
			m.showFilterOpDrop = true
			return m, nil
		}
		// Execute scan with filter
		if m.schema == nil {
			return m, nil
		}
		m.filterField.Blur()
		m.filterValue.Blur()
		m.showFilter = false
		m.clearCache()
		m.columnOffset = 0
		m.loading.SetMessage("Scanning with filter...")
		input := dbtable.ScanInput{}
		if m.hasActiveFilter() {
			input.FilterExpression = m.buildFilterExpression()
		}
		return m, m.client.ScanCmd(m.schema, input)

	case "ctrl+d":
		// Clear filter
		m.filterField.SetValue("")
		m.filterValue.SetValue("")
		m.filterTypeIdx = 0
		m.filterOpIdx = 0
		return m, nil
	}

	// Forward to focused filter component
	var cmd tea.Cmd
	if m.filterInputFocus == 0 {
		m.filterField, cmd = m.filterField.Update(msg)
	} else if m.filterInputFocus == 3 {
		m.filterValue, cmd = m.filterValue.Update(msg)
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
	for _, item := range m.allItems {
		for k := range item {
			if !seen[k] {
				cols = append(cols, k)
				seen[k] = true
			}
		}
	}
	m.columns = cols
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
		if m.hasActiveFilter() {
			modeStr += " | " + m.filterField.Value() + " " + filterOperators[m.filterOpIdx].symbol + " " + m.filterValue.Value()
		}
	case modeQuery, modeQueryInput:
		if m.selectedIndex != "" {
			modeStr = "QUERY:" + m.selectedIndex
		} else {
			modeStr = "QUERY"
		}
		if m.hasActiveFilter() {
			modeStr += " | " + m.filterField.Value() + " " + filterOperators[m.filterOpIdx].symbol + " " + m.filterValue.Value()
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

		// Filter row
		s.WriteString(m.renderFilterRow(m.inputFocused == 3))

		s.WriteString("\n")
		s.WriteString(components.MutedStyle.Render("↑/↓/tab: switch fields | enter: select index / query | esc: cancel"))
		return components.Container.Render(s.String())
	}

	if m.err != nil {
		s.WriteString("\n" + components.ErrorStyle.Render("Error: "+m.err.Error()) + "\n")
		s.WriteString(components.MutedStyle.Render("r: retry | s: scan | Esc: back") + "\n")
		return components.Container.Render(s.String())
	}

	if len(m.cache) == 0 {
		s.WriteString("\n" + m.loading.View() + "\n")
		return components.Container.Render(s.String())
	}

	if len(m.allItems) == 0 {
		s.WriteString("\n" + components.MutedStyle.Render("No items found") + "\n")
		return components.Container.Render(s.String())
	}

	// Info line
	visibleItems := m.getVisibleItems()
	totalLogicalPages := (len(m.allItems) + LogicalPageSize - 1) / LogicalPageSize
	info := fmt.Sprintf("%d items | Page %d/%d", len(visibleItems), m.logicalPageIdx+1, totalLogicalPages)
	// Show cache age (oldest page)
	if len(m.cache) > 0 {
		oldestAge := m.cache[0].age()
		for _, p := range m.cache[1:] {
			if p.age() > oldestAge {
				oldestAge = p.age()
			}
		}
		info += fmt.Sprintf(" | Cache: %s", formatDuration(oldestAge))
	}
	// Can go to next logical page if there are more items or more physical pages
	hasNextLogical := (m.logicalPageIdx+1)*LogicalPageSize < len(m.allItems) || m.hasMorePhysicalPages()
	if hasNextLogical {
		info += " | n: next"
	}
	if m.logicalPageIdx > 0 {
		info += " | p: prev"
	}
	// Column scroll indicator
	if len(m.columns) > 0 {
		info += fmt.Sprintf(" | Cols: %d-%d/%d", m.columnOffset+1, min(m.columnOffset+5, len(m.columns)), len(m.columns))
	}
	s.WriteString(components.MutedStyle.Render(info) + "\n\n")

	// Show filter row if active in scan mode
	if m.mode == modeScan && m.showFilter {
		s.WriteString(m.renderFilterRow(true))
	}

	// Table
	s.WriteString(m.table.View() + "\n")

	// Export prompt or message
	if m.showExport {
		s.WriteString("\n")
		if m.exportOverwrite {
			s.WriteString(components.WarningStyle.Render("File exists! Overwrite? (enter: yes, n: no)") + "\n")
		} else {
			s.WriteString(components.HelpKey.Render("Export to: ") + m.exportPath.View() + "\n")
		}
	} else if m.exportMsg != "" {
		s.WriteString("\n" + components.MutedStyle.Render(m.exportMsg) + "\n")
	}

	// Help
	var help string
	if m.showExport {
		help = "enter: export | esc: cancel"
	} else if m.mode == modeScan && m.showFilter {
		help = "tab/←/→: switch fields | enter: apply filter | ctrl+d: clear | esc: close filter"
	} else {
		help = "↑/↓: rows | ←/→: columns | /: filter | enter: view | d: describe | f: query | s: scan | r: refresh | ctrl+e: export | esc: back"
	}
	s.WriteString("\n" + components.MutedStyle.Render(help))

	return components.Container.Render(s.String())
}

// formatDuration formats a duration for display (e.g., "2m", "1h5m")
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
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

// updateFilterSuggestions extracts field names from loaded items for autocomplete
func (m *TableBrowserModel) updateFilterSuggestions() {
	seen := make(map[string]bool)
	var fields []string

	for _, item := range m.allItems {
		for k := range item {
			if !seen[k] {
				fields = append(fields, k)
				seen[k] = true
			}
		}
	}

	m.filterField.SetSuggestions(fields)
}

// buildFilterExpression creates an expression.ConditionBuilder from filter inputs
func (m TableBrowserModel) buildFilterExpression() expression.ConditionBuilder {
	field := m.filterField.Value()
	value := m.filterValue.Value()

	if field == "" || value == "" {
		return expression.ConditionBuilder{}
	}

	op := filterOperators[m.filterOpIdx].symbol
	name := expression.Name(field)

	// Convert value based on selected type
	var typedValue any
	switch filterTypes[m.filterTypeIdx] {
	case "Boolean":
		typedValue = value == "true"
	case "Number":
		if num, err := strconv.ParseFloat(value, 64); err == nil {
			typedValue = num
		} else {
			typedValue = value // fallback to string if parse fails
		}
	default: // String
		typedValue = value
	}

	switch op {
	case "=":
		return name.Equal(expression.Value(typedValue))
	case "<>":
		return name.NotEqual(expression.Value(typedValue))
	case "<":
		return name.LessThan(expression.Value(typedValue))
	case ">":
		return name.GreaterThan(expression.Value(typedValue))
	case "<=":
		return name.LessThanEqual(expression.Value(typedValue))
	case ">=":
		return name.GreaterThanEqual(expression.Value(typedValue))
	case "begins_with":
		return name.BeginsWith(value)
	case "contains":
		return name.Contains(value)
	default:
		return expression.ConditionBuilder{}
	}
}

// hasActiveFilter returns true if a valid filter is configured
func (m TableBrowserModel) hasActiveFilter() bool {
	return m.filterField.Value() != "" && m.filterValue.Value() != ""
}

// GetFilterState returns the current filter state for persistence
func (m TableBrowserModel) GetFilterState() storage.FilterState {
	return storage.FilterState{
		Field:    m.filterField.Value(),
		Type:     m.filterTypeIdx,
		Operator: m.filterOpIdx,
		Value:    m.filterValue.Value(),
	}
}

// clearFilter resets filter inputs
func (m *TableBrowserModel) clearFilter() {
	m.filterField.SetValue("")
	m.filterValue.SetValue("")
	m.filterOpIdx = 0
	m.showFilter = false
}

// Cache and pagination helpers

// rebuildAllItems flattens all cached pages into allItems
func (m *TableBrowserModel) rebuildAllItems() {
	m.allItems = nil
	for _, page := range m.cache {
		m.allItems = append(m.allItems, page.items...)
	}
}

// getVisibleItems returns items for the current logical page
func (m TableBrowserModel) getVisibleItems() []map[string]any {
	start := m.logicalPageIdx * LogicalPageSize
	if start >= len(m.allItems) {
		return nil
	}
	end := start + LogicalPageSize
	if end > len(m.allItems) {
		end = len(m.allItems)
	}
	return m.allItems[start:end]
}

// hasMorePhysicalPages returns true if there are more pages to fetch from DynamoDB
func (m TableBrowserModel) hasMorePhysicalPages() bool {
	if len(m.cache) == 0 {
		return false
	}
	return m.cache[len(m.cache)-1].hasMore
}

// fetchNextPhysicalPage returns a command to fetch the next physical page
func (m TableBrowserModel) fetchNextPhysicalPage() tea.Cmd {
	if len(m.cache) == 0 || !m.hasMorePhysicalPages() {
		return nil
	}
	lastPage := m.cache[len(m.cache)-1]

	if m.mode == modeQuery {
		input := dbtable.QueryInput{
			Key:           dbtable.Key{PK: m.pkInput.Value()},
			Index:         m.selectedIndex,
			PaginationKey: lastPage.nextKey,
			// No Limit - fetch full 1MB page
		}
		if m.skInput.Value() != "" {
			input.Key.SK = m.skInput.Value()
		}
		if m.hasActiveFilter() {
			input.FilterExpression = m.buildFilterExpression()
		}
		return m.client.QueryCmd(m.schema, input)
	}

	input := dbtable.ScanInput{
		PaginationKey: lastPage.nextKey,
		// No Limit - fetch full 1MB page
	}
	if m.hasActiveFilter() {
		input.FilterExpression = m.buildFilterExpression()
	}
	return m.client.ScanCmd(m.schema, input)
}


// buildTableForLogicalPage builds the table view for the current logical page
func (m *TableBrowserModel) buildTableForLogicalPage() {
	visibleItems := m.getVisibleItems()
	if len(m.columns) == 0 || len(visibleItems) == 0 {
		m.table.SetRows([]table.Row{})
		return
	}

	// Ensure columnOffset is valid
	if m.columnOffset >= len(m.columns) {
		m.columnOffset = len(m.columns) - 1
	}
	if m.columnOffset < 0 {
		m.columnOffset = 0
	}

	// Calculate column widths
	colWidths := make(map[string]int)
	for _, col := range m.columns {
		colWidths[col] = len(col)
	}
	for _, item := range visibleItems {
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
	totalWidth := m.width - 4
	if totalWidth < 40 {
		totalWidth = 80
	}

	// Determine visible columns
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
		if usedWidth+width+3 > totalWidth && len(visibleCols) > 0 {
			break
		}
		visibleCols = append(visibleCols, col)
		usedWidth += width + 3
	}

	if len(visibleCols) == 0 && len(m.columns) > 0 {
		idx := m.columnOffset
		if idx >= len(m.columns) {
			idx = len(m.columns) - 1
		}
		visibleCols = []string{m.columns[idx]}
	}

	// Build columns
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

	// Build rows
	rows := make([]table.Row, len(visibleItems))
	for i, item := range visibleItems {
		row := make(table.Row, len(visibleCols))
		for j, col := range visibleCols {
			val := m.formatValue(item[col])
			maxLen := columns[j].Width
			if len(val) > maxLen {
				val = val[:maxLen-1] + "…"
			}
			row[j] = val
		}
		rows[i] = row
	}

	cursor := m.table.Cursor()
	m.table.SetRows([]table.Row{})
	m.table.SetColumns(columns)
	m.table.SetRows(rows)
	if cursor >= len(rows) {
		cursor = 0
	}
	m.table.SetCursor(cursor)
}

// clearCache resets the cache (for new scan/query)
func (m *TableBrowserModel) clearCache() {
	m.cache = nil
	m.allItems = nil
	m.logicalPageIdx = 0
	m.fetchingMore = false
}

// Export helpers

// handleExportInput handles keys when export prompt is visible
func (m TableBrowserModel) handleExportInput(msg tea.KeyMsg) (TableBrowserModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showExport = false
		m.exportOverwrite = false
		m.exportPath.Blur()
		return m, nil

	case "enter":
		path := m.exportPath.Value()
		if path == "" {
			m.exportMsg = "Error: filename required"
			return m, nil
		}

		// Check if file exists
		if !m.exportOverwrite {
			if _, err := os.Stat(path); err == nil {
				m.exportOverwrite = true
				m.exportPath.Blur()
				return m, nil
			}
		}

		// Export all cached items
		count, err := m.exportAllItems(path)
		m.showExport = false
		m.exportOverwrite = false
		m.exportPath.Blur()
		if err != nil {
			m.exportMsg = fmt.Sprintf("Error: %v", err)
		} else {
			m.exportMsg = fmt.Sprintf("Exported %d items to %s", count, path)
		}
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return clearExportMsgMsg{}
		})

	case "n", "N":
		if m.exportOverwrite {
			m.exportOverwrite = false
			m.exportPath.Focus()
			return m, textinput.Blink
		}
	}

	if !m.exportOverwrite {
		var cmd tea.Cmd
		m.exportPath, cmd = m.exportPath.Update(msg)
		return m, cmd
	}
	return m, nil
}

// exportAllItems exports all cached items as-is (snapshot)
func (m *TableBrowserModel) exportAllItems(path string) (int, error) {
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Write all cached items to file
	for _, item := range m.allItems {
		jsonBytes, err := json.Marshal(item)
		if err != nil {
			return 0, err
		}
		if _, err := file.Write(jsonBytes); err != nil {
			return 0, err
		}
		if _, err := file.WriteString("\n"); err != nil {
			return 0, err
		}
	}

	return len(m.allItems), nil
}
