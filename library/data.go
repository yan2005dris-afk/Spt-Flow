package library

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Track represents a Spotify track.
type Track struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Artist   string        `json:"artist"`
	Album    string        `json:"album"`
	Duration time.Duration `json:"duration"`
}

// Playlist represents a Spotify playlist saved by the user.
type Playlist struct {
	ID        string   `json:"id"`         // crypto/rand v4 UUID
	Name      string   `json:"name"`       // user-supplied label
	URL       string   `json:"url"`        // original Spotify URL
	TrackIDs  []string `json:"track_ids"`  // Spotify track IDs (URI fragments)
	CreatedAt int64    `json:"created_at"` // unix seconds
}

// Library holds all user-managed playlists, favorites, and recent tracks.
type Library struct {
	Playlists []Playlist `json:"playlists"`
	Favorites []Track    `json:"favorites"`
	Recent    []Track    `json:"recent"` // max 50, FIFO
}

// Store manages the library persistence.
type Store struct {
	lib  *Library
	path string
}

// newUUID generates a v4 UUID using crypto/rand (no external dependencies).
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return hex.EncodeToString(b)
}
