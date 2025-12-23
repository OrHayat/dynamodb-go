package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Loading is a simple spinner component for async operations
type Loading struct {
	spinner spinner.Model
	message string
}

// NewLoading creates a new loading spinner
func NewLoading(message string) Loading {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Primary)
	return Loading{
		spinner: s,
		message: message,
	}
}

// SetMessage updates the loading message
func (l *Loading) SetMessage(msg string) {
	l.message = msg
}

// Init starts the spinner
func (l Loading) Init() tea.Cmd {
	return l.spinner.Tick
}

// Update handles spinner updates
func (l Loading) Update(msg tea.Msg) (Loading, tea.Cmd) {
	var cmd tea.Cmd
	l.spinner, cmd = l.spinner.Update(msg)
	return l, cmd
}

// View renders the loading spinner
func (l Loading) View() string {
	return l.spinner.View() + " " + l.message
}

// Tick returns the spinner tick command
func (l Loading) Tick() tea.Cmd {
	return l.spinner.Tick
}
