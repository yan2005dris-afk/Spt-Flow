package ui

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tui-spotify/lyrics"
	"tui-spotify/mpris"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

type (
	TickMsg          struct{}
	PollTickMsg      struct{}
	SpotifySignalMsg struct{}
	MenuChoiceMsg    struct{ Choice string }
	MenuTimerMsg     struct{}
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
	MprisClient        *mpris.Client
	LyricsClient       *lyrics.Client
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
	LaunchedSpotify    bool   // set true after first successful LaunchSpotify
	ViewState          string // "menu" or "tui"
	SelectedMenuOption int    // 0-3 for menu navigation
	ShowingHelp        bool   // for ? overlay
	HelpTimer          bool   // if true, ? overlay auto-dismisses
	StatusMessage      string // for check-status option
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

	return Model{
		LastUpdated:        time.Now(),
		Visualizer:         visualizer,
		ViewState:          viewState,
		SelectedMenuOption: 0,
		ShowingHelp:        false,
	}
}

func (m Model) Init() tea.Cmd {
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
				m.MprisClient.Close()
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
		}
		return m, nil

	case MenuTimerMsg:
		if m.ShowingHelp {
			m.ShowingHelp = false
			return m, nil
		}
		m.ViewState = "tui"
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
				m.SelectedMenuOption = (m.SelectedMenuOption + 1) % 5
			case "k", "up":
				m.SelectedMenuOption = (m.SelectedMenuOption - 1 + 5) % 5
			case "enter":
				choices := []string{ChoiceStartLibrespot, ChoiceTUIOnly, ChoiceCheckStatus, ChoiceHelp, ChoiceStartSpotifyDesktop}
				return m, func() tea.Msg { return MenuChoiceMsg{Choice: choices[m.SelectedMenuOption]} }
			case "q", "ctrl+c":
				if m.MprisClient != nil {
					m.MprisClient.Close()
				}
				if m.Visualizer != nil && m.Visualizer.AudioCapture != nil {
					m.Visualizer.AudioCapture.Close()
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

		// ESC key returns to menu from TUI
		if msg.String() == "esc" {
			m.ViewState = "menu"
			m.SelectedMenuOption = 0
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			if m.MprisClient != nil {
				m.MprisClient.Close()
			}
			if m.Visualizer != nil && m.Visualizer.AudioCapture != nil {
				m.Visualizer.AudioCapture.Close()
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
			if !m.Lyrics.Synced {
				m.scrollUp()
			}
		case "j", "down":
			if !m.Lyrics.Synced {
				m.scrollDown()
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

	header := renderHeader(m)
	footer := renderFooter(m)

	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	mainHeight := m.Height - headerHeight - footerHeight
	if mainHeight < 0 {
		mainHeight = 0
	}

	// Grid layout (wide screens): lyrics | album art + visualizer below.
	// Layout:
	//   ┌────────────────────┬──────────┐
	//   │      lyrics        │  album   │
	//   │                    │   art    │
	//   ├────────────────────┴──────────┤
	//   │        visualizer bars        │
	//   └───────────────────────────────┘
	var mainArea string
	if m.Width >= 80 {
		visRowHeight := 3 // rows for visualizer at bottom
		contentHeight := mainHeight - visRowHeight
		if contentHeight < 5 {
			contentHeight = mainHeight
			visRowHeight = 0
		}

		albumWidth := 28
		if albumWidth > m.Width-40 {
			albumWidth = m.Width - 40
		}
		lyricsWidth := m.Width - albumWidth - 1

		lyricsPanel := renderLyrics(m, lyricsWidth, contentHeight)
		albumPanel := renderAlbumArt(m, albumWidth, contentHeight)

		contentArea := lipgloss.JoinHorizontal(lipgloss.Top, lyricsPanel, albumPanel)

		if visRowHeight > 0 {
			visBar := renderVisualizer(m, m.Width, visRowHeight)
			mainArea = lipgloss.JoinVertical(lipgloss.Left, contentArea, visBar)
		} else {
			mainArea = contentArea
		}
	} else {
		mainArea = renderLyrics(m, m.Width, mainHeight)
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

func (m *Model) watchSignalsCmd() tea.Cmd {
	return m.watchSignalCmd()
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
		lyr, err := m.LyricsClient.FetchLyrics(track.Title, track.Artist, track.Album, track.Duration)
		if err != nil {
			return LyricsMsg{Err: err}
		}
		return LyricsMsg{Lyrics: lyr}
	}
}

func renderHeader(m Model) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(DefaultTheme.Header).
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
		footerStyle = footerStyle.Foreground(DefaultTheme.Waiting)
	} else {
		footerStyle = footerStyle.Foreground(DefaultTheme.Footer)
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
		if m.LoadingMessage != "" {
			return lipgloss.NewStyle().
				Width(width).
				Align(lipgloss.Center).
				Foreground(DefaultTheme.Waiting).
				Render(m.LoadingMessage)
		}
		if m.ErrorMessage != "" {
			return lipgloss.NewStyle().
				Width(width).
				Align(lipgloss.Center).
				Foreground(DefaultTheme.Error).
				Render("Error: " + m.ErrorMessage)
		}
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render("No lyrics available")
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
					style = style.Foreground(DefaultTheme.LyricActive).Bold(true)
				} else {
					style = style.Foreground(DefaultTheme.LyricInactive)
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
				style := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Foreground(DefaultTheme.LyricPlain)
				renderedLines = append(renderedLines, style.Render(content))
			} else {
				renderedLines = append(renderedLines, "")
			}
		}
	}
	return strings.Join(renderedLines, "\n")
}

