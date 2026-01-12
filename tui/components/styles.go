package components

import "github.com/charmbracelet/lipgloss"

// Theme detection
var IsDarkBackground = lipgloss.HasDarkBackground()

// Colors - adaptive based on terminal background
var (
	Primary   = lipgloss.Color("#7D56F4")
	Secondary = adaptiveColor("#6C757D", "#5A6268")
	Success   = lipgloss.Color("#28A745")
	Error     = lipgloss.Color("#DC3545")
	Warning   = lipgloss.Color("#FFC107")
	Muted     = adaptiveColor("#6C757D", "#495057")
	Light     = lipgloss.Color("#F8F9FA")
	Dark      = lipgloss.Color("#343A40")

	// Text colors that adapt to background
	TextPrimary   = adaptiveColor("#FFFFFF", "#212529")
	TextSecondary = adaptiveColor("#ADB5BD", "#495057")
)

// adaptiveColor returns dark theme color if dark background, light theme color otherwise
func adaptiveColor(darkTheme, lightTheme string) lipgloss.Color {
	if IsDarkBackground {
		return lipgloss.Color(darkTheme)
	}
	return lipgloss.Color(lightTheme)
}

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

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

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
			Foreground(statusBarFg()).
			Background(statusBarBg()).
			Padding(0, 1)

	HelpKey = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	HelpDesc = lipgloss.NewStyle().
			Foreground(Muted)

	// Status bar specific styles (always visible on dark bg)
	StatusBarKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	StatusBarDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ADB5BD"))
)

// JSON syntax highlighting styles
var (
	JSONKey = lipgloss.NewStyle().
		Foreground(adaptiveColor("#79C0FF", "#0550AE")) // Blue for keys

	JSONString = lipgloss.NewStyle().
			Foreground(adaptiveColor("#A5D6FF", "#0A3069")) // Light blue for strings

	JSONNumber = lipgloss.NewStyle().
			Foreground(adaptiveColor("#FFA657", "#953800")) // Orange for numbers

	JSONBool = lipgloss.NewStyle().
			Foreground(adaptiveColor("#FF7B72", "#CF222E")) // Red for booleans

	JSONNull = lipgloss.NewStyle().
			Foreground(Muted) // Gray for null

	JSONBracket = lipgloss.NewStyle().
			Foreground(adaptiveColor("#8B949E", "#57606A")) // Subtle for brackets
)

func statusBarFg() lipgloss.Color {
	if IsDarkBackground {
		return Light // white text on dark bg
	}
	return Light // white text on dark bg (inverted for light terminals)
}

func statusBarBg() lipgloss.Color {
	if IsDarkBackground {
		return Dark // dark background
	}
	return lipgloss.Color("#343A40") // dark background (inverted for light terminals)
}
