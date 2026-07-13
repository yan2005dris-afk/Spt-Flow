package ui

import (
	"errors"
	"fmt"
	"strings"
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
)

const launchMaxRetries = 3

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
	MprisClient     *mpris.Client
	LyricsClient    *lyrics.Client
	Track           mpris.Track
	Lyrics          lyrics.Lyrics
	PlaybackStatus  string
	Position        time.Duration
	LastUpdated     time.Time
	Width, Height   int
	ScrollOffset    int
	SpotifyRunning  bool
	ErrorMessage    string
	Visualizer      *Visualizer
	SignalChan      chan *dbus.Signal
	LaunchedSpotify bool // set true after first successful LaunchSpotify
	launchRetries   int  // consecutive failed launches; capped at launchMaxRetries
}

func NewModel() Model {
	return Model{
		LastUpdated: time.Now(),
		Visualizer:  NewVisualizer(30, 10),
	}
}

func (m Model) Init() tea.Cmd {
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
			m.ErrorMessage = "Fetching lyrics..."
			cmds = append(cmds, m.fetchLyricsCmd(msg.Track))
		}

		return m, tea.Batch(cmds...)

	case LyricsMsg:
		if msg.Err != nil {
			m.ErrorMessage = msg.Err.Error()
			m.Lyrics = lyrics.Lyrics{}
		} else {
			m.Lyrics = msg.Lyrics
			m.ErrorMessage = ""
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.MprisClient != nil {
				m.MprisClient.Close()
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

	var mainArea string
	if m.Width >= 80 {
		lyricsWidth := m.Width - 32
		visWidth := 30
		mainArea = lipgloss.JoinHorizontal(
			lipgloss.Top,
			renderLyrics(m, lyricsWidth, mainHeight),
			renderVisualizer(m, visWidth, mainHeight),
		)
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
			// Attempt to launch Spotify the FIRST time (and on retries).
			if !m.LaunchedSpotify && m.launchRetries < launchMaxRetries {
				if err := m.MprisClient.LaunchSpotify(); err != nil {
					if errors.Is(err, mpris.ErrFlatpakSandbox) {
						m.ErrorMessage =
							"Spotify is running as Flatpak/Snap — not compatible with this TUI"
						m.SpotifyRunning = false
						return SpotifyStateMsg{Running: false, Err: err}
					}
					m.launchRetries++
					if m.launchRetries >= launchMaxRetries {
						m.ErrorMessage = "Failed to launch Spotify after 3 attempts"
						m.SpotifyRunning = false
						return SpotifyStateMsg{Running: false, Err: err}
					}
					return SpotifyStateMsg{Running: false, Err: err}
				}
				m.LaunchedSpotify = true
			}
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

func renderVisualizer(m Model, width, height int) string {
	if m.Visualizer == nil {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	if m.Visualizer.Width != width {
		m.Visualizer = NewVisualizer(width, float64(height))
	}
	lines := m.Visualizer.Render(height)
	style := lipgloss.NewStyle().Foreground(DefaultTheme.Visualizer)
	var styledLines []string
	for _, l := range lines {
		styledLines = append(styledLines, style.Render(l))
	}
	return strings.Join(styledLines, "\n")
}
