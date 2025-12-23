package app

import (
	"log/slog"

	"github.com/orhayat/dynamodb-go/tui/components"
	"github.com/orhayat/dynamodb-go/tui/dynamo"
	"github.com/orhayat/dynamodb-go/tui/messages"
	"github.com/orhayat/dynamodb-go/tui/views"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the root application model
type Model struct {
	client    *dynamo.Client
	logger    *slog.Logger
	ctx       Context
	statusbar components.StatusBar

	// Views
	tablesList   views.TablesListModel
	tableBrowser views.TableBrowserModel
	itemDetail   views.ItemDetailModel

	// Current view
	currentView View

	// Terminal size
	width  int
	height int

	// Help overlay
	showHelp    bool
	helpOverlay components.HelpOverlay

	// Quitting flag
	quitting bool
}

// New creates a new app model
func New(client *dynamo.Client, logger *slog.Logger, profile, region string) Model {
	ctx := Context{
		Profile: profile,
		Region:  region,
	}

	statusbar := components.NewStatusBar()
	statusbar.SetContext(profile, region, "")
	statusbar.SetBindings(components.TablesListBindings())

	return Model{
		client:      client,
		logger:      logger,
		ctx:         ctx,
		statusbar:   statusbar,
		currentView: ViewTablesList,
		tablesList:  views.NewTablesListModel(client),
		helpOverlay: components.NewHelpOverlay(),
	}
}

// Init initializes the app
func (m Model) Init() tea.Cmd {
	return m.tablesList.Init()
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusbar.SetSize(msg.Width)
		m.helpOverlay.SetSize(msg.Width, msg.Height)

		// Propagate to active view
		switch m.currentView {
		case ViewTablesList:
			m.tablesList.SetSize(msg.Width, msg.Height-2) // -2 for statusbar
		case ViewTableBrowser:
			m.tableBrowser.SetSize(msg.Width, msg.Height-2)
		case ViewItemDetail:
			m.itemDetail.SetSize(msg.Width, msg.Height-2)
		}

	case tea.KeyMsg:
		keyStr := msg.String()
		m.logger.Debug("key", "key", keyStr)

		// Global quit handler
		if keyStr == "ctrl+q" {
			m.quitting = true
			return m, tea.Quit
		}

		// Help overlay toggle
		if keyStr == "?" {
			m.showHelp = !m.showHelp
			if m.showHelp {
				viewName := m.currentViewName()
				bindings, title := components.FullBindingsForView(viewName)
				m.helpOverlay.SetBindings(bindings, title)
			}
			return m, nil
		}

		// When help is shown, only esc closes it
		if m.showHelp {
			if keyStr == "esc" {
				m.showHelp = false
			}
			return m, nil
		}

		// Quit from tables list with q or esc
		if (msg.String() == "q" || msg.String() == "esc") && m.currentView == ViewTablesList {
			m.quitting = true
			return m, tea.Quit
		}

	case messages.NavigateToTableMsg:
		m.ctx.TableName = msg.TableName
		m.currentView = ViewTableBrowser
		m.tableBrowser = views.NewTableBrowserModel(m.client, m.logger, msg.TableName, int(msg.InitialMode))
		m.tableBrowser.SetSize(m.width, m.height-2)
		m.statusbar.SetContext(m.ctx.Profile, m.ctx.Region, msg.TableName)
		m.statusbar.SetBindings(components.BrowserBindings())
		return m, m.tableBrowser.Init()

	case messages.NavigateToItemMsg:
		m.ctx.CurrentItem = msg.Item
		m.ctx.ItemKey = msg.Key
		m.currentView = ViewItemDetail
		m.itemDetail = views.NewItemDetailModel(msg.Item, msg.Key, msg.PkName, msg.SkName)
		m.itemDetail.SetSize(m.width, m.height-2)
		m.statusbar.SetBindings(components.DetailBindings())
		return m, m.itemDetail.Init()

	case messages.NavigateBackMsg:
		switch m.currentView {
		case ViewTableBrowser:
			m.currentView = ViewTablesList
			m.ctx.TableName = ""
			m.ctx.TableSchema = nil
			m.statusbar.SetContext(m.ctx.Profile, m.ctx.Region, "")
			m.statusbar.SetBindings(components.TablesListBindings())
		case ViewItemDetail:
			m.currentView = ViewTableBrowser
			m.ctx.CurrentItem = nil
			m.statusbar.SetBindings(components.BrowserBindings())
		}
		return m, nil
	}

	// Route to active view
	var cmd tea.Cmd
	switch m.currentView {
	case ViewTablesList:
		m.tablesList, cmd = m.tablesList.Update(msg)
	case ViewTableBrowser:
		m.tableBrowser, cmd = m.tableBrowser.Update(msg)
	case ViewItemDetail:
		m.itemDetail, cmd = m.itemDetail.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the app
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// If help is shown, render overlay only
	if m.showHelp {
		return m.helpOverlay.View()
	}

	var content string
	switch m.currentView {
	case ViewTablesList:
		content = m.tablesList.View()
	case ViewTableBrowser:
		content = m.tableBrowser.View()
	case ViewItemDetail:
		content = m.itemDetail.View()
	}

	// Layout: content + statusbar
	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		m.statusbar.View(),
	)
}

// currentViewName returns the current view name for help bindings
func (m Model) currentViewName() string {
	switch m.currentView {
	case ViewTablesList:
		return "tables"
	case ViewTableBrowser:
		if m.tableBrowser.IsInDescribeMode() {
			return "browser_describe"
		}
		if m.tableBrowser.IsInQueryInput() {
			return "browser_query"
		}
		return "browser"
	case ViewItemDetail:
		return "detail"
	default:
		return ""
	}
}
