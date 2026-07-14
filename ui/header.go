package ui

import (
	"fmt"
	"strings"
)

func renderHeader(m Model) string {
	// Simple 2-line header that's always visible
	trackInfo := fmt.Sprintf("♪  %s  —  %s", m.Track.Title, m.Track.Artist)
	if m.Track.Album != "" {
		trackInfo += fmt.Sprintf("  (%s)", m.Track.Album)
	}

	// Pad trackInfo to full width with spaces for the background
	if len(trackInfo) < m.Width {
		trackInfo = trackInfo + strings.Repeat(" ", m.Width-len(trackInfo))
	} else if len(trackInfo) > m.Width {
		trackInfo = trackInfo[:m.Width]
	}

	// Line 1: track info with ANSI background for visibility
	headerLine := fmt.Sprintf("\x1b[1;37;100m%s\x1b[0m", trackInfo)

	// Line 2: separator with ANSI dim color
	sep := strings.Repeat("─", m.Width)
	separator := fmt.Sprintf("\x1b[38;5;240m%s\x1b[0m", sep)

	return headerLine + "\n" + separator
}