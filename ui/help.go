package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderKeybindingsOverlay renders a full-screen keybindings overlay
func renderKeybindingsOverlay(m Model) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuTitle).
		Bold(true).
		Align(lipgloss.Center)

	keyStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuSelected).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuOption)

	boxStyle := lipgloss.NewStyle().
		Width(50).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.Theme.MenuBorder).
		Padding(1, 2)

	var overlayContent strings.Builder
	overlayContent.WriteString(titleStyle.Render("Keybindings"))
	overlayContent.WriteString("\n\n")
	overlayContent.WriteString(descStyle.Render("Global:"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s  Quit", keyStyle.Render("q / Ctrl+C"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s   Show this help", keyStyle.Render("?"))
	overlayContent.WriteString("\n\n")
	overlayContent.WriteString(descStyle.Render("Playback:"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s   Play/Pause", keyStyle.Render("Space"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s    Next track", keyStyle.Render("n / l"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s  Previous track", keyStyle.Render("p / h"))
	overlayContent.WriteString("\n\n")
	overlayContent.WriteString(descStyle.Render("Volume:"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s    Increase volume", keyStyle.Render("+ / ="))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s    Decrease volume", keyStyle.Render("-"))
	overlayContent.WriteString("\n\n")
	overlayContent.WriteString(descStyle.Render("Navigation:"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s  Scroll lyrics up", keyStyle.Render("k / ↑"))
	overlayContent.WriteString("\n")
	fmt.Fprintf(&overlayContent, "  %s  Scroll lyrics down", keyStyle.Render("j / ↓"))

	boxContent := boxStyle.Render(overlayContent.String())

	return lipgloss.NewStyle().
		Width(m.Width).
		Height(m.Height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(boxContent)
}