package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Autocomplete is a text input with dropdown suggestions
type Autocomplete struct {
	textInput   textinput.Model
	suggestions []string // all possible suggestions
	filtered    []string // suggestions matching current input
	showDropdown bool
	selectedIdx  int // index in filtered list
	width        int
}

// NewAutocomplete creates a new autocomplete component
func NewAutocomplete(placeholder string) Autocomplete {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256

	return Autocomplete{
		textInput:    ti,
		suggestions:  []string{},
		filtered:     []string{},
		showDropdown: false,
		selectedIdx:  0,
		width:        30,
	}
}

// SetSuggestions updates the available suggestions
func (a *Autocomplete) SetSuggestions(suggestions []string) {
	a.suggestions = suggestions
	a.updateFiltered()
}

// SetWidth sets the component width
func (a *Autocomplete) SetWidth(width int) {
	a.width = width
	a.textInput.Width = width
}

// SetValue sets the input value
func (a *Autocomplete) SetValue(value string) {
	a.textInput.SetValue(value)
	a.updateFiltered()
}

// Value returns the current input value
func (a Autocomplete) Value() string {
	return a.textInput.Value()
}

// Focus focuses the text input
func (a *Autocomplete) Focus() tea.Cmd {
	return a.textInput.Focus()
}

// Blur removes focus from the text input
func (a *Autocomplete) Blur() {
	a.textInput.Blur()
	a.showDropdown = false
}

// Focused returns whether the input is focused
func (a Autocomplete) Focused() bool {
	return a.textInput.Focused()
}

// updateFiltered filters suggestions based on current input
func (a *Autocomplete) updateFiltered() {
	a.filtered = FilterSuggestions(a.textInput.Value(), a.suggestions)
	// Reset selection if out of bounds
	if a.selectedIdx >= len(a.filtered) {
		a.selectedIdx = 0
	}
}

// FilterSuggestions filters suggestions based on input (case-insensitive prefix match)
// Exported for testing
func FilterSuggestions(input string, suggestions []string) []string {
	if input == "" {
		return suggestions
	}

	inputLower := strings.ToLower(input)
	var result []string

	// First pass: prefix matches (higher priority)
	for _, s := range suggestions {
		if strings.HasPrefix(strings.ToLower(s), inputLower) {
			result = append(result, s)
		}
	}

	// Second pass: contains matches (lower priority, avoid duplicates)
	for _, s := range suggestions {
		sLower := strings.ToLower(s)
		if !strings.HasPrefix(sLower, inputLower) && strings.Contains(sLower, inputLower) {
			result = append(result, s)
		}
	}

	return result
}

// GetSelectedSuggestion returns the currently selected suggestion, or empty if none
func (a Autocomplete) GetSelectedSuggestion() string {
	if len(a.filtered) == 0 || a.selectedIdx >= len(a.filtered) {
		return ""
	}
	return a.filtered[a.selectedIdx]
}

// Update handles key messages
func (a Autocomplete) Update(msg tea.Msg) (Autocomplete, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "ctrl+p":
			if a.showDropdown && len(a.filtered) > 0 {
				a.selectedIdx--
				if a.selectedIdx < 0 {
					a.selectedIdx = len(a.filtered) - 1
				}
				return a, nil
			}

		case "down", "ctrl+n":
			if a.showDropdown && len(a.filtered) > 0 {
				a.selectedIdx++
				if a.selectedIdx >= len(a.filtered) {
					a.selectedIdx = 0
				}
				return a, nil
			} else if !a.showDropdown && len(a.filtered) > 0 {
				// Open dropdown on down arrow
				a.showDropdown = true
				return a, nil
			}

		case "enter":
			if a.showDropdown && len(a.filtered) > 0 && a.selectedIdx < len(a.filtered) {
				// Select the highlighted suggestion
				a.textInput.SetValue(a.filtered[a.selectedIdx])
				a.showDropdown = false
				a.updateFiltered()
				return a, nil
			}

		case "esc":
			if a.showDropdown {
				a.showDropdown = false
				return a, nil
			}

		case "tab", "shift+tab":
			// Let parent handle tab navigation
			a.showDropdown = false
			return a, nil
		}
	}

	// Forward to text input
	var cmd tea.Cmd
	a.textInput, cmd = a.textInput.Update(msg)

	// Update filtered list and show dropdown when typing
	a.updateFiltered()

	// Show dropdown when we have matches, hide when empty
	if a.textInput.Focused() && len(a.filtered) > 0 && a.textInput.Value() != "" {
		a.showDropdown = true
	} else if len(a.filtered) == 0 {
		a.showDropdown = false
	}

	return a, cmd
}

// View renders the autocomplete component
func (a Autocomplete) View() string {
	var s strings.Builder

	// Render text input
	s.WriteString(a.textInput.View())

	// Render dropdown if visible
	if a.showDropdown && len(a.filtered) > 0 {
		s.WriteString("\n")
		s.WriteString(a.renderDropdown())
	}

	return s.String()
}

// renderDropdown renders the suggestion dropdown
func (a Autocomplete) renderDropdown() string {
	if len(a.filtered) == 0 {
		return ""
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(0, 1)

	// Limit visible items
	maxVisible := 5
	start := 0
	end := len(a.filtered)

	if end > maxVisible {
		// Scroll to keep selected item visible
		if a.selectedIdx >= maxVisible {
			start = a.selectedIdx - maxVisible + 1
		}
		end = start + maxVisible
		if end > len(a.filtered) {
			end = len(a.filtered)
			start = end - maxVisible
		}
	}

	var rows []string
	for i := start; i < end; i++ {
		item := a.filtered[i]
		if i == a.selectedIdx {
			item = SelectedItem.Render(item)
		} else {
			item = "  " + item
		}
		rows = append(rows, item)
	}

	return boxStyle.Render(strings.Join(rows, "\n"))
}

// DropdownVisible returns whether the dropdown is currently shown
func (a Autocomplete) DropdownVisible() bool {
	return a.showDropdown
}

// Position returns cursor position in the text input
func (a Autocomplete) Position() int {
	return a.textInput.Position()
}