// renderAlbumArt renders a pixelated "album art" block using Unicode
// block characters (█ ▓ ▒ ░) generated from the album/artist name hash.
// The pattern is deterministic per album, creating a consistent "art"
// representation without requiring actual image fetching.
func renderAlbumArt(m Model, width, height int) string {
	if m.Track.Album == "" && m.Track.Artist == "" {
		return emptyAlbumArt(width, height)
	}

	// Seed from album + artist for more uniqueness.
	h := fnv.New64a()
	h.Write([]byte(m.Track.Album + "|" + m.Track.Artist))
	seed := h.Sum64()

	// Pick two colors from the seed (ANSI color indices 1-15).
	c1 := lipgloss.ANSIColor(1 + int(seed%14))
	c2 := lipgloss.ANSIColor(1 + int(seed>>4)%14)

	// Unicode block characters from dark to light.
	blocks := []rune{' ', '░', '▒', '▓', '█'}

	// Render grid of block characters with alternating colors.
	var rows []string
	for row := 0; row < height; row++ {
		var sb strings.Builder
		for col := 0; col < width; col++ {
			// Mix colors in checkerboard-like pattern.
			useC1 := (row+col)%2 == 0
			color := c1
			if !useC1 {
				color = c2
			}

			// Vary density based on position and seed.
			idx := int(seed>>(row+col*3)%8) % len(blocks)
			ch := blocks[idx]

			style := lipgloss.NewStyle().Foreground(color)
			sb.WriteString(style.Render(string(ch)))
		}
		rows = append(rows, sb.String())
	}

	// Center the album name and artist at the bottom of the art block.
	albumStyle := lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Color("15")).
		Bold(true)
	artistStyle := lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Color("7"))

	// Truncate if needed.
	albumName := m.Track.Album
	artistName := m.Track.Artist
	if len(albumName) > width {
		albumName = albumName[:width]
	}
	if len(artistName) > width {
		artistName = artistName[:width]
	}

	// Find last non-empty row to place text at bottom.
	startRow := height - 2
	if startRow < 0 {
		startRow = 0
	}

	// Rebuild with text overlaid in last rows.
	var resultRows []string
	for i := 0; i < height; i++ {
		if i == startRow && albumName != "" {
			resultRows = append(resultRows, lipgloss.Place(width, 1, lipgloss.Center, lipgloss.Bottom, albumStyle.Render(albumName)))
		} else if i == startRow+1 && artistName != "" {
			resultRows = append(resultRows, lipgloss.Place(width, 1, lipgloss.Center, lipgloss.Bottom, artistStyle.Render(artistName)))
		} else {
			resultRows = append(resultRows, rows[i])
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, resultRows...)
}

// emptyAlbumArt renders a placeholder when no track info is available.
func emptyAlbumArt(width, height int) string {
	var rows []string
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i := 0; i < height; i++ {
		row := ""
		for j := 0; j < width; j++ {
			if (i+j)%4 == 0 {
				row += style.Render("▒")
			} else {
				row += style.Render(" ")
			}
		}
		rows = append(rows, row)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func renderVisualizer(m Model, width, height int) string {
	if m.Visualizer == nil {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	if m.Visualizer.Width != width {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	lines := m.Visualizer.Render(height)

	// Use VisualizerBackground color for audio-driven mode,
	// and Visualizer color for procedural mode.
	color := DefaultTheme.Visualizer
	if m.Visualizer.AudioCapture != nil {
		color = DefaultTheme.VisualizerBackground
	}

	style := lipgloss.NewStyle().Foreground(color)
	var styledLines []string
	for _, l := range lines {
		styledLines = append(styledLines, style.Render(l))
	}
	return strings.Join(styledLines, "\n")
}
