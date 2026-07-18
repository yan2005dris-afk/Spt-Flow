package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tui-spotify/library"
	"tui-spotify/lyrics"
	"tui-spotify/lyrics/cache"
	"tui-spotify/mpris"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

type Model struct {
	MprisClient        *mpris.Client
	LyricsClient       *lyrics.Client
	LyricsCache        *cache.Store
	Track              mpris.Track
	Lyrics             lyrics.Lyrics
	PlaybackStatus     string
	Position           time.Duration
	LastUpdated        time.Time
	Width, Height      int
	ScrollOffset       int
	SpotifyRunning     bool
	LoadingMessage     string // e.g. "Cargando letras..." — transient loading state
	ErrorMessage       string // actual errors
	Visualizer         *Visualizer
	SignalChan         chan *dbus.Signal
	LaunchedSpotify    bool // set true after first successful LaunchSpotify
	ViewState          string // "menu" or "tui"
	SelectedMenuOption int    // 0-4 for menu navigation
	ShowingHelp        bool   // for ? overlay
	HelpTimer          bool   // if true, ? overlay auto-dismisses
	StatusMessage      string // for check-status option
	Theme              Theme  // active palette
	// Library fields
	Library            *library.Store
	FilterView         string // "playlists", "favorites", "recent", or ""
	SelectedPlaylistID string // currently selected playlist ID
	SelectedTrackIndex int    // cursor position in track list
	SidebarWidth       int    // width of left pane in two-column layout
	// URL input modal
	URLInputActive bool   // if true, capture URL input
	URLInputValue  string // accumulated URL string
}

func mustHome() string {
	h, _ := os.UserHomeDir()
	return h
}

func NewModel(viewState string) Model {
	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel // cancel will be called on quit

	visualizer := NewVisualizer(30, 10)

	// Try to initialize audio capture
	audioCapture, _ := NewAudioCapture(ctx)
	if audioCapture != nil {
		visualizer.AudioCapture = audioCapture
		visualizer.DFT = NewDFT()
		visualizer.AudioData = make([]float64, DFTBands)
		visualizer.sampleBuf = make([]float32, DFTInputSize)
	}

	cfg, _ := LoadConfig()
	theme := Themes[cfg.Theme]
	if theme == (Theme{}) {
		theme = Themes["default"]
	}

	return Model{
		LastUpdated:        time.Now(),
		Visualizer:         visualizer,
		ViewState:          viewState,
		SelectedMenuOption: 0,
		ShowingHelp:        false,
		Theme:              theme,
	}
}

