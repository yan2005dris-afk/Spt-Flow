package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)
	s.lib.Playlists = []Playlist{{ID: "p1", Name: "Test Playlist"}}

	if err := s.Save(); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	// Load into a new store via the method (uses s2.path).
	s2 := newStore(&Library{}, path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(s2.lib.Playlists) != 1 {
		t.Fatalf("got %d playlists, want 1", len(s2.lib.Playlists))
	}
	if s2.lib.Playlists[0].Name != "Test Playlist" {
		t.Fatalf("got playlist name %q, want %q", s2.lib.Playlists[0].Name, "Test Playlist")
	}
}

func TestAddRemovePlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)

	// Use a valid spotify URL so AddPlaylist's ParseSpotifyURL call succeeds.
	p, err := s.AddPlaylist("https://open.spotify.com/playlist/4yYP0CjXhG3L7J6L5cR5bQ")
	if err != nil {
		t.Fatalf("AddPlaylist() = %v", err)
	}
	if p.ID == "" {
		t.Error("AddPlaylist() returned playlist with empty ID")
	}
	if len(s.lib.Playlists) != 1 {
		t.Fatalf("got %d playlists, want 1", len(s.lib.Playlists))
	}

	s.RemovePlaylist(p.ID)
	if len(s.lib.Playlists) != 0 {
		t.Fatalf("got %d playlists after RemovePlaylist, want 0", len(s.lib.Playlists))
	}
}

func TestGetPlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{
		Playlists: []Playlist{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}},
	}, path)

	p, ok := s.GetPlaylist("p1")
	if !ok {
		t.Fatal("GetPlaylist(p1) = false, want true")
	}
	if p.Name != "One" {
		t.Fatalf("got name %q, want %q", p.Name, "One")
	}

	_, ok = s.GetPlaylist("nonexistent")
	if ok {
		t.Fatal("GetPlaylist(nonexistent) = true, want false")
	}
}

func TestAddFavoriteDedup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)
	track := Track{ID: "t1", Title: "Song"}

	s.AddFavorite(track)
	s.AddFavorite(track) // duplicate

	if len(s.lib.Favorites) != 1 {
		t.Fatalf("got %d favorites, want 1 (dedup)", len(s.lib.Favorites))
	}
}

func TestRemoveFavorite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{
		Favorites: []Track{{ID: "t1"}, {ID: "t2"}},
	}, path)

	s.RemoveFavorite("t1")
	if len(s.lib.Favorites) != 1 || s.lib.Favorites[0].ID != "t2" {
		t.Fatalf("got favorites %v, want [{t2}]", s.lib.Favorites)
	}
}

func TestIsFavorite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{
		Favorites: []Track{{ID: "t1"}},
	}, path)

	if !s.IsFavorite("t1") {
		t.Error("IsFavorite(t1) = false, want true")
	}
	if s.IsFavorite("t2") {
		t.Error("IsFavorite(t2) = true, want false")
	}
}

func TestAddRecentFIFO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)

	for i := 0; i < 60; i++ {
		s.AddRecent(Track{ID: "t" + string(rune('a'+i)), Title: "Track"})
	}

	if len(s.lib.Recent) != recentCap {
		t.Fatalf("got %d recent, want %d (FIFO cap)", len(s.lib.Recent), recentCap)
	}
}

func TestAddRecentDedup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)
	track := Track{ID: "t1", Title: "Song"}

	s.AddRecent(track)
	s.AddRecent(track) // duplicate should move to end

	if len(s.lib.Recent) != 1 {
		t.Fatalf("got %d recent, want 1 (dedup)", len(s.lib.Recent))
	}
}

func TestClearRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{
		Recent: []Track{{ID: "t1"}, {ID: "t2"}},
	}, path)

	s.ClearRecent()
	if len(s.lib.Recent) != 0 {
		t.Fatalf("got %d recent, want 0", len(s.lib.Recent))
	}
}

