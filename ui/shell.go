package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

type (
	TickMsg            struct{}
	PollTickMsg        struct{}
	SpotifySignalMsg   struct{}
	MenuChoiceMsg      struct{ Choice string }
	MenuTimerMsg       struct{}
	LibraryChangedMsg  struct{}
	AddPlaylistMsg    struct{ URL string }
	DeletePlaylistMsg  struct{ ID string }
	ToggleFavoriteMsg  struct{ TrackID string }
	PlayTrackMsg       struct{ TrackID string }
)

type SpotifyStateMsg struct {
	Running  bool
	Track    mpris.Track
	Status   string
	Position time.Duration
	Volume   float64
	Err      error
}

type LyricsMsg struct {
	Lyrics lyrics.Lyrics
	Err    error
}

type Model struct {
	MprisClient         *mpris.Client
	LyricsClient        *lyrics.Client
	LyricsCache         *cache.Store
	Track               mpris.Track
	Lyrics              lyrics.Lyrics
	PlaybackStatus      string
	Position            time.Duration
	LastUpdated         time.Time
	Width, Height       int
	ScrollOffset        int
	SpotifyRunning      bool
	LoadingMessage      string // e.g. "Cargando letras..." — transient loading state
	ErrorMessage        string // actual errors
	Visualizer          *Visualizer
	SignalChan          chan *dbus.Signal
	LaunchedSpotify     bool   // set true after first successful LaunchSpotify
	ViewState           string // "menu" or "tui"
	SelectedMenuOption  int    // 0-5 for menu navigation
	ShowingHelp         bool   // for ? overlay
	HelpTimer           bool   // if true, ? overlay auto-dismisses
	StatusMessage       string // for check-status option
	Theme               Theme  // active palette
	// Library fields
	Library             *library.Store
	FilterView          string   // "playlists", "favorites", "recent", or ""
	SelectedPlaylistID  string   // currently selected playlist ID
	SelectedTrackIndex  int      // cursor position in track list
	SidebarWidth        int      // width of left pane in two-column layout
	// URL input modal
	URLInputActive      bool     // if true, capture URL input
	URLInputValue       string   // accumulated URL string
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
			return m, tea.Batch(m.pollSpotifyCmd(), m.tickCmd(), pollTickCmd())

		case ChoiceTUIOnly:
			m.ViewState = "tui"
			m.LaunchedSpotify = false
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

	header := renderHeader(m)
	footer := renderFooter(m)

	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	mainHeight := m.Height - headerHeight - footerHeight
	if mainHeight < 0 {
		mainHeight = 0
	}

	// Two-column layout when FilterView is active and width >= 80
	var mainArea string
	if m.Width >= 80 && m.FilterView != "" {
		m.SidebarWidth = m.Width * 40 / 100
		sidebarContent := m.renderSidebar(m.SidebarWidth, mainHeight)
		trackListWidth := m.Width - m.SidebarWidth - 1
		trackListContent := m.renderTrackList(trackListWidth, mainHeight)
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, sidebarContent, trackListContent)
	} else {
		// Lyrics fill the main area minus a fixed visualizer strip below.
		// The visualizer area is ALWAYS reserved (when wide enough) so lyrics
		// don't shift when the visualizer appears/disappears (e.g. on track
		// change when playback status flips).
		visRow := 0
		if m.Width >= 80 {
			visRow = 3
		}

		lyricsHeight := mainHeight - visRow
		if lyricsHeight < 3 {
			lyricsHeight = mainHeight
			visRow = 0
		}

		lyricsContent := renderLyrics(m, m.Width, lyricsHeight)
		if visRow > 0 {
			var visContent string
			if m.PlaybackStatus == "Playing" {
				visContent = renderVisualizer(m, m.Width, visRow)
			} else {
				// Reserve the visualizer area with blank lines so the layout
				// stays put when playback toggles.
				visContent = strings.Repeat("\n", visRow-1)
			}
			mainArea = lipgloss.JoinVertical(lipgloss.Left, lyricsContent, visContent)
		} else {
			mainArea = lyricsContent
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)
}

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

func renderHeader(m Model) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.Theme.Header).
		Background(lipgloss.Color("0")).                           // subtle dark bg behind header
		Border(lipgloss.NormalBorder(), true, false, true, false). // top + bottom border
		BorderForeground(m.Theme.MenuBorder).
		Width(m.Width).
		Padding(0, 1)

	trackInfo := fmt.Sprintf("%s - %s", m.Track.Title, m.Track.Artist)
	if m.Track.Album != "" {
		trackInfo += fmt.Sprintf(" (%s)", m.Track.Album)
	}

	return style.Render(trackInfo)
}

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
		Padding(0, 1)
	if !m.SpotifyRunning {
		footerStyle = footerStyle.Foreground(m.Theme.Waiting)
	} else {
		footerStyle = footerStyle.Foreground(m.Theme.Footer)
	}
	return footerStyle.Render(info)
}

