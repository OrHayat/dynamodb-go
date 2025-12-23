package views

import (
	"encoding/json"
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/orhayat/dynamodb-go/table"
	"github.com/orhayat/dynamodb-go/tui/components"
	"github.com/orhayat/dynamodb-go/tui/messages"
)

// ItemDetailModel displays a single item as JSON
type ItemDetailModel struct {
	item       map[string]any
	key        table.Key
	jsonString string
	viewport   viewport.Model
	copied     bool

	width  int
	height int
}

// NewItemDetailModel creates a new item detail view
func NewItemDetailModel(item map[string]any, key table.Key) ItemDetailModel {
	jsonBytes, _ := json.MarshalIndent(item, "", "  ")
	jsonStr := string(jsonBytes)

	return ItemDetailModel{
		item:       item,
		key:        key,
		jsonString: jsonStr,
	}
}

// SetSize sets the view dimensions
func (m *ItemDetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	headerHeight := 4 // Title + key info + spacing
	m.viewport = viewport.New(width-4, height-headerHeight)
	m.viewport.SetContent(m.jsonString)
}

// Init initializes the view
func (m ItemDetailModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m ItemDetailModel) Update(msg tea.Msg) (ItemDetailModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return messages.NavigateBackMsg{} }
		case "y":
			// Copy to clipboard
			if err := clipboard.WriteAll(m.jsonString); err == nil {
				m.copied = true
			}
			return m, nil
		case "j", "down":
			m.viewport.LineDown(1)
		case "k", "up":
			m.viewport.LineUp(1)
		case "d", "ctrl+d":
			m.viewport.HalfViewDown()
		case "u", "ctrl+u":
			m.viewport.HalfViewUp()
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the item detail
func (m ItemDetailModel) View() string {
	var s string

	s += components.Title.Render("Item Detail") + "\n"

	// Show key info
	keyInfo := fmt.Sprintf("PK: %v", m.key.PK)
	if m.key.SK != nil {
		keyInfo += fmt.Sprintf("  SK: %v", m.key.SK)
	}
	s += components.MutedStyle.Render(keyInfo) + "\n\n"

	// Show copied indicator
	if m.copied {
		s += components.SuccessStyle.Render("Copied to clipboard!") + "\n"
	}

	// JSON viewport
	s += m.viewport.View()

	// Scroll indicator
	scrollPercent := m.viewport.ScrollPercent() * 100
	s += "\n" + components.MutedStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent))

	return components.Container.Render(s)
}
