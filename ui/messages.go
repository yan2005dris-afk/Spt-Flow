package ui

import (
	"time"

	"tui-spotify/lyrics"
	"tui-spotify/mpris"
)

type (
	TickMsg           struct{}
	PollTickMsg       struct{}
	SpotifySignalMsg  struct{}
	MenuChoiceMsg     struct{ Choice string }
	MenuTimerMsg      struct{}
	LibraryChangedMsg struct{}
	AddPlaylistMsg    struct{ URL string }
	DeletePlaylistMsg struct{ ID string }
	ToggleFavoriteMsg struct{ TrackID string }
	PlayTrackMsg      struct{ TrackID string }
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