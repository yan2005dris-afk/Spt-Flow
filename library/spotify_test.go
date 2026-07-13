package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSpotifyURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantK   string
		wantID  string
		wantErr error
	}{
		// --- Valid: URL form, track ---
		{"https track basic", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ", "track", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"https track with query", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ?si=abc123", "track", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"https track with fragment", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ#some-fragment", "track", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"https track with query and fragment", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ?si=abc#fragment", "track", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"http track basic", "http://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ", "track", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"https track all lowercase id", "https://open.spotify.com/track/abcdefghijklmnopqrstuv", "track", "abcdefghijklmnopqrstuv", nil},
		{"https track all uppercase id", "https://open.spotify.com/track/ABCDEFGHIJKLMNOPQRSTUV", "track", "ABCDEFGHIJKLMNOPQRSTUV", nil},
		{"https track mixed case id", "https://open.spotify.com/track/AbCdEfGhIjKlMnOpQrStUv", "track", "AbCdEfGhIjKlMnOpQrStUv", nil},
		{"https track alphanum id", "https://open.spotify.com/track/0123456789ABCDEFGHIJKL", "track", "0123456789ABCDEFGHIJKL", nil},

		// --- Valid: URL form, playlist ---
		{"https playlist basic", "https://open.spotify.com/playlist/4yYP0CjXhG3L7J6L5cR5bQ", "playlist", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"https playlist with query", "https://open.spotify.com/playlist/4yYP0CjXhG3L7J6L5cR5bQ?si=xyz", "playlist", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"http playlist basic", "http://open.spotify.com/playlist/4yYP0CjXhG3L7J6L5cR5bQ", "playlist", "4yYP0CjXhG3L7J6L5cR5bQ", nil},

		// --- Valid: URI form, track ---
		{"uri track basic", "spotify:track:4yYP0CjXhG3L7J6L5cR5bQ", "track", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"uri track all lowercase", "spotify:track:abcdefghijklmnopqrstuv", "track", "abcdefghijklmnopqrstuv", nil},
		{"uri track all uppercase", "spotify:track:ABCDEFGHIJKLMNOPQRSTUV", "track", "ABCDEFGHIJKLMNOPQRSTUV", nil},

		// --- Valid: URI form, playlist ---
		{"uri playlist basic", "spotify:playlist:4yYP0CjXhG3L7J6L5cR5bQ", "playlist", "4yYP0CjXhG3L7J6L5cR5bQ", nil},
		{"uri playlist lowercase", "spotify:playlist:abcdefghijklmnopqrstuv", "playlist", "abcdefghijklmnopqrstuv", nil},

		// --- Invalid: wrong scheme ---
		{"ftp track", "ftp://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"file track", "file:///track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},

		// --- Invalid: wrong host ---
		{"spotify.com without https", "open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"www.spotify.com", "https://www.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"play.spotify.com", "https://play.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},

		// --- Invalid: wrong path (album) ---
		{"album URL rejected", "https://open.spotify.com/album/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"album URI rejected", "spotify:album:4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"artist URL rejected", "https://open.spotify.com/artist/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"show URL rejected", "https://open.spotify.com/show/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"episode URL rejected", "https://open.spotify.com/episode/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},

		// --- Invalid: wrong length ---
		{"id too short (21 chars)", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5b", "", "", ErrInvalidSpotifyURL},
		{"id too long (23 chars)", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQA", "", "", ErrInvalidSpotifyURL},
		{"uri id too short", "spotify:track:4yYP0CjXhG3L7J6L5cR5b", "", "", ErrInvalidSpotifyURL},
		{"uri id too long", "spotify:track:4yYP0CjXhG3L7J6L5cR5bQA", "", "", ErrInvalidSpotifyURL},

		// --- Invalid: bad characters ---
		{"id with dash rejected", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5b-", "", "", ErrInvalidSpotifyURL},
		{"id with underscore rejected", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5b_", "", "", ErrInvalidSpotifyURL},
		{"id with space rejected", "https://open.spotify.com/track/4yYP0CjXhG3L7J6 5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"id with special char", "https://open.spotify.com/track/4yYP0CjXhG3L7J6L5cR5b!", "", "", ErrInvalidSpotifyURL},
		{"uri id with dash rejected", "spotify:track:4yYP0CjXhG3L7J6L5cR5b-", "", "", ErrInvalidSpotifyURL},

		// --- Invalid: malformed URI ---
		{"uri missing kind", "spotify:4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"uri wrong separator slash", "spotify:track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"uri double colon", "spotify:track::4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"uri empty id", "spotify:track:", "", "", ErrInvalidSpotifyURL},

		// --- Invalid: empty / blank ---
		{"empty string", "", "", "", ErrInvalidSpotifyURL},
		{"whitespace only", "   ", "", "", ErrInvalidSpotifyURL},

		// --- Invalid: open.spotify.com prefix variations ---
		{"open.spotify.com with extra dot", "https://open..spotify.com/track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},
		{"open.spotify.com trailing slash", "https://open.spotify.com//track/4yYP0CjXhG3L7J6L5cR5bQ", "", "", ErrInvalidSpotifyURL},

		// --- Edge: exactly 22 char base62 id ---
		{"min valid id", "https://open.spotify.com/track/0000000000000000000001", "track", "0000000000000000000001", nil},
		{"max valid id", "https://open.spotify.com/track/zzzzzzzzzzzzzzzzzzzzzz", "track", "zzzzzzzzzzzzzzzzzzzzzz", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, id, err := ParseSpotifyURL(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseSpotifyURL(%q) error = %v, want %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr != nil {
				return
			}
			if k != tt.wantK {
				t.Errorf("ParseSpotifyURL(%q) kind = %q, want %q", tt.input, k, tt.wantK)
			}
			if id != tt.wantID {
				t.Errorf("ParseSpotifyURL(%q) id = %q, want %q", tt.input, id, tt.wantID)
			}
		})
	}
}

func TestOpenTrackFallback(t *testing.T) {
	// Create a temp dir with fake xdg-open and gio.
	dir := t.TempDir()
	xdgPath := filepath.Join(dir, "xdg-open")
	gioPath := filepath.Join(dir, "gio")

	// Write fake xdg-open that does nothing.
	if err := os.WriteFile(xdgPath, []byte("#!/bin/sh\nexit 0"), 0755); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	// Patch lookPath to only find our fake binaries.
	origLookPath := lookPath
	lookPath = func(name string) (string, error) {
		if name == "xdg-open" {
			return xdgPath, nil
		}
		if name == "gio" {
			return gioPath, nil
		}
		return "", errors.New("not found")
	}
	defer func() { lookPath = origLookPath }()

	err := OpenTrack("4yYP0CjXhG3L7J6L5cR5bQ")
	if err != nil {
		t.Errorf("OpenTrack() = %v, want nil", err)
	}
}

func TestOpenTrackNoHandler(t *testing.T) {
	origLookPath := lookPath
	lookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}
	defer func() { lookPath = origLookPath }()

	err := OpenTrack("4yYP0CjXhG3L7J6L5cR5bQ")
	if !errors.Is(err, ErrNoURIHanlder) {
		t.Errorf("OpenTrack() = %v, want ErrNoURIHanlder", err)
	}
}

func TestOpenTrackGioFallback(t *testing.T) {
	dir := t.TempDir()
	gioPath := filepath.Join(dir, "gio")

	if err := os.WriteFile(gioPath, []byte("#!/bin/sh\nexit 0"), 0755); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	origLookPath := lookPath
	lookPath = func(name string) (string, error) {
		if name == "xdg-open" {
			return "", errors.New("not found")
		}
		if name == "gio" {
			return gioPath, nil
		}
		return "", errors.New("not found")
	}
	defer func() { lookPath = origLookPath }()

	err := OpenTrack("4yYP0CjXhG3L7J6L5cR5bQ")
	if err != nil {
		t.Errorf("OpenTrack() with gio fallback = %v, want nil", err)
	}
}

func TestOpenTrackExecError(t *testing.T) {
	dir := t.TempDir()
	xdgPath := filepath.Join(dir, "xdg-open")

	// Write a fake xdg-open that fails.
	if err := os.WriteFile(xdgPath, []byte("#!/bin/sh\nexit 1"), 0755); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	origLookPath := lookPath
	lookPath = func(name string) (string, error) {
		if name == "xdg-open" {
			return xdgPath, nil
		}
		return "", errors.New("not found")
	}
	defer func() { lookPath = origLookPath }()

	// Start() returning an error is valid (the process may have exited).
	// We just verify no panic.
	_ = OpenTrack("4yYP0CjXhG3L7J6L5cR5bQ")
}

func TestErrNoURIHanlderMessage(t *testing.T) {
	if ErrNoURIHanlder.Error() == "" {
		t.Error("ErrNoURIHanlder has empty error message")
	}
}

func TestErrInvalidSpotifyURLMessage(t *testing.T) {
	if ErrInvalidSpotifyURL.Error() == "" {
		t.Error("ErrInvalidSpotifyURL has empty error message")
	}
}