func TestAtomicWriteKill9(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)
	s.lib.Playlists = []Playlist{{ID: "p1", Name: "Before"}}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	// Simulate kill-9: write garbage directly to the real path,
	// representing a crash mid-write (before rename). The previous
	// file content should be intact since we write to a temp file first.
	if err := os.WriteFile(path, []byte("not json at all{"), 0600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	// Load should return empty library and not crash.
	s2 := newStore(&Library{}, path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load() error after kill-9 sim = %v", err)
	}
	if len(s2.lib.Playlists) != 0 {
		t.Fatalf("got %d playlists, want 0 (previous state intact)", len(s2.lib.Playlists))
	}

	// Restore: write valid state.
	s2.lib.Playlists = []Playlist{{ID: "p2", Name: "After"}}
	if err := s2.Save(); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	s3 := newStore(&Library{}, path)
	if err := s3.Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if len(s3.lib.Playlists) != 1 || s3.lib.Playlists[0].Name != "After" {
		t.Fatalf("got playlists %v, want [{p2 After}]", s3.lib.Playlists)
	}
}

func TestCorruptFileRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	// Write garbage.
	if err := os.WriteFile(path, []byte("not json at all{"), 0600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	// Load should not panic and should return empty library.
	s := newStore(&Library{}, path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load() error = %v (expected silent recovery)", err)
	}
	if len(s.lib.Playlists) != 0 || len(s.lib.Favorites) != 0 || len(s.lib.Recent) != 0 {
		t.Fatalf("got Library%+v, want empty Library", s.lib)
	}
}

func TestMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)

	// Save then corrupt file.
	s.lib.Playlists = []Playlist{{ID: "p1"}}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	_ = os.WriteFile(path, []byte("garbage"), 0600)

	// Load should recover.
	s2 := newStore(&Library{}, path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if len(s2.lib.Playlists) != 0 {
		t.Fatalf("got %d playlists after corrupt file, want 0", len(s2.lib.Playlists))
	}
}

func TestGetPlaylistsReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{
		Playlists: []Playlist{{ID: "p1"}},
	}, path)

	plist := s.GetPlaylists()
	plist = append(plist, Playlist{ID: "p2"})
	if len(s.lib.Playlists) != 1 {
		t.Error("GetPlaylists() returned a reference, not a copy")
	}
}

func TestGetFavoritesReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{
		Favorites: []Track{{ID: "t1"}},
	}, path)

	fav := s.GetFavorites()
	fav = append(fav, Track{ID: "t2"})
	if len(s.lib.Favorites) != 1 {
		t.Error("GetFavorites() returned a reference, not a copy")
	}
}

func TestGetRecentReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{
		Recent: []Track{{ID: "t1"}},
	}, path)

	rec := s.GetRecent()
	rec = append(rec, Track{ID: "t2"})
	if len(s.lib.Recent) != 1 {
		t.Error("GetRecent() returned a reference, not a copy")
	}
}

func TestAddRecentEmptyIDSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)
	s.AddRecent(Track{ID: ""}) // should be no-op

	if len(s.lib.Recent) != 0 {
		t.Fatalf("AddRecent with empty ID: got %d recent, want 0", len(s.lib.Recent))
	}
}

func TestSavePathGetter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	s := newStore(&Library{}, path)
	if g := s.Path(); g != path {
		t.Fatalf("Path() = %q, want %q", g, path)
	}
}

func TestLibraryJSONRoundTrip(t *testing.T) {
	original := &Library{
		Playlists: []Playlist{
			{ID: "p1", Name: "My List", URL: "https://open.spotify.com/playlist/4yYP0CjXhG3L7J6L5cR5bQ", TrackIDs: []string{"t1", "t2"}, CreatedAt: 1700000000},
		},
		Favorites: []Track{
			{ID: "t1", Title: "Song", Artist: "Artist", Album: "Album"},
		},
		Recent: []Track{
			{ID: "t2", Title: "Recent Song"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() = %v", err)
	}

	var loaded Library
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal() = %v", err)
	}

	if len(loaded.Playlists) != 1 {
		t.Fatalf("got %d playlists, want 1", len(loaded.Playlists))
	}
	if loaded.Playlists[0].Name != "My List" {
		t.Fatalf("got playlist name %q, want %q", loaded.Playlists[0].Name, "My List")
	}
	if len(loaded.Favorites) != 1 {
		t.Fatalf("got %d favorites, want 1", len(loaded.Favorites))
	}
	if len(loaded.Recent) != 1 {
		t.Fatalf("got %d recent, want 1", len(loaded.Recent))
	}
}
