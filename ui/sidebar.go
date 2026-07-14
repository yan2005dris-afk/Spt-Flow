package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderSidebar renders the left pane with filter tabs and list.
// Design 3 style: inline tabs ▶ Playlists ♡ Favorites ◷ Recent
func (m Model) renderSidebar(width, height int) string {
	if width < 20 {
		width = 20
	}

	tabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")) // visible gray

	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")). // bright white when active
		Background(lipgloss.Color("236")). // dark bg for active tab
		Bold(true)

	// Truncate tabs if width is too narrow
	var tabsLine string
	if width < 25 {
		// Compact tabs for narrow sidebar
		if m.FilterView == "playlists" {
			tabsLine = activeStyle.Render("▶PL") + tabStyle.Render(" ♡FA ◷RE")
		} else if m.FilterView == "favorites" {
			tabsLine = tabStyle.Render("▶PL ") + activeStyle.Render("♡FA") + tabStyle.Render(" ◷RE")
		} else {
			tabsLine = tabStyle.Render("▶PL ♡FA ") + activeStyle.Render("◷RE")
		}
	} else {
		// Full tabs
		if m.FilterView == "playlists" {
			tabsLine = activeStyle.Render("▶ Playlists") + tabStyle.Render("  ♡ Favorites  ◷ Recent")
		} else if m.FilterView == "favorites" {
			tabsLine = tabStyle.Render("▶ Playlists  ") + activeStyle.Render("♡ Favorites") + tabStyle.Render("  ◷ Recent")
		} else {
			tabsLine = tabStyle.Render("▶ Playlists  ♡ Favorites  ") + activeStyle.Render("◷ Recent")
		}
	}

	// Build list content
	var listContent []string
	listHeight := height - 1 // minus 1 for tabs row
	if listHeight < 1 {
		listHeight = 1
	}

	switch m.FilterView {
	case "playlists":
		if m.Library != nil {
			playlists := m.Library.GetPlaylists()
			for i, pl := range playlists {
				prefix := "  "
				if i == m.SelectedTrackIndex {
					prefix = "> "
				}
				name := pl.Name
				if name == "" {
					name = "Playlist"
				}
				// Truncate if too wide
				if len(name) > width-2 {
					name = name[:width-5] + "..."
				}
				listContent = append(listContent, prefix+name)
			}
			if len(playlists) == 0 {
				listContent = append(listContent, "(empty)")
				listContent = append(listContent, "Press 'A' to add")
			}
		} else {
			listContent = append(listContent, "(loading...)")
		}
	case "favorites":
		if m.Library != nil {
			favorites := m.Library.GetFavorites()
			for i, tr := range favorites {
				prefix := "  "
				if i == m.SelectedTrackIndex {
					prefix = "> "
				}
				text := tr.Title + " - " + tr.Artist
				// Truncate if too wide
				if len(text) > width-2 {
					text = text[:width-5] + "..."
				}
				listContent = append(listContent, prefix+text)
			}
			if len(favorites) == 0 {
				listContent = append(listContent, "(empty)")
				listContent = append(listContent, "Press 'F' to fav")
			}
		} else {
			listContent = append(listContent, "(loading...)")
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
				text := marker + " " + tr.Title + " - " + tr.Artist
				// Truncate if too wide
				if len(text) > width-2 {
					text = text[:width-5] + "..."
				}
				listContent = append(listContent, prefix+text)
			}
			if len(recent) == 0 {
				listContent = append(listContent, "(empty)")
				listContent = append(listContent, "Play tracks")
				listContent = append(listContent, "to populate")
			}
		} else {
			listContent = append(listContent, "(loading...)")
		}
	}

	// Pad list to fill height
	for len(listContent) < listHeight {
		listContent = append(listContent, "")
	}
	if len(listContent) > listHeight {
		listContent = listContent[:listHeight]
	}

	listStyle := lipgloss.NewStyle().
		Width(width).
		Height(listHeight)

	listRendered := listStyle.Render(strings.Join(listContent, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(width).Render(tabsLine),
		listRendered,
	)
}

func (m *Model) cursorUp() {
	if m.SelectedTrackIndex > 0 {
		m.SelectedTrackIndex--
	}
}

func (m *Model) cursorDown() {
	if m.Library == nil {
		return
	}
	var max int
	switch m.FilterView {
	case "playlists":
		if m.SelectedPlaylistID != "" {
			pl, ok := m.Library.GetPlaylist(m.SelectedPlaylistID)
			if ok {
				max = len(pl.TrackIDs)
			}
		} else {
			max = len(m.Library.GetPlaylists())
		}
	case "favorites":
		max = len(m.Library.GetFavorites())
	case "recent":
		max = len(m.Library.GetRecent())
	}
	if m.SelectedTrackIndex < max-1 {
		m.SelectedTrackIndex++
	}
}