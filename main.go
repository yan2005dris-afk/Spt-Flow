package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"tui-spotify/mpris"
	"tui-spotify/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	mprisClient, err := mpris.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mpris: %v\n", err)
		os.Exit(1)
	}

	// Determine initial view state based on --no-menu flag
	viewState := "menu"
	for _, arg := range os.Args[1:] {
		if arg == "--no-menu" {
			viewState = "tui"
			break
		}
	}

	p := tea.NewProgram(ui.NewModel(viewState), tea.WithAltScreen())

	// Signal trap: catch Ctrl+C and SIGTERM before the bubble tea loop
	// swallows them. KillSpotify is safe to call even when spotifyCmd is nil.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = mprisClient.KillSpotify()
		p.Quit()
		os.Exit(0)
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	// Final safety net: even when SIGINT/SIGTERM did NOT fire (e.g. normal
	// exit via 'q' key), kill Spotify if we launched it. Client.Close is
	// idempotent thanks to the nil-guard on spotifyCmd.
	_ = mprisClient.Close()
}
