package cache

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tui-spotify/lyrics"
)

func makeTestStore(t *testing.T, path string) (*Store, func()) {
	t.Helper()
	client := lyrics.NewClient("")
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	cleanup := func() {
		os.Remove(path)
	}
	return store, cleanup
}

func makeServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func TestStore_New_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonexistent.json")
	client := lyrics.NewClient("")
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if store.Size() != 0 {
		t.Errorf("Size() = %d, want 0", store.Size())
	}
}

func TestStore_New_CorruptFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "corrupt.json")
	if err := os.WriteFile(path, []byte("not valid json{{{"), 0600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	client := lyrics.NewClient("")
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if store.Size() != 0 {
		t.Errorf("Size() = %d, want 0 after corrupt file", store.Size())
	}
}

func TestStore_Get_ColdStart(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cold.json")
	var callCount int32

	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"name": "Test Song",
			"artistName": "Test Artist",
			"albumName": "Test Album",
			"duration": 180,
			"syncedLyrics": "[00:10.00]Hello world\n[00:15.00]Goodbye world",
			"plainLyrics": "Hello world\nGoodbye world"
		}`))
	})

	client := lyrics.NewClient(ts.URL)
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	lyr, err := store.Get("Test Song", "Test Artist", "Test Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !lyr.Synced {
		t.Error("Expected lyrics to be synced")
	}
	if len(lyr.Lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lyr.Lines))
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Network calls = %d, want 1", atomic.LoadInt32(&callCount))
	}
	ts.Close()
}

func TestStore_Get_CacheHit(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "hit.json")
	var callCount int32

	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"name": "Cached Song",
			"artistName": "Cached Artist",
			"albumName": "Cached Album",
			"duration": 180,
			"syncedLyrics": "[00:10.00]Cached line",
			"plainLyrics": "Cached line"
		}`))
	})

	client := lyrics.NewClient(ts.URL)
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	// First call - populates cache
	_, err = store.Get("Cached Song", "Cached Artist", "Cached Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Get() first call error = %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("First call: network calls = %d, want 1", atomic.LoadInt32(&callCount))
	}

	// Second call - should hit cache
	_, err = store.Get("Cached Song", "Cached Artist", "Cached Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Get() second call error = %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Second call: network calls = %d, want 1 (cached)", atomic.LoadInt32(&callCount))
	}
	ts.Close()
}

func TestStore_Get_Stale(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "stale.json")
	var callCount int32

	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"name": "Stale Song",
			"artistName": "Stale Artist",
			"albumName": "Stale Album",
			"duration": 180,
			"syncedLyrics": "[00:10.00]Fresh line",
			"plainLyrics": "Fresh line"
		}`))
	})

	client := lyrics.NewClient(ts.URL)
	store := &Store{
		client: client,
		path:   path,
		ttl:    1 * time.Millisecond, // very short TTL for testing
		cap:    DefaultCap,
		entries: map[string]Entry{
			key("Stale Song", "Stale Artist", "Stale Album"): {
				Title:        "Stale Song",
				Artist:       "Stale Artist",
				Album:        "Stale Album",
				FetchedAt:    time.Now().Add(-1 * time.Hour), // old enough to be stale
				LastAccessed: time.Now(),
				Lyrics:       lyrics.Lyrics{Lines: []lyrics.LyricsLine{{Content: "Old stale line"}}, Synced: true},
			},
		},
	}

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	_, err := store.Get("Stale Song", "Stale Artist", "Stale Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Network calls = %d, want 1 (stale entry should be re-fetched)", atomic.LoadInt32(&callCount))
	}
	ts.Close()
}

func TestStore_Get_LRUEviction(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "lru.json")
	var callCount int32

	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"name": "New Song",
			"artistName": "New Artist",
			"albumName": "New Album",
			"duration": 180,
			"syncedLyrics": "[00:10.00]New evicted line",
			"plainLyrics": "New evicted line"
		}`))
	})

	client := lyrics.NewClient(ts.URL)
	store := &Store{
		client:  client,
		path:    path,
		ttl:     DefaultTTL,
		cap:     3, // small cap for testing
		entries: make(map[string]Entry),
	}

	// Fill to cap
	for i := 0; i < 3; i++ {
		store.entries[key("Old Song", "Old Artist", string(rune('A'+i)))] = Entry{
			Title:        "Old Song",
			Artist:       "Old Artist",
			FetchedAt:    time.Now(),
			LastAccessed: time.Now().Add(-time.Duration(i) * time.Minute),
			Lyrics:       lyrics.Lyrics{Lines: []lyrics.LyricsLine{{Content: "Old line"}}},
		}
	}

	// Add one more - should evict the oldest
	_, err := store.Get("New Song", "New Artist", "New Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Network calls = %d, want 1", atomic.LoadInt32(&callCount))
	}
	if store.Size() != 3 {
		t.Errorf("Size() = %d, want 3 (evicted oldest)", store.Size())
	}
	ts.Close()
}