func renderProgressBar(width int, pos, dur time.Duration) string {
	if dur <= 0 {
		return strings.Repeat("░", width)
	}
	ratio := float64(pos) / float64(dur)
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(width))
	empty := width - filled
	if empty < 0 {
		empty = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func formatDuration(d time.Duration) string {
	s := int(d.Seconds())
	m := s / 60
	s = s % 60
	return fmt.Sprintf("%02d:%02d", m, s)
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
	if m.Lyrics.Synced {
		active := m.getActiveLyricIndex()
		mid := height / 2
		start := active - mid
		if start < 0 {
			start = 0
		}

		for i := 0; i < height; i++ {
			lineIdx := start + i
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
			} else {
				renderedLines = append(renderedLines, "")
			}
		}
	} else {
		for i := 0; i < height; i++ {
			lineIdx := m.ScrollOffset + i
			if lineIdx < len(m.Lyrics.Lines) {
				line := m.Lyrics.Lines[lineIdx]
				content := line.Content
				if len(content) > width {
					content = content[:width]
				}
				style := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Foreground(m.Theme.LyricPlain)
				renderedLines = append(renderedLines, style.Render(content))
			} else {
				renderedLines = append(renderedLines, "")
			}
		}
	}
	return strings.Join(renderedLines, "\n")
}

func renderVisualizer(m Model, width, height int) string {
	if m.Visualizer == nil {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	if m.Visualizer.Width != width {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	lines := m.Visualizer.Render(height)

	// Pick the active theme color. Audio mode → VisualizerBackground;
	// procedural mode → Visualizer.
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

// renderURLInputModal renders the URL paste modal overlay.
func (m Model) renderURLInputModal() string {
	width := 60
	height := 5
	x := (m.Width - width) / 2
	if x < 0 {
		x = 0
	}
	y := (m.Height - height) / 2
	if y < 0 {
		y = 0
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.Theme.MenuBorder).
		Width(width).
		Height(height)

	content := fmt.Sprintf("Paste Spotify URL and press Enter:\n\n  %s", m.URLInputValue)
	if m.URLInputValue == "" {
		content = "Paste Spotify URL and press Enter:\n\n  (waiting for input...)"
	}

	modal := lipgloss.Place(m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		borderStyle.Render(content),
	)

	// Also show the URL value at the bottom of the screen for feedback
	return lipgloss.JoinVertical(lipgloss.Left,
		modal,
		lipgloss.NewStyle().
			Foreground(m.Theme.Waiting).
			Render("URL: "+m.URLInputValue),
	)
}

// renderSidebar renders the left pane with filter tabs and list.
func (m Model) renderSidebar(width, height int) string {
	if width < 5 {
		width = 5
	}

	tabStyle := lipgloss.NewStyle().
		Width(width).
		Foreground(m.Theme.MenuDim)

	activeStyle := lipgloss.NewStyle().
		Width(width).
		Foreground(m.Theme.Header).
		Bold(true)

	var tabsLine string
	if m.FilterView == "playlists" {
		tabsLine = activeStyle.Render("[1]Playlists") + tabStyle.Render(" [2]Favorites [3]Recent")
	} else if m.FilterView == "favorites" {
		tabsLine = tabStyle.Render("[1]Playlists ") + activeStyle.Render("[2]Favorites") + tabStyle.Render(" [3]Recent")
	} else if m.FilterView == "recent" {
		tabsLine = tabStyle.Render("[1]Playlists [2]Favorites ") + activeStyle.Render("[3]Recent")
	}

	// Build list content
	var listContent []string
	listHeight := height - 1 // minus 1 for tabs row

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
				listContent = append(listContent, prefix+name)
			}
		}
	case "favorites":
		if m.Library != nil {
			favorites := m.Library.GetFavorites()
			for i, tr := range favorites {
				prefix := "  "
				if i == m.SelectedTrackIndex {
					prefix = "> "
				}
				listContent = append(listContent, prefix+tr.Title+" - "+tr.Artist)
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
				listContent = append(listContent, prefix+marker+" "+tr.Title+" - "+tr.Artist)
			}
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

// lipglossToAnsi converts a lipgloss.Color (string) to its raw ANSI escape
// sequence for foreground color. Supports both hex colors ("#rrggbb") and
// ANSI numbers ("15", "12", "256:N").
func lipglossToAnsi(c lipgloss.Color) string {
	s := string(c)
	if strings.HasPrefix(s, "#") {
		return hexToAnsi(s)
	}
	// ANSI 16-color
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 15 {
		if n < 8 {
			return fmt.Sprintf("\x1b[%dm", 30+n)
		}
		return fmt.Sprintf("\x1b[%dm", 90+n-8)
	}
	// 256-color: "256:N"
	if strings.HasPrefix(s, "256:") {
		if n, err := strconv.Atoi(strings.TrimPrefix(s, "256:")); err == nil {
			return fmt.Sprintf("\x1b[38;5;%dm", n)
		}
	}
	// Fallback
	return "\x1b[93m" // bright yellow
}

// hexToAnsi converts "#rrggbb" to a 24-bit ANSI foreground escape.
func hexToAnsi(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "\x1b[93m"
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}
