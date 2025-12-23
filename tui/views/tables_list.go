package views

import (
	"fmt"

	"github.com/orhayat/dynamodb-go/tui/components"
	"github.com/orhayat/dynamodb-go/tui/dynamo"
	"github.com/orhayat/dynamodb-go/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

// TablesListModel displays a list of DynamoDB tables
type TablesListModel struct {
	client  *dynamo.Client
	tables  []string
	cursor  int
	loading components.Loading
	err     error

	width  int
	height int
}

// NewTablesListModel creates a new tables list view
func NewTablesListModel(client *dynamo.Client) TablesListModel {
	return TablesListModel{
		client:  client,
		loading: components.NewLoading("Loading tables..."),
	}
}

// SetSize sets the view dimensions
func (m *TablesListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init starts loading tables
func (m TablesListModel) Init() tea.Cmd {
	return tea.Batch(
		m.loading.Init(),
		m.client.ListTablesCmd(),
	)
}

// Update handles messages
func (m TablesListModel) Update(msg tea.Msg) (TablesListModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case dynamo.TablesListMsg:
		m.tables = msg.Tables
		m.err = nil

	case dynamo.ErrorMsg:
		m.err = msg.Err

	case tea.KeyMsg:
		if m.tables == nil {
			// Still loading, ignore navigation
			break
		}

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.tables)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter", "s":
			// Scan table (default)
			if len(m.tables) > 0 {
				tableName := m.tables[m.cursor]
				return m, func() tea.Msg {
					return messages.NavigateToTableMsg{TableName: tableName, InitialMode: messages.TableModeScan}
				}
			}
		case "f":
			// Query table
			if len(m.tables) > 0 {
				tableName := m.tables[m.cursor]
				return m, func() tea.Msg {
					return messages.NavigateToTableMsg{TableName: tableName, InitialMode: messages.TableModeQuery}
				}
			}
		case "d":
			// Describe table
			if len(m.tables) > 0 {
				tableName := m.tables[m.cursor]
				return m, func() tea.Msg {
					return messages.NavigateToTableMsg{TableName: tableName, InitialMode: messages.TableModeDescribe}
				}
			}
		case "r":
			// Refresh
			m.tables = nil
			m.err = nil
			return m, tea.Batch(
				m.loading.Init(),
				m.client.ListTablesCmd(),
			)
		}
	}

	// Update loading spinner
	if m.tables == nil && m.err == nil {
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the tables list
func (m TablesListModel) View() string {
	var s string

	s += components.Title.Render("DynamoDB Tables") + "\n\n"

	if m.err != nil {
		s += components.ErrorStyle.Render("Error: "+m.err.Error()) + "\n"
		s += components.MutedStyle.Render("Press 'r' to retry") + "\n"
		return components.Container.Render(s)
	}

	if m.tables == nil {
		s += m.loading.View() + "\n"
		return components.Container.Render(s)
	}

	if len(m.tables) == 0 {
		s += components.MutedStyle.Render("No tables found") + "\n"
		return components.Container.Render(s)
	}

	// Calculate visible range based on height
	visibleItems := m.height - 6 // Account for title, padding
	if visibleItems < 1 {
		visibleItems = 10
	}

	start := 0
	if m.cursor >= visibleItems {
		start = m.cursor - visibleItems + 1
	}
	end := start + visibleItems
	if end > len(m.tables) {
		end = len(m.tables)
	}

	for i := start; i < end; i++ {
		cursor := "  "
		if i == m.cursor {
			cursor = components.Cursor.Render("> ")
		}

		tableName := m.tables[i]
		if i == m.cursor {
			tableName = components.SelectedItem.Render(tableName)
		} else {
			tableName = components.NormalItem.Render(tableName)
		}
		s += cursor + tableName + "\n"
	}

	// Show position indicator
	s += "\n" + components.MutedStyle.Render(
		fmt.Sprintf("%d of %d tables", m.cursor+1, len(m.tables)),
	)

	return components.Container.Render(s)
}
