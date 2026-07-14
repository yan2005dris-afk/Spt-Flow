package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// renderURLInputModal renders the URL paste modal overlay.
func (m Model) renderURLInputModal() string {
	width := 60
	height := 5
	x := (m.Width - width) / 2
	if x < 0 {
		x = 0
	}
	y := (m.Height - height) / 2
	if y < 0 {
		y = 0
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.Theme.MenuBorder).
		Width(width).
		Height(height)

	content := fmt.Sprintf("Paste Spotify URL and press Enter:\n\n  %s", m.URLInputValue)
	if m.URLInputValue == "" {
		content = "Paste Spotify URL and press Enter:\n\n  (waiting for input...)"
	}

	modal := lipgloss.Place(m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		borderStyle.Render(content),
	)

	// Also show the URL value at the bottom of the screen for feedback
	return lipgloss.JoinVertical(lipgloss.Left,
		modal,
		lipgloss.NewStyle().
			Foreground(m.Theme.Waiting).
			Render("URL: "+m.URLInputValue),
	)
}