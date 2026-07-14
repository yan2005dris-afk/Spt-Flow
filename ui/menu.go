package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

var menuChoices = []string{
	ChoiceTUIOnly,
	ChoiceCheckStatus,
	ChoiceCycleTheme,
	ChoiceHelp,
	ChoiceStartSpotifyDesktop,
}

type MenuModel struct {
	Selected int
	Width    int
	Height   int
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
			m.Selected = (m.Selected + 1) % len(menuChoices)
		case "k", "up":
			m.Selected = (m.Selected - 1 + len(menuChoices)) % len(menuChoices)
		case "enter":
			return m, func() tea.Msg { return MenuChoiceMsg{Choice: menuChoices[m.Selected]} }
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// menuTickCmd returns a tea.Cmd that sends MenuTimerMsg after a duration
func menuTickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return MenuTimerMsg{}
	})
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
		renderMenuOption(0, "Open TUI only", m.Selected, optionStyle, selectedStyle),
		renderMenuOption(1, "Check Spotify status", m.Selected, optionStyle, selectedStyle),
		renderMenuOption(2, "Cycle Theme", m.Selected, optionStyle, selectedStyle),
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
	menuContent := fmt.Sprintf("%s\n%s\n\n%s\n%s\n%s\n%s\n%s\n\n%s",
		titleStyle.Render("Spt-Flow"),
		subtitleStyle.Render("Spotify TUI Mirror"),
		renderMenuOption(0, "Open TUI only", m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(1, "Check Spotify status", m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(2, fmt.Sprintf("Theme: %s", themeName), m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(3, "Help / Keybindings", m.SelectedMenuOption, optionStyle, selectedStyle),
		renderMenuOption(4, "Start with Spotify Desktop", m.SelectedMenuOption, optionStyle, selectedStyle),
		footerStyle.Render("↑↓ navigate · Enter select · q quit"),
	)

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

	result := lipgloss.NewStyle().
		Width(m.Width).
		Height(m.Height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(verticalContent)

	return result
}
