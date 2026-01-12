package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// KeyBinding represents a key and its description
type KeyBinding struct {
	Key  string
	Desc string
}

// StatusBar displays context info and keybindings at the bottom
type StatusBar struct {
	width    int
	profile  string
	region   string
	table    string
	bindings []KeyBinding
}

// NewStatusBar creates a new status bar
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// SetSize sets the width of the status bar
func (s *StatusBar) SetSize(width int) {
	s.width = width
}

// SetContext sets the AWS context (profile, region, table)
func (s *StatusBar) SetContext(profile, region, table string) {
	s.profile = profile
	s.region = region
	s.table = table
}

// SetBindings sets the keybindings to display
func (s *StatusBar) SetBindings(bindings []KeyBinding) {
	s.bindings = bindings
}

// View renders the status bar
func (s StatusBar) View() string {
	// Context section (left side)
	var contextParts []string
	if s.profile != "" {
		contextParts = append(contextParts, "profile:"+s.profile)
	}
	if s.region != "" {
		contextParts = append(contextParts, "region:"+s.region)
	}
	if s.table != "" {
		contextParts = append(contextParts, "table:"+s.table)
	}
	context := strings.Join(contextParts, " | ")

	// Keybindings section (right side)
	var bindingParts []string
	for _, b := range s.bindings {
		bindingParts = append(bindingParts, b.Key+" "+b.Desc)
	}
	bindings := strings.Join(bindingParts, "  ")

	// If no width set, just render without spacing
	if s.width == 0 {
		return StatusBarStyle.Render(context + "  " + bindings)
	}

	// Calculate spacing
	contextWidth := lipgloss.Width(context)
	bindingsWidth := lipgloss.Width(bindings)
	spacing := s.width - contextWidth - bindingsWidth - 2 // -2 for padding
	if spacing < 1 {
		spacing = 1
	}

	return StatusBarStyle.Width(s.width).Render(context + strings.Repeat(" ", spacing) + bindings)
}

// CommonBindings returns common keybindings used across views
func CommonBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}
}

// TablesListBindings returns keybindings for tables list view
func TablesListBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "j/k", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "r", Desc: "refresh"},
		{Key: "q/esc", Desc: "quit"},
	}
}

// BrowserBindings returns keybindings for table browser view
func BrowserBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "j/k", Desc: "navigate"},
		{Key: "h/l", Desc: "scroll cols"},
		{Key: "n/p", Desc: "page"},
		{Key: "f", Desc: "find"},
		{Key: "ctrl+c", Desc: "copy"},
		{Key: "ctrl+e", Desc: "export"},
		{Key: "q", Desc: "back"},
	}
}

// DetailBindings returns keybindings for item detail view
func DetailBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "j/k", Desc: "scroll"},
		{Key: "y", Desc: "copy JSON"},
		{Key: "q", Desc: "back"},
	}
}
