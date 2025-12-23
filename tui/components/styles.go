package components

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	Primary   = lipgloss.Color("#7D56F4")
	Secondary = lipgloss.Color("#6C757D")
	Success   = lipgloss.Color("#28A745")
	Error     = lipgloss.Color("#DC3545")
	Warning   = lipgloss.Color("#FFC107")
	Muted     = lipgloss.Color("#6C757D")
	Light     = lipgloss.Color("#F8F9FA")
	Dark      = lipgloss.Color("#343A40")
)

// Base styles
var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	Subtitle = lipgloss.NewStyle().
			Foreground(Secondary).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)
)

// List styles
var (
	SelectedItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Primary).
			Padding(0, 1)

	NormalItem = lipgloss.NewStyle().
			Padding(0, 1)

	Cursor = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)
)

// Table styles
var (
	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(Secondary)

	TableCell = lipgloss.NewStyle().
			Padding(0, 1)

	TableSelectedRow = lipgloss.NewStyle().
				Background(Primary).
				Foreground(lipgloss.Color("#FFFFFF"))
)

// Layout styles
var (
	Container = lipgloss.NewStyle().
			Padding(1, 2)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(Light).
			Background(Dark).
			Padding(0, 1)

	HelpKey = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	HelpDesc = lipgloss.NewStyle().
			Foreground(Muted)
)