func (m Model) Init() tea.Cmd {
	// Load library on startup
	m.Library, _ = library.Load()

	if m.ViewState == "menu" {
		return nil
	}
	return tea.Batch(
		m.pollSpotifyCmd(),
		m.tickCmd(),
		pollTickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		// Update visualizer dimensions to match new terminal size.
		// This persists across calls since Update() returns the modified m.
		visMaxHeight := 10.0
		if m.Width >= 80 {
			visMaxHeight = 3.0
		}
		if m.Visualizer == nil || m.Visualizer.Width != m.Width || m.Visualizer.MaxHeight != visMaxHeight {
			m.Visualizer = NewVisualizer(m.Width, visMaxHeight)
		}
		return m, nil

	case TickMsg:
		if m.SpotifyRunning && m.PlaybackStatus == "Playing" {
			m.Position += time.Since(m.LastUpdated)
			if m.Position > m.Track.Duration {
				m.Position = m.Track.Duration
			}
			m.Visualizer.Update(true)
		} else {
			m.Visualizer.Update(false)
		}
		m.LastUpdated = time.Now()
		return m, m.tickCmd()

	case PollTickMsg:
		return m, tea.Batch(m.pollSpotifyCmd(), pollTickCmd())

	case SpotifySignalMsg:
		return m, tea.Batch(m.pollSpotifyCmd(), m.watchSignalCmd())

	case SpotifyStateMsg:
		if msg.Err != nil {
			m.ErrorMessage = msg.Err.Error()
		}
		if !msg.Running {
			m.SpotifyRunning = false
			m.PlaybackStatus = "Paused"
			if m.MprisClient != nil {
				_ = m.MprisClient.Close()
				m.MprisClient = nil
			}
			m.SignalChan = nil
			return m, nil
		}

		m.SpotifyRunning = true
		m.PlaybackStatus = msg.Status
		m.Position = msg.Position
		m.LastUpdated = time.Now()

		var cmds []tea.Cmd

		// Setup watch channel if not done
		if m.MprisClient != nil && m.SignalChan == nil {
			ch, err := m.MprisClient.Watch()
			if err == nil {
				m.SignalChan = ch
				cmds = append(cmds, m.watchSignalCmd())
			}
		}

		if msg.Track.ID != m.Track.ID {
			m.Track = msg.Track
			m.ScrollOffset = 0
			m.Lyrics = lyrics.Lyrics{}
			m.LoadingMessage = "Cargando letras..."
			m.ErrorMessage = ""
			cmds = append(cmds, m.fetchLyricsCmd(msg.Track))
			// Add to recent tracks (skip if Library not yet loaded or ID empty)
			if m.Library != nil && msg.Track.ID != "" {
				m.Library.AddRecent(library.Track{
					ID:       msg.Track.ID,
					Title:    msg.Track.Title,
					Artist:   msg.Track.Artist,
					Album:    msg.Track.Album,
					Duration: msg.Track.Duration,
				})
			}
		}

		return m, tea.Batch(cmds...)

	case LyricsMsg:
		m.LoadingMessage = ""
		if msg.Err != nil {
			m.ErrorMessage = msg.Err.Error()
			m.Lyrics = lyrics.Lyrics{}
		} else {
			m.Lyrics = msg.Lyrics
			m.ErrorMessage = ""
		}
		return m, nil

	case MenuChoiceMsg:
		switch msg.Choice {
		case ChoiceStartLibrespot:
			// Create mpris client and launch librespot
			client, err := mpris.NewClient()
			if err != nil {
				m.ErrorMessage = err.Error()
				return m, menuTickCmd(3 * time.Second)
			}
			m.MprisClient = client
			cfg := filepath.Join(mustHome(), ".config", "tui-spotify", "librespot.conf")
			if err := client.LaunchLibrespot(cfg); err != nil {
				switch {
				case errors.Is(err, mpris.ErrLibrespotNotInstalled):
					m.ErrorMessage = "librespot not found. Install: cargo install librespot"
				case errors.Is(err, mpris.ErrSpotifyDesktopRunning):
					m.ErrorMessage = "Spotify desktop is running. Please close it first."
				default:
					m.ErrorMessage = err.Error()
				}
				return m, menuTickCmd(3 * time.Second)
			}
			m.LaunchedSpotify = true
			m.ViewState = "tui"
			return m, tea.Batch(m.pollSpotifyCmd(), m.tickCmd(), pollTickCmd())

		case ChoiceStartSpotifyDesktop:
			client, err := mpris.NewClient()
			if err != nil {
				m.ErrorMessage = "Cannot connect to D-Bus: " + err.Error()
				return m, menuTickCmd(3 * time.Second)
			}
			m.MprisClient = client
			if !m.MprisClient.IsRunning() {
				cmd := exec.Command("spotify")
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				if startErr := cmd.Start(); startErr != nil {
					m.ErrorMessage = "Spotify not found. Install Spotify or use Librespot."
					return m, menuTickCmd(3 * time.Second)
				}
				m.MprisClient.SetSpotifyCmd(cmd)
				m.LaunchedSpotify = true
			}
			m.ViewState = "tui"
			if m.Library == nil {
				m.Library, _ = library.Load()
			}
			if m.FilterView == "" {
				m.FilterView = "playlists"
			}
			return m, tea.Batch(m.pollSpotifyCmd(), m.tickCmd(), pollTickCmd())

		case ChoiceTUIOnly:
			m.ViewState = "tui"
			m.LaunchedSpotify = false
			if m.Library == nil {
				m.Library, _ = library.Load()
			}
			if m.FilterView == "" {
				m.FilterView = "playlists"
			}
			return m, tea.Batch(m.pollSpotifyCmd(), m.tickCmd(), pollTickCmd())

		case ChoiceCheckStatus:
			if m.MprisClient == nil {
				client, err := mpris.NewClient()
				if err == nil {
					m.MprisClient = client
				}
			}
			if m.MprisClient != nil {
				if m.MprisClient.IsRunning() {
					m.StatusMessage = "Spotify is running"
				} else {
					m.StatusMessage = "Spotify is not running"
				}
			} else {
				m.StatusMessage = "Unable to connect to D-Bus"
			}
			return m, menuTickCmd(3 * time.Second)

		case ChoiceHelp:
			m.ShowingHelp = true
			return m, menuTickCmd(5 * time.Second)

		case ChoiceCycleTheme:
			nextName := CycleTheme(m.Theme.Name())
			m.Theme = Themes[nextName]
			cfg, _ := LoadConfig()
			cfg.Theme = nextName
			_ = cfg.Save() // log errors inside Save
			return m, nil
		}
		return m, nil

	case MenuTimerMsg:
		if m.ShowingHelp {
			m.ShowingHelp = false
			return m, nil
		}
		m.ViewState = "tui"
		// Load library and set default filter view when transitioning to TUI
		if m.Library == nil {
			m.Library, _ = library.Load()
		}
		if m.FilterView == "" {
			m.FilterView = "playlists"
		}
		return m, nil

	case AddPlaylistMsg:
		if m.Library != nil {
			_, err := m.Library.AddPlaylist(msg.URL)
			if err != nil {
				m.ErrorMessage = "Invalid Spotify URL"
			} else {
				m.ErrorMessage = ""
			}
		}
		m.URLInputActive = false
		m.URLInputValue = ""
		return m, nil

	case DeletePlaylistMsg:
		if m.Library != nil {
			m.Library.RemovePlaylist(msg.ID)
		}
		return m, nil

	case ToggleFavoriteMsg:
		if m.Library != nil && msg.TrackID != "" {
			if m.Library.IsFavorite(msg.TrackID) {
				m.Library.RemoveFavorite(msg.TrackID)
			} else {
				// Find track in current view and add to favorites
				var track library.Track
				switch m.FilterView {
				case "playlists":
					playlists := m.Library.GetPlaylists()
					for _, p := range playlists {
						if p.ID == m.SelectedPlaylistID {
							// Build track from playlist context (TrackIDs only, no metadata)
							// For favorites view, we'd need full track info
						}
					}
				case "favorites":
					favorites := m.Library.GetFavorites()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(favorites) {
						track = favorites[m.SelectedTrackIndex]
					}
				case "recent":
					recent := m.Library.GetRecent()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(recent) {
						track = recent[m.SelectedTrackIndex]
					}
				}
				if track.ID != "" {
					m.Library.AddFavorite(track)
				}
			}
		}
		return m, nil

	case PlayTrackMsg:
		if msg.TrackID != "" {
			if err := library.OpenTrack(msg.TrackID); err != nil {
				m.ErrorMessage = "Cannot open track: install xdg-open"
			} else {
				m.ErrorMessage = ""
			}
		}
		return m, nil

	case LibraryChangedMsg:
		// Library was modified externally; state already updated via pointer
		return m, nil

	case tea.KeyMsg:
		// Handle help overlay dismissal
		if m.ShowingHelp {
			m.ShowingHelp = false
			return m, nil
		}

		// Menu navigation
		if m.ViewState == "menu" {
			switch msg.String() {
			case "j", "down":
				m.SelectedMenuOption = (m.SelectedMenuOption + 1) % len(menuChoices)
			case "k", "up":
				m.SelectedMenuOption = (m.SelectedMenuOption - 1 + len(menuChoices)) % len(menuChoices)
			case "enter":
				return m, func() tea.Msg { return MenuChoiceMsg{Choice: menuChoices[m.SelectedMenuOption]} }
			case "q", "ctrl+c":
				if m.MprisClient != nil {
					_ = m.MprisClient.Close()
				}
				if m.Visualizer != nil && m.Visualizer.AudioCapture != nil {
					_ = m.Visualizer.AudioCapture.Close()
				}
				return m, tea.Quit
			}
			return m, nil
		}

		// ? key shows help overlay in TUI state
		if msg.String() == "?" {
			m.ShowingHelp = true
			return m, menuTickCmd(5 * time.Second)
		}

		// ESC key returns to menu from TUI, or closes sidebar/modal
		if msg.String() == "esc" {
			if m.URLInputActive {
				m.URLInputActive = false
				m.URLInputValue = ""
				return m, nil
			}
			if m.FilterView != "" {
				m.FilterView = ""
				return m, nil
			}
			m.ViewState = "menu"
			m.SelectedMenuOption = 0
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			if m.MprisClient != nil {
				_ = m.MprisClient.Close()
			}
			if m.Visualizer != nil && m.Visualizer.AudioCapture != nil {
				_ = m.Visualizer.AudioCapture.Close()
			}
			return m, tea.Quit
		}

		if !m.SpotifyRunning {
			return m, nil
		}

		switch msg.String() {
		case " ":
			if m.MprisClient != nil {
				_ = m.MprisClient.PlayPause()
			}
		case "n", "l":
			if m.MprisClient != nil {
				_ = m.MprisClient.Next()
			}
		case "p", "h":
			if m.MprisClient != nil {
				_ = m.MprisClient.Previous()
			}
		case "+", "=":
			if m.MprisClient != nil {
				if vol, err := m.MprisClient.GetVolume(); err == nil {
					newVol := vol + 0.1
					if newVol > 1.0 {
						newVol = 1.0
					}
					_ = m.MprisClient.SetVolume(newVol)
				}
			}
		case "-":
			if m.MprisClient != nil {
				if vol, err := m.MprisClient.GetVolume(); err == nil {
					newVol := vol - 0.1
					if newVol < 0.0 {
						newVol = 0.0
					}
					_ = m.MprisClient.SetVolume(newVol)
				}
			}
		case "k", "up":
			if m.FilterView != "" {
				m.cursorUp()
			} else if !m.Lyrics.Synced {
				m.scrollUp()
			}
		case "j", "down":
			if m.FilterView != "" {
				m.cursorDown()
			} else if !m.Lyrics.Synced {
				m.scrollDown()
			}
		case "1":
			m.FilterView = "playlists"
			m.SelectedTrackIndex = 0
			m.SidebarWidth = m.Width * 40 / 100
		case "2":
			m.FilterView = "favorites"
			m.SelectedTrackIndex = 0
			m.SidebarWidth = m.Width * 40 / 100
		case "3":
			m.FilterView = "recent"
			m.SelectedTrackIndex = 0
			m.SidebarWidth = m.Width * 40 / 100
		case "a":
			m.URLInputActive = true
			m.URLInputValue = ""
		case "d":
			if m.FilterView == "playlists" && m.SelectedPlaylistID != "" {
				return m, func() tea.Msg { return DeletePlaylistMsg{ID: m.SelectedPlaylistID} }
			}
			if m.FilterView == "favorites" || m.FilterView == "recent" {
				// Delete by track index
				var trackID string
				if m.FilterView == "favorites" && m.Library != nil {
					favs := m.Library.GetFavorites()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(favs) {
						trackID = favs[m.SelectedTrackIndex].ID
						m.Library.RemoveFavorite(trackID)
					}
				} else if m.FilterView == "recent" && m.Library != nil {
					rec := m.Library.GetRecent()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(rec) {
						trackID = rec[m.SelectedTrackIndex].ID
					}
				}
			}
		case "f":
			if m.FilterView != "" && m.Library != nil {
				var trackID string
				if m.FilterView == "favorites" && m.Library != nil {
					favs := m.Library.GetFavorites()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(favs) {
						trackID = favs[m.SelectedTrackIndex].ID
					}
				} else if m.FilterView == "recent" && m.Library != nil {
					rec := m.Library.GetRecent()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(rec) {
						trackID = rec[m.SelectedTrackIndex].ID
					}
				} else if m.FilterView == "playlists" && m.SelectedPlaylistID != "" {
					// For playlists, get track ID from selected index
					pl, ok := m.Library.GetPlaylist(m.SelectedPlaylistID)
					if ok && m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(pl.TrackIDs) {
						trackID = pl.TrackIDs[m.SelectedTrackIndex]
					}
				}
				if trackID != "" {
					return m, func() tea.Msg { return ToggleFavoriteMsg{TrackID: trackID} }
				}
			}
		case "enter":
			if m.URLInputActive {
				// Submit URL
				if m.URLInputValue != "" {
					return m, func() tea.Msg { return AddPlaylistMsg{URL: m.URLInputValue} }
				}
				m.URLInputActive = false
				m.URLInputValue = ""
			} else if m.FilterView != "" {
				// Play selected track
				var trackID string
				if m.FilterView == "playlists" && m.SelectedPlaylistID != "" {
					pl, ok := m.Library.GetPlaylist(m.SelectedPlaylistID)
					if ok && m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(pl.TrackIDs) {
						trackID = pl.TrackIDs[m.SelectedTrackIndex]
					}
				} else if m.FilterView == "favorites" && m.Library != nil {
					favs := m.Library.GetFavorites()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(favs) {
						trackID = favs[m.SelectedTrackIndex].ID
					}
				} else if m.FilterView == "recent" && m.Library != nil {
					rec := m.Library.GetRecent()
					if m.SelectedTrackIndex >= 0 && m.SelectedTrackIndex < len(rec) {
						trackID = rec[m.SelectedTrackIndex].ID
					}
				}
				if trackID != "" {
					return m, func() tea.Msg { return PlayTrackMsg{TrackID: trackID} }
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.ViewState == "menu" {
		return renderMenuView(m)
	}
	return renderTUIScreen(m)
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func pollTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return PollTickMsg{}
	})
}

func (m *Model) watchSignalCmd() tea.Cmd {
	return func() tea.Msg {
		if m.SignalChan == nil {
			return nil
		}
		_, ok := <-m.SignalChan
		if !ok {
			return nil
		}
		return SpotifySignalMsg{}
	}
}

func (m *Model) pollSpotifyCmd() tea.Cmd {
	return func() tea.Msg {
		if m.MprisClient == nil {
			client, err := mpris.NewClient()
			if err != nil {
				return SpotifyStateMsg{Running: false, Err: err}
			}
			m.MprisClient = client
		}

		if !m.MprisClient.IsRunning() {
			return SpotifyStateMsg{Running: false}
		}

		track, err := m.MprisClient.GetTrack()
		if err != nil {
			return SpotifyStateMsg{Running: true, Err: err}
		}

		status, err := m.MprisClient.GetPlaybackStatus()
		if err != nil {
			return SpotifyStateMsg{Running: true, Err: err}
		}

		pos, err := m.MprisClient.GetPosition()
		if err != nil {
			pos = 0
		}

		vol, err := m.MprisClient.GetVolume()
		if err != nil {
			vol = 1.0
		}

		return SpotifyStateMsg{
			Running:  true,
			Track:    track,
			Status:   status,
			Position: pos,
			Volume:   vol,
		}
	}
}

func (m *Model) fetchLyricsCmd(track mpris.Track) tea.Cmd {
	return func() tea.Msg {
		if m.LyricsClient == nil {
			m.LyricsClient = lyrics.NewClient("")
		}
		if m.LyricsCache == nil {
			cacheDir, _ := os.UserCacheDir()
			cachePath := filepath.Join(cacheDir, "spt-flow", "lyrics.json")
			store, err := cache.New(cachePath, m.LyricsClient)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cache: init failed: %v\n", err)
			} else {
				m.LyricsCache = store
			}
		}
		var lyr lyrics.Lyrics
		var err error
		if m.LyricsCache != nil {
			lyr, err = m.LyricsCache.Get(track.Title, track.Artist, track.Album, track.Duration)
		} else {
			lyr, err = m.LyricsClient.FetchLyrics(track.Title, track.Artist, track.Album, track.Duration)
		}
		if err != nil {
			return LyricsMsg{Err: err}
		}
		return LyricsMsg{Lyrics: lyr}
	}
}

// renderTUIScreen builds the main TUI view: header, body (lyrics + optional sidebar),
// and footer. Returns the empty-state placeholder when Spotify is not running.
func renderTUIScreen(m Model) string {
	if m.ShowingHelp {
		return renderKeybindingsOverlay(m)
	}

	if !m.SpotifyRunning {
		return lipgloss.NewStyle().
			Width(m.Width).
			Height(m.Height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Waiting for Spotify...")
	}

	// URL input modal overlay
	if m.URLInputActive {
		return m.renderURLInputModal()
	}

	// Build header as a simple visible string (NO lipgloss for header)
	trackInfo := fmt.Sprintf("♪  %s  —  %s", m.Track.Title, m.Track.Artist)
	if m.Track.Album != "" {
		trackInfo += fmt.Sprintf("  (%s)", m.Track.Album)
	}
	// Make sure trackInfo is at least visible
	if trackInfo == "" || trackInfo == "♪  — " {
		trackInfo = "♪  [No track info]"
	}
	// Header line 1: track info
	hdrLine1 := fmt.Sprintf("=== HEADER: %s ===", trackInfo)
	// Header line 2: separator
	hdrLine2 := strings.Repeat("=", m.Width)
	header := hdrLine1 + "\n" + hdrLine2

	// Footer (1 line)
	footer := renderFooter(m)

	// Main area
	const headerHeight = 2
	const footerHeight = 1
	mainHeight := m.Height - headerHeight - footerHeight
	if mainHeight < 3 {
		mainHeight = 3
	}

	var mainAreaLines []string
	if m.Width < 80 || m.SidebarWidth == 0 {
		lyrics := renderLyrics(m, m.Width, mainHeight)
		mainAreaLines = strings.Split(lyrics, "\n")
	} else {
		sidebarContent := m.renderSidebar(m.SidebarWidth, mainHeight)
		lyricsWidth := m.Width - m.SidebarWidth - 1
		if lyricsWidth < 10 {
			lyricsWidth = 10
		}
		lyricsContent := renderLyrics(m, lyricsWidth, mainHeight)

		sidebarLines := strings.Split(sidebarContent, "\n")
		lyricsLines := strings.Split(lyricsContent, "\n")

		for len(sidebarLines) < mainHeight {
			sidebarLines = append(sidebarLines, strings.Repeat(" ", m.SidebarWidth))
		}
		for len(lyricsLines) < mainHeight {
			lyricsLines = append(lyricsLines, strings.Repeat(" ", lyricsWidth))
		}
		if len(sidebarLines) > mainHeight {
			sidebarLines = sidebarLines[:mainHeight]
		}
		if len(lyricsLines) > mainHeight {
			lyricsLines = lyricsLines[:mainHeight]
		}

		for i := 0; i < mainHeight; i++ {
			sLine := sidebarLines[i]
			if len(sLine) < m.SidebarWidth {
				sLine = sLine + strings.Repeat(" ", m.SidebarWidth-len(sLine))
			} else if len(sLine) > m.SidebarWidth {
				sLine = sLine[:m.SidebarWidth]
			}
			lLine := lyricsLines[i]
			if len(lLine) < lyricsWidth {
				lLine = lLine + strings.Repeat(" ", lyricsWidth-len(lLine))
			} else if len(lLine) > lyricsWidth {
				lLine = lLine[:lyricsWidth]
			}
			mainAreaLines = append(mainAreaLines, sLine+"│"+lLine)
		}
	}

	for len(mainAreaLines) < mainHeight {
		mainAreaLines = append(mainAreaLines, strings.Repeat(" ", m.Width))
	}
	if len(mainAreaLines) > mainHeight {
		mainAreaLines = mainAreaLines[:mainHeight]
	}

	mainArea := strings.Join(mainAreaLines, "\n")

	return header + "\n" + mainArea + "\n" + footer
}

// renderVisualizer renders the visualizer block for the current model state.
func renderVisualizer(m Model, width, height int) string {
	if m.Visualizer == nil {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	if m.Visualizer.Width != width {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	lines := m.Visualizer.Render(height)

	lipglossColor := m.Theme.Visualizer
	if m.Visualizer.AudioCapture != nil {
		lipglossColor = m.Theme.VisualizerBackground
	}
	ansiColor := lipglossToAnsi(lipglossColor)
	ansiReset := "\x1b[0m"
	var styledLines []string
	for _, l := range lines {
		if l != "" {
			styledLines = append(styledLines, ansiColor+l+ansiReset)
		} else {
			styledLines = append(styledLines, l)
		}
	}
	return strings.Join(styledLines, "\n")
}