package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"tui-spotify/mpris"
	"tui-spotify/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Check for --setup flag first
	for _, arg := range os.Args[1:] {
		if arg == "--setup" {
			if err := runSetup(); err != nil {
				fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
			return
		}
	}

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

func mustHome() string {
	h, _ := os.UserHomeDir()
	return h
}

func printInstallInstructions() {
	fmt.Println("librespot is not installed. Install it with:")
	fmt.Println()
	fmt.Println("  # With cargo (recommended)")
	fmt.Println("  cargo install librespot")
	fmt.Println()
	fmt.Println("  # On Arch Linux")
	fmt.Println("  sudo pacman -S librespot")
	fmt.Println()
	fmt.Println("  # On other distros, check your package manager or build from source:")
	fmt.Println("  # https://github.com/librespot-org/librespot")
}

func runSetup() error {
	path, err := exec.LookPath("librespot")
	if err != nil {
		printInstallInstructions()
		return mpris.ErrLibrespotNotInstalled
	}

	// Ensure cache directory exists
	cacheDir := filepath.Join(mustHome(), ".cache", "librespot")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create cache directory: %v\n", err)
	}

	cfg := filepath.Join(mustHome(), ".config", "tui-spotify", "librespot.conf")

	// Ensure config directory exists
	configDir := filepath.Join(mustHome(), ".config", "tui-spotify")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create config directory: %v\n", err)
	}

	cmd := exec.Command(path,
		"--name", "Spt-Flow",
		"--enable-oauth",
		"--cache", cacheDir,
		"--config", cfg,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start librespot: %w", err)
	}

	// Wait for authentication with timeout
	done := make(chan error, 1)
	go func() {
		// Wait for the process to finish
		done <- cmd.Wait()
	}()

	timeout := time.After(60 * time.Second)

	// Poll for IsRunning to become true
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			_ = cmd.Process.Kill()
			return fmt.Errorf("authentication timed out after 60 seconds")
		case err := <-done:
			return fmt.Errorf("librespot exited before authentication: %w", err)
		case <-ticker.C:
			// Check if spotify is running via MPRIS
			client, err := mpris.NewClient()
			if err == nil {
				if client.IsRunning() {
					fmt.Println("\nAuthentication successful!")
					return nil
				}
				_ = client.Close()
			}
		}
	}
}
