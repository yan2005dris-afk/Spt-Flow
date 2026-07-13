package main

import (
	"fmt"
	"os"

	"tui-spotify/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := ui.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