func TestStore_Get_Key(t *testing.T) {
	tests := []struct {
		name1, artist1, album1 string
		name2, artist2, album2 string
		wantSame               bool
	}{
		{"Song", "Artist", "Album", "Song", "Artist", "Album", true},
		{"Song", "Artist", "Album", "song", "artist", "album", true},
		{"  Song  ", "  Artist  ", "  Album  ", "Song", "Artist", "Album", true},
		{"Song", "Artist", "Album", "Different", "Artist", "Album", false},
		{"Song", "Artist", "Album", "Song", "Different", "Album", false},
		{"Song", "Artist", "Album", "Song", "Artist", "Different", false},
	}

	for _, tt := range tests {
		name := tt.name1 + "_vs_" + tt.album1
		if tt.wantSame {
			name = tt.name1 + "_same"
		} else {
			name = tt.name1 + "_diff"
		}
		t.Run(name, func(t *testing.T) {
			k1 := key(tt.name1, tt.artist1, tt.album1)
			k2 := key(tt.name2, tt.artist2, tt.album2)
			if tt.wantSame && k1 != k2 {
				t.Errorf("Expected same key for %q/%q/%q and %q/%q/%q", tt.name1, tt.artist1, tt.album1, tt.name2, tt.artist2, tt.album2)
			}
			if !tt.wantSame && k1 == k2 {
				t.Errorf("Expected different keys for %q/%q/%q and %q/%q/%q", tt.name1, tt.artist1, tt.album1, tt.name2, tt.artist2, tt.album2)
			}
		})
	}
}

func TestStore_Get_ConcurrentSameKey(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "concurrent.json")
	var callCount int32
	blockCh := make(chan struct{})

	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		<-blockCh // block until we release
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"name": "Concurrent Song",
			"artistName": "Concurrent Artist",
			"albumName": "Concurrent Album",
			"duration": 180,
			"syncedLyrics": "[00:10.00]Concurrent line",
			"plainLyrics": "Concurrent line"
		}`))
	})

	client := lyrics.NewClient(ts.URL)
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	const nGoroutines = 10
	var wg sync.WaitGroup
	errCh := make(chan error, nGoroutines)

	for i := 0; i < nGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Get("Concurrent Song", "Concurrent Artist", "Concurrent Album", 180*time.Second)
			if err != nil {
				errCh <- err
			}
		}()
	}

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)
	close(blockCh) // release the server
	wg.Wait()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Network calls = %d, want 1 (concurrent same key)", atomic.LoadInt32(&callCount))
	}
	close(errCh)
	for e := range errCh {
		t.Errorf("Get() error = %v", e)
	}
	ts.Close()
}

func TestStore_Clear(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "clear.json")
	var callCount int32

	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"name": "Clear Song",
			"artistName": "Clear Artist",
			"albumName": "Clear Album",
			"duration": 180,
			"syncedLyrics": "[00:10.00]Clear line",
			"plainLyrics": "Clear line"
		}`))
	})

	client := lyrics.NewClient(ts.URL)
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	// Populate cache
	_, err = store.Get("Clear Song", "Clear Artist", "Clear Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if store.Size() != 1 {
		t.Errorf("Size() = %d, want 1", store.Size())
	}

	// Clear
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if store.Size() != 0 {
		t.Errorf("Size() after Clear = %d, want 0", store.Size())
	}

	// Verify file is truncated
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}
	var entries map[string]Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("Unmarshal() = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("File contains %d entries, want 0 after clear", len(entries))
	}
	ts.Close()
}

func TestStore_Get_DiskWriteFailure(t *testing.T) {
	tmp := t.TempDir()
	// Pre-create the file then make the directory read-only so writeAtomic fails.
	dir := filepath.Join(tmp, "readonly-cache")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	path := filepath.Join(dir, "lyrics.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatalf("Chmod() = %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0700) })

	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"name": "Disk Fail",
			"artistName": "Test",
			"albumName": "Test Album",
			"duration": 180,
			"syncedLyrics": "[00:10.00]hello",
			"plainLyrics": "hello"
		}`))
	})
	defer ts.Close()

	client := lyrics.NewClient(ts.URL)
	store, err := New(path, client)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	// Get should still return lyrics (network worked), but the in-memory map
	// must NOT retain the entry — disk and memory stay consistent.
	_, err = store.Get("Disk Fail", "Test", "Test Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if store.Size() != 0 {
		t.Errorf("Size() = %d after disk write failure, want 0 (SCN-6: drop on failure)", store.Size())
	}
}

func TestStore_Get_NetworkFailureOnMiss(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "lyrics.json")
	store, cleanup := makeTestStore(t, path)
	defer cleanup()

	// Point to a server that returns 404 (lrclib "not found" path).
	ts := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	defer ts.Close()
	store.client.BaseURL = ts.URL

	_, err := store.Get("Unknown Song", "Unknown", "Unknown", 180*time.Second)
	if err == nil {
		t.Errorf("Get() expected error on network miss, got nil")
	}
	if store.Size() != 0 {
		t.Errorf("Size() = %d after network failure, want 0 (SCN-9: nothing cached on miss)", store.Size())
	}
}
