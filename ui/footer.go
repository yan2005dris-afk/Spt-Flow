package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func renderFooter(m Model) string {
	width := m.Width - 4
	if width < 10 {
		width = 10
	}

	pbWidth := width - 15
	if pbWidth < 5 {
		pbWidth = 5
	}

	pb := renderProgressBar(pbWidth, m.Position, m.Track.Duration)

	volPercent := 0
	if m.MprisClient != nil {
		if v, err := m.MprisClient.GetVolume(); err == nil {
			volPercent = int(v * 100)
		}
	}

	posStr := formatDuration(m.Position)
	durStr := formatDuration(m.Track.Duration)

	info := fmt.Sprintf("%s %s %s [%d%% Volume]", posStr, pb, durStr, volPercent)

	footerStyle := lipgloss.NewStyle().
		Width(m.Width)
	if !m.SpotifyRunning {
		footerStyle = footerStyle.Foreground(m.Theme.Waiting)
	} else {
		footerStyle = footerStyle.Foreground(m.Theme.Footer)
	}
	return footerStyle.Render(info)
}