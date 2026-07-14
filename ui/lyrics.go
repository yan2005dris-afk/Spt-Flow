package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) getActiveLyricIndex() int {
	if !m.Lyrics.Synced || len(m.Lyrics.Lines) == 0 {
		return -1
	}
	active := -1
	for i, line := range m.Lyrics.Lines {
		if line.Timestamp <= m.Position {
			active = i
		} else {
			break
		}
	}
	return active
}

func (m *Model) scrollUp() {
	if m.ScrollOffset > 0 {
		m.ScrollOffset--
	}
}

func (m *Model) scrollDown() {
	maxScroll := len(m.Lyrics.Lines) - 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.ScrollOffset < maxScroll {
		m.ScrollOffset++
	}
}

func renderLyrics(m Model, width, height int) string {
	if len(m.Lyrics.Lines) == 0 {
		var msg string
		var color lipgloss.Color
		switch {
		case m.LoadingMessage != "":
			msg = m.LoadingMessage
			color = m.Theme.Waiting
		case m.ErrorMessage != "":
			msg = "Error: " + m.ErrorMessage
			color = m.Theme.Error
		default:
			msg = "No lyrics available"
			color = m.Theme.MenuDim
		}

		// Render the message centered in a single line, then pad to `height`
		// lines so the layout stays stable across content changes.
		rendered := lipgloss.NewStyle().
			Width(width).
			Align(lipgloss.Center).
			Foreground(color).
			Render(msg)

		// Center the message vertically within the lyrics area.
		padCount := (height - 1) / 2
		padding := strings.Repeat("\n", padCount)
		return padding + rendered + strings.Repeat("\n", height-1-padCount)
	}

	var renderedLines []string

	// Fade effect: add top padding (2 lines fade effect)
	topFade := 2

	if m.Lyrics.Synced {
		active := m.getActiveLyricIndex()
		mid := height / 2
		start := active - mid
		if start < 0 {
			start = 0
		}

		for i := 0; i < height; i++ {
			// Apply fade effect at top
			if i < topFade {
				renderedLines = append(renderedLines, "")
				continue
			}

			lineIdx := start + i - topFade
			if lineIdx < len(m.Lyrics.Lines) {
				line := m.Lyrics.Lines[lineIdx]
				content := line.Content
				if len(content) > width {
					content = content[:width]
				}

				style := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
				if lineIdx == active {
					style = style.Foreground(m.Theme.LyricActive).Bold(true)
				} else {
					style = style.Foreground(m.Theme.LyricInactive)
				}
				renderedLines = append(renderedLines, style.Render(content))
				// Add blank line every 4 lines for better readability (Design 3 style)
				if (i+1)%4 == 0 && i < height-1 {
					renderedLines = append(renderedLines, "")
				}
			} else {
				renderedLines = append(renderedLines, "")
			}
		}
	} else {
		for i := 0; i < height; i++ {
			// Apply fade effect at top
			if i < topFade {
				renderedLines = append(renderedLines, "")
				continue
			}

			lineIdx := m.ScrollOffset + i - topFade
			if lineIdx < len(m.Lyrics.Lines) {
				line := m.Lyrics.Lines[lineIdx]
				content := line.Content
				if len(content) > width {
					content = content[:width]
				}
				style := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Foreground(m.Theme.LyricPlain)
				renderedLines = append(renderedLines, style.Render(content))
				// Add blank line every 4 lines for better readability (Design 3 style)
				if (i+1)%4 == 0 && i < height-1 {
					renderedLines = append(renderedLines, "")
				}
			} else {
				renderedLines = append(renderedLines, "")
			}
		}
	}
	return strings.Join(renderedLines, "\n")
}

// renderTrackList renders the right pane with track details.
func (m Model) renderTrackList(width, height int) string {
	if width < 5 {
		width = 5
	}

	headerText := fmt.Sprintf("<%s>", m.FilterView)
	count := 0
	if m.Library != nil {
		switch m.FilterView {
		case "playlists":
			if m.SelectedPlaylistID != "" {
				if pl, ok := m.Library.GetPlaylist(m.SelectedPlaylistID); ok {
					count = len(pl.TrackIDs)
				}
			}
		case "favorites":
			count = len(m.Library.GetFavorites())
		case "recent":
			count = len(m.Library.GetRecent())
		}
	}
	headerText = fmt.Sprintf("%s (%d)", headerText, count)

	headerStyle := lipgloss.NewStyle().
		Width(width).
		Bold(true).
		Foreground(m.Theme.Header)

	listHeight := height - 1
	var tracks []string
	tracks = append(tracks, headerStyle.Render(headerText))

	switch m.FilterView {
	case "playlists":
		if m.Library != nil && m.SelectedPlaylistID != "" {
			if pl, ok := m.Library.GetPlaylist(m.SelectedPlaylistID); ok {
				for i, tid := range pl.TrackIDs {
					prefix := "  "
					if i == m.SelectedTrackIndex {
						prefix = "> "
					}
					tracks = append(tracks, prefix+tid)
				}
			}
		}
	case "favorites":
		if m.Library != nil {
			favorites := m.Library.GetFavorites()
			for i, tr := range favorites {
				prefix := "  "
				marker := " "
				if tr.ID == m.Track.ID {
					marker = "▶"
				}
				if i == m.SelectedTrackIndex {
					prefix = "> "
				}
				tracks = append(tracks, fmt.Sprintf("%s%s %s - %s", prefix, marker, tr.Title, tr.Artist))
			}
		}
	case "recent":
		if m.Library != nil {
			recent := m.Library.GetRecent()
			for i, tr := range recent {
				prefix := "  "
				marker := " "
				if tr.ID == m.Track.ID {
					marker = "▶"
				}
				if i == m.SelectedTrackIndex {
					prefix = "> "
				}
				tracks = append(tracks, fmt.Sprintf("%s%s %s - %s", prefix, marker, tr.Title, tr.Artist))
			}
		}
	}

	if len(tracks) == 1 { // only header
		emptyStyle := lipgloss.NewStyle().
			Width(width).
			Height(listHeight).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(m.Theme.MenuDim)
		tracks = append(tracks, emptyStyle.Render("No tracks"))
	}

	// Pad to fill height
	for len(tracks) < height {
		tracks = append(tracks, "")
	}
	if len(tracks) > height {
		tracks = tracks[:height]
	}

	contentStyle := lipgloss.NewStyle().
		Width(width).
		Height(height)

	return contentStyle.Render(strings.Join(tracks, "\n"))
}