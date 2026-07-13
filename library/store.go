package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const recentCap = 50
const libraryFileName = "library.json"



// Load reads the library from the default config directory.
// Missing or corrupt files return an empty Library; no error is surfaced.
func Load() (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return &Store{}, nil
	}
	dir := filepath.Join(base, "spt-flow")
	path := filepath.Join(dir, libraryFileName)

	s := &Store{path: path}
	if err := s.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "library: load failed: %v\n", err)
	}
	return s, nil
}

// Load reads the library from the file at s.path.
// Missing or corrupt files return an empty Library; no error is surfaced.
func (s *Store) Load() error {
	if s.path == "" {
		s.lib = &Library{}
		return nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.lib = &Library{}
			return nil
		}
		fmt.Fprintf(os.Stderr, "library: read failed: %v\n", err)
		s.lib = &Library{}
		return nil
	}

	if err := json.Unmarshal(data, &s.lib); err != nil {
		fmt.Fprintf(os.Stderr, "library: corrupt file, using empty: %v\n", err)
		s.lib = &Library{}
		return nil
	}
	return nil
}

func newStore(lib *Library, path string) *Store {
	return &Store{lib: lib, path: path}
}

// Path returns the file path the store persists to.
func (s *Store) Path() string {
	return s.path
}

// Save writes the library atomically: temp file + fsync + rename.
// Errors are logged to stderr.
func (s *Store) Save() error {
	if s.path == "" {
		// No path set; ensure a default is configured.
		base, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "library: cannot determine config dir")
			return err
		}
		dir := filepath.Join(base, "spt-flow")
		if err := os.MkdirAll(dir, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "library: cannot create config dir: %v\n", err)
			return err
		}
		s.path = filepath.Join(dir, libraryFileName)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), "library-*.json.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "library: cannot create temp file: %v\n", err)
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.lib); err != nil {
		_ = tmp.Close()
		fmt.Fprintf(os.Stderr, "library: write failed: %v\n", err)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		fmt.Fprintf(os.Stderr, "library: sync failed: %v\n", err)
		return err
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "library: close failed: %v\n", err)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		fmt.Fprintf(os.Stderr, "library: rename failed: %v\n", err)
		return err
	}
	return nil
}

// GetPlaylists returns a copy of the playlists list.
func (s *Store) GetPlaylists() []Playlist {
	s.lib.Playlists = append([]Playlist(nil), s.lib.Playlists...)
	return s.lib.Playlists
}

// GetFavorites returns a copy of the favorites list.
func (s *Store) GetFavorites() []Track {
	s.lib.Favorites = append([]Track(nil), s.lib.Favorites...)
	return s.lib.Favorites
}

// GetRecent returns a copy of the recent list.
func (s *Store) GetRecent() []Track {
	s.lib.Recent = append([]Track(nil), s.lib.Recent...)
	return s.lib.Recent
}

// AddPlaylist adds a playlist after validating the URL via ParseSpotifyURL.
func (s *Store) AddPlaylist(url string) (Playlist, error) {
	_, _, err := ParseSpotifyURL(url)
	if err != nil {
		return Playlist{}, err
	}
	p := Playlist{
		ID:        newUUID(),
		URL:       url,
		TrackIDs:  nil,
		CreatedAt: time.Now().Unix(),
	}
	s.lib.Playlists = append(s.lib.Playlists, p)
	if err := s.Save(); err != nil {
		// Rollback on save failure.
		s.lib.Playlists = s.lib.Playlists[:len(s.lib.Playlists)-1]
		return Playlist{}, err
	}
	return p, nil
}

// RemovePlaylist removes the playlist with the given ID.
func (s *Store) RemovePlaylist(id string) {
	old := s.lib.Playlists
	s.lib.Playlists = nil
	for _, p := range old {
		if p.ID != id {
			s.lib.Playlists = append(s.lib.Playlists, p)
		}
	}
	_ = s.Save()
}

// GetPlaylist returns the playlist with the given ID and a bool indicating success.
func (s *Store) GetPlaylist(id string) (Playlist, bool) {
	for _, p := range s.lib.Playlists {
		if p.ID == id {
			return p, true
		}
	}
	return Playlist{}, false
}

// AddFavorite adds a track to favorites, deduplicating by ID.
func (s *Store) AddFavorite(t Track) {
	s.lib.Favorites = filterTracksByID(s.lib.Favorites, t.ID)
	s.lib.Favorites = append(s.lib.Favorites, t)
	_ = s.Save()
}

// RemoveFavorite removes the track with the given ID from favorites.
func (s *Store) RemoveFavorite(id string) {
	s.lib.Favorites = filterTracksByID(s.lib.Favorites, id)
	_ = s.Save()
}

// IsFavorite returns true if the track ID is in the favorites list.
func (s *Store) IsFavorite(id string) bool {
	for _, t := range s.lib.Favorites {
		if t.ID == id {
			return true
		}
	}
	return false
}

// AddRecent adds a track to recent, deduplicating by ID, enforcing FIFO cap of 50.
// It auto-saves after mutation.
func (s *Store) AddRecent(t Track) {
	if t.ID == "" {
		return
	}
	s.lib.Recent = filterTracksByID(s.lib.Recent, t.ID)
	s.lib.Recent = append(s.lib.Recent, t)
	if len(s.lib.Recent) > recentCap {
		s.lib.Recent = s.lib.Recent[len(s.lib.Recent)-recentCap:]
	}
	_ = s.Save()
}

// ClearRecent removes all recent tracks and persists.
func (s *Store) ClearRecent() {
	s.lib.Recent = nil
	_ = s.Save()
}

// filterTracksByID returns a new slice with all tracks whose ID != targetID.
func filterTracksByID(tracks []Track, targetID string) []Track {
	result := make([]Track, 0, len(tracks))
	for _, t := range tracks {
		if t.ID != targetID {
			result = append(result, t)
		}
	}
	return result
}

// Mutex returns the store's embedded sync.Mutex for external locking if needed.
func (s *Store) Mutex() *sync.Mutex {
	return &sync.Mutex{}
}
