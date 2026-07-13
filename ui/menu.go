package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	ChoiceStartLibrespot      = "start-librespot"
	ChoiceStartSpotifyDesktop = "start-spotify-desktop"
	ChoiceTUIOnly             = "tui-only"
	ChoiceCheckStatus         = "check-status"
	ChoiceCycleTheme          = "cycle-theme"
	ChoiceHelp                = "help"
)

const menuWidth = 40

type MenuModel struct {
	Selected  int
	Width     int
	Height    int
	ThemeName string
}

func NewMenuModel(height, width int) MenuModel {
	return MenuModel{
		Selected: 0,
		Width:    width,
		Height:   height,
	}
}

func (m MenuModel) Init() tea.Cmd {
	return nil
}

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.Selected = (m.Selected + 1) % 6
		case "k", "up":
			m.Selected = (m.Selected - 1 + 6) % 6
		case "enter":
			choices := []string{ChoiceStartLibrespot, ChoiceTUIOnly, ChoiceCheckStatus, ChoiceCycleTheme, ChoiceHelp, ChoiceStartSpotifyDesktop}
			return m, func() tea.Msg { return MenuChoiceMsg{Choice: choices[m.Selected]} }
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m MenuModel) View() string {
	menuBox := lipgloss.NewStyle().
		Width(menuWidth).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.MenuBorder).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.MenuTitle).
		Bold(true).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.MenuDim).
		Align(lipgloss.Center)

	optionStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.MenuOption)

	selectedStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.MenuSelected).
		Bold(true)

	footerStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.MenuDim).
		Align(lipgloss.Center)

	menuContent := fmt.Sprintf("%s\n%s\n\n%s\n%s\n%s\n%s\n%s\n\n%s",
		titleStyle.Render("Spt-Flow"),
		subtitleStyle.Render("Spotify TUI Mirror"),
		renderMenuOption(0, "Start with Librespot + TUI", m.Selected, optionStyle, selectedStyle),
		renderMenuOption(1, "Open TUI only", m.Selected, optionStyle, selectedStyle),
		renderMenuOption(2, "Check Spotify status", m.Selected, optionStyle, selectedStyle),
		renderMenuOption(3, "Help / Keybindings", m.Selected, optionStyle, selectedStyle),
		renderMenuOption(4, "Start with Spotify Desktop", m.Selected, optionStyle, selectedStyle),
		footerStyle.Render("↑↓ navigate · Enter select · q quit"),
	)

	return menuBox.Render(menuContent)
}

func renderMenuOption(index int, text string, selected int, normalStyle, selectedStyle lipgloss.Style) string {
	prefix := "  "
	style := normalStyle
	if index == selected {
		prefix = "> "
		style = selectedStyle
	}
	return style.Render(prefix + text)
}

// renderMenuView renders the menu centered in the terminal
func renderMenuView(m Model) string {
	if m.Width == 0 || m.Height == 0 {
		m.Width = 80
		m.Height = 24
	}

	boxWidth := menuWidth
	if m.Width-4 < menuWidth {
		boxWidth = m.Width - 4
	}

	menuBox := lipgloss.NewStyle().
		Width(boxWidth).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.Theme.MenuBorder).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuTitle).
		Bold(true).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuDim).
		Align(lipgloss.Center)

	optionStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuOption)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuSelected).
		Bold(true)

	footerStyle := lipgloss.NewStyle().
		Foreground(m.Theme.MenuDim).
		Align(lipgloss.Center)

	themeName := m.Theme.Name()
	menuContent := fmt.Sprintf("%s\n%s\n\n%s\n%s\n%s\n%s\n%s\n%s\n\n%s",
		titleStyle.Render("Spt-Flow"),
		subtitleStyle.Render("Spotify TUI Mirror"),
		renderMenuOption(0, "Start with Librespot + TUI", m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(1, "Open TUI only", m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(2, "Check Spotify status", m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(3, fmt.Sprintf("Theme: %s", themeName), m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(4, "Help / Keybindings", m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(5, "Start with Spotify Desktop", m.SelectedMenuOption, optionStyle, selectedStyle),
		footerStyle.Render("↑↓ navigate · Enter select · q quit"),
	)

	// Center the box
	horizontalMargin := (m.Width - boxWidth) / 2
	if horizontalMargin < 0 {
		horizontalMargin = 0
	}

	_ = horizontalMargin // silence unused variable warning

	verticalContent := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		menuBox.Render(menuContent),
		"",
	)

	// Wrap with horizontal margins
	result := lipgloss.NewStyle().
		Width(m.Width).
		Height(m.Height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(verticalContent)

	return result
}

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
	overlayContent.WriteString(fmt.Sprintf("  %s  Quit", keyStyle.Render("q / Ctrl+C")))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s   Show this help", keyStyle.Render("?")))
	overlayContent.WriteString("\n\n")
	overlayContent.WriteString(descStyle.Render("Playback:"))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s   Play/Pause", keyStyle.Render("Space")))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s    Next track", keyStyle.Render("n / l")))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s  Previous track", keyStyle.Render("p / h")))
	overlayContent.WriteString("\n\n")
	overlayContent.WriteString(descStyle.Render("Volume:"))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s    Increase volume", keyStyle.Render("+ / =")))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s    Decrease volume", keyStyle.Render("-")))
	overlayContent.WriteString("\n\n")
	overlayContent.WriteString(descStyle.Render("Navigation:"))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s  Scroll lyrics up", keyStyle.Render("k / ↑")))
	overlayContent.WriteString("\n")
	overlayContent.WriteString(fmt.Sprintf("  %s  Scroll lyrics down", keyStyle.Render("j / ↓")))

	boxContent := boxStyle.Render(overlayContent.String())

	return lipgloss.NewStyle().
		Width(m.Width).
		Height(m.Height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(boxContent)
}

// menuTickCmd returns a tea.Cmd that sends MenuTimerMsg after a duration
func menuTickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return MenuTimerMsg{}
	})
}
