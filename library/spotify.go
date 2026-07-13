package library

import (
	"errors"
	"os/exec"
	"regexp"
)

var (
	// ErrNoURIHanlder is returned when neither xdg-open nor gio is available.
	ErrNoURIHanlder = errors.New("neither xdg-open nor gio open is installed")
	// ErrInvalidSpotifyURL is returned when the URL does not match Spotify's format.
	ErrInvalidSpotifyURL = errors.New("invalid Spotify URL")
)

// Two anchored regexes: one for https://open.spotify.com/ form, one for spotify:uri form.
// The URL form handles: bare ID, ?query, #fragment, ?query#fragment.
var spotifyURL = regexp.MustCompile(`^https?://open\.spotify\.com/(track|playlist)/([A-Za-z0-9]{22})(?:\?[^#]*)?(?:#.*)?$`)
var spotifyURI = regexp.MustCompile(`^spotify:(track|playlist):([A-Za-z0-9]{22})$`)

// ParseSpotifyURL parses a raw Spotify URL or URI string and returns the kind
// ("track" or "playlist") and the ID. It returns ErrInvalidSpotifyURL for
// malformed or unsupported URLs (e.g. album links).
func ParseSpotifyURL(raw string) (kind, id string, err error) {
	if matches := spotifyURL.FindStringSubmatch(raw); len(matches) == 3 {
		return matches[1], matches[2], nil
	}
	if matches := spotifyURI.FindStringSubmatch(raw); len(matches) == 3 {
		return matches[1], matches[2], nil
	}
	return "", "", ErrInvalidSpotifyURL
}

// OpenTrack opens a Spotify track in the user's browser via xdg-open, falling
// back to gio open. It returns ErrNoURIHanlder if neither is available.
func OpenTrack(id string) error {
	uri := "spotify:track:" + id

	if _, err := lookPath("xdg-open"); err == nil {
		return exec.Command("xdg-open", uri).Start()
	}
	if _, err := lookPath("gio"); err == nil {
		return exec.Command("gio", "open", uri).Start()
	}
	return ErrNoURIHanlder
}

// lookPath is a testable wrapper around os/exec.LookPath.
var lookPath = exec.LookPath
