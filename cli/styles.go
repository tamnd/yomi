package cli

import "charm.land/lipgloss/v2"

// Styles for the human-readable progress and summary lines. They degrade to
// plain text when the terminal has no colour profile.
var (
	styleTitle  = lipgloss.NewStyle().Bold(true)
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
