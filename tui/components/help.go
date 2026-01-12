package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpOverlay renders a centered help modal with keybindings
type HelpOverlay struct {
	width    int
	height   int
	bindings []KeyBinding
	title    string
}

// NewHelpOverlay creates a new help overlay
func NewHelpOverlay() HelpOverlay {
	return HelpOverlay{}
}

// SetSize sets the terminal dimensions for centering
func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// SetBindings sets the keybindings to display
func (h *HelpOverlay) SetBindings(bindings []KeyBinding, title string) {
	h.bindings = bindings
	h.title = title
}

// View renders the help overlay
func (h HelpOverlay) View() string {
	// Build keybinding rows
	var rows []string
	for _, b := range h.bindings {
		row := HelpKey.Render(padRight(b.Key, 10)) + " " + HelpDesc.Render(b.Desc)
		rows = append(rows, row)
	}

	// Add dismiss hint
	rows = append(rows, "")
	rows = append(rows, MutedStyle.Render("Press ? or esc to close"))

	content := strings.Join(rows, "\n")

	// Box style - adaptive background
	boxBg := lipgloss.Color("#1a1a2e") // dark mode
	if !IsDarkBackground {
		boxBg = lipgloss.Color("#FFFFFF") // light mode: white
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 2).
		Background(boxBg)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	box := boxStyle.Render(titleStyle.Render(h.title) + "\n\n" + content)

	// Center the box
	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)

	if h.width == 0 || h.height == 0 {
		return box
	}

	// Calculate padding for centering
	padLeft := (h.width - boxWidth) / 2
	padTop := (h.height - boxHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	return lipgloss.NewStyle().
		PaddingLeft(padLeft).
		PaddingTop(padTop).
		Render(box)
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

// FullBindingsForView returns complete keybindings for a given view
func FullBindingsForView(view string) ([]KeyBinding, string) {
	switch view {
	case "tables":
		return []KeyBinding{
			{Key: "j / ↓", Desc: "Move down"},
			{Key: "k / ↑", Desc: "Move up"},
			{Key: "enter / s", Desc: "Scan table"},
			{Key: "f", Desc: "Query table"},
			{Key: "d", Desc: "Describe table"},
			{Key: "r", Desc: "Refresh table list"},
			{Key: "q / esc", Desc: "Quit"},
			{Key: "ctrl+q", Desc: "Quit (global)"},
		}, "Tables List"
	case "browser_describe":
		return []KeyBinding{
			{Key: "enter / s", Desc: "Scan table"},
			{Key: "f", Desc: "Query table"},
			{Key: "q / esc", Desc: "Back to tables list"},
			{Key: "ctrl+q", Desc: "Quit"},
		}, "Table Info"
	case "browser":
		return []KeyBinding{
			{Key: "j / ↓", Desc: "Move down"},
			{Key: "k / ↑", Desc: "Move up"},
			{Key: "h / ←", Desc: "Scroll columns left"},
			{Key: "l / →", Desc: "Scroll columns right"},
			{Key: "n", Desc: "Next page"},
			{Key: "p", Desc: "Previous page"},
			{Key: "d", Desc: "Describe table info"},
			{Key: "f", Desc: "Find/query mode"},
			{Key: "s", Desc: "Switch to scan mode"},
			{Key: "ctrl+c", Desc: "Copy item as JSON to clipboard"},
			{Key: "ctrl+e", Desc: "Export items to JSONL"},
			{Key: "enter", Desc: "View item details"},
			{Key: "q / esc", Desc: "Back to tables list"},
			{Key: "ctrl+q", Desc: "Quit"},
		}, "Table Browser"
	case "browser_query":
		return []KeyBinding{
			{Key: "↑ / ↓ / tab", Desc: "Switch between Index/PK/SK"},
			{Key: "enter", Desc: "Open index dropdown / Execute query"},
			{Key: "j / k", Desc: "Navigate dropdown (when open)"},
			{Key: "ctrl+v", Desc: "Paste copied key value"},
			{Key: "ctrl+d", Desc: "Clear current field"},
			{Key: "esc", Desc: "Close dropdown / Cancel input"},
			{Key: "ctrl+q", Desc: "Quit"},
		}, "Query Input"
	case "detail":
		return []KeyBinding{
			{Key: "j / ↓", Desc: "Scroll down"},
			{Key: "k / ↑", Desc: "Scroll up"},
			{Key: "d", Desc: "Scroll half page down"},
			{Key: "u", Desc: "Scroll half page up"},
			{Key: "g", Desc: "Go to top"},
			{Key: "G", Desc: "Go to bottom"},
			{Key: "ctrl+c", Desc: "Copy item as JSON to clipboard"},
			{Key: "q / esc", Desc: "Back to browser"},
			{Key: "ctrl+q", Desc: "Quit"},
		}, "Item Detail"
	default:
		return CommonBindings(), "Help"
	}
}
