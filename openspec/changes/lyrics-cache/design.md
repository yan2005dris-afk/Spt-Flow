# Design: Lyrics Cache

## Architecture

```
+--------------------+
|     ui/shell.go    |
|  fetchLyricsCmd    |
+--------+-----------+
         | Store.Get(title, artist, album, dur)
         v
+--------------------+
|   lyrics/cache     |   <-- new package
|     Store          |
|                    |
|  in-memory map     |   <-- guarded by sync.Mutex
|  atomic disk I/O   |
+--------+-----------+
         |  miss / stale
         v
+--------------------+
|   lyrics.Client    |   <-- existing, untouched
|   FetchLyrics      |
+--------+-----------+
         |
         v
      lrclib.net
```

The cache is a thin wrapper around the existing `lyrics.Client`. It does not modify `FetchLyrics`; it sits in front.

## Package Layout

```
lyrics/
  engine.go        # existing: Lyrics, LyricsLine, Client, FetchLyrics
  engine_test.go   # existing
  cache/
    cache.go       # new: Store type, public API
    cache_test.go  # new
```

We keep it in a sub-package `lyrics/cache` so:
- `lyrics.Client` stays network-only and trivially testable.
- `cache.Store` can be tested with stubbed HTTP servers.
- The TUI's `ui/shell.go` only imports one extra package.

## Type Design

```go
package cache

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"

    "tui-spotify/lyrics"
)

const (
    DefaultTTL = 7 * 24 * time.Hour
    DefaultCap = 200
)

type Entry struct {
    Title        string           `json:"title"`
    Artist       string           `json:"artist"`
    Album        string           `json:"album"`
    FetchedAt    time.Time        `json:"fetchedAt"`
    LastAccessed time.Time        `json:"lastAccessed"`
    Lyrics       lyrics.Lyrics    `json:"lyrics"`
}

type Store struct {
    mu      sync.Mutex
    client  *lyrics.Client
    path    string
    ttl     time.Duration
    cap     int
    entries map[string]Entry // keyed by SHA-256 hex
}

func New(path string, client *lyrics.Client) (*Store, error)
func (s *Store) Get(title, artist, album string, duration time.Duration) (lyrics.Lyrics, error)
func (s *Store) Clear() error
func (s *Store) Size() int
```

## Key Computation

```go
func key(title, artist, album string) string {
    raw := strings.ToLower(strings.TrimSpace(title)) + "|" +
        strings.ToLower(strings.TrimSpace(artist)) + "|" +
        strings.ToLower(strings.TrimSpace(album))
    sum := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(sum[:])
}
```

Duration is intentionally NOT in the key — same song across releases should hit.

## Get Flow

```
func (s *Store) Get(title, artist, album string, duration time.Duration) (lyrics.Lyrics, error) {
    k := key(title, artist, album)

    s.mu.Lock()
    defer s.mu.Unlock()

    now := time.Now()

    // 1. Hit check
    if e, ok := s.entries[k]; ok {
        if now.Sub(e.FetchedAt) < s.ttl {
            e.LastAccessed = now
            s.entries[k] = e
            return e.Lyrics, nil
        }
        // Stale — drop and fall through
        delete(s.entries, k)
    }

    // 2. Miss / stale → network
    l, err := s.client.FetchLyrics(title, artist, album, duration)
    if err != nil {
        return lyrics.Lyrics{}, err
    }

    // 3. Persist
    s.entries[k] = Entry{
        Title:        title,
        Artist:       artist,
        Album:        album,
        FetchedAt:    now,
        LastAccessed: now,
        Lyrics:       l,
    }
    s.evictIfOver()
    if err := s.writeAtomic(); err != nil {
        // Log to stderr; do not return error to caller.
        fmt.Fprintf(os.Stderr, "cache: write failed: %v\n", err)
    }
    return l, nil
}
```

## LRU Eviction

```go
func (s *Store) evictIfOver() {
    if len(s.entries) <= s.cap {
        return
    }
    type kv struct {
        k string
        t time.Time
    }
    sorted := make([]kv, 0, len(s.entries))
    for k, e := range s.entries {
        sorted = append(sorted, kv{k, e.LastAccessed})
    }
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].t.Before(sorted[j].t)
    })
    // Drop oldest until under cap.
    for len(s.entries) > s.cap {
        delete(s.entries, sorted[0].k)
        sorted = sorted[1:]
    }
}
```

## Atomic Write

```go
func (s *Store) writeAtomic() error {
    dir := filepath.Dir(s.path)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return err
    }
    tmp, err := os.CreateTemp(dir, ".lyrics-*.json.tmp")
    if err != nil {
        return err
    }
    tmpName := tmp.Name()
    defer os.Remove(tmpName) // no-op if rename succeeded

    enc := json.NewEncoder(tmp)
    enc.SetIndent("", "  ")
    if err := enc.Encode(s.entries); err != nil {
        tmp.Close()
        return err
    }
    if err := tmp.Sync(); err != nil {
        tmp.Close()
        return err
    }
    if err := tmp.Close(); err != nil {
        return err
    }
    return os.Rename(tmpName, s.path)
}
```

`rename` is atomic on POSIX — the target either stays as the old file or becomes the new file, never half-written.

## Load on Startup

```go
func New(path string, client *lyrics.Client) (*Store, error) {
    s := &Store{
        client:  client,
        path:    path,
        ttl:     DefaultTTL,
        cap:     DefaultCap,
        entries: make(map[string]Entry),
    }

    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return s, nil // cold start, fine
        }
        return nil, err
    }

    if err := json.Unmarshal(data, &s.entries); err != nil {
        // Corrupt file — start fresh, log warning.
        fmt.Fprintf(os.Stderr, "cache: corrupt file %q, ignoring: %v\n", path, err)
        s.entries = make(map[string]Entry)
        return s, nil
    }

    // Drop already-stale entries on load.
    now := time.Now()
    for k, e := range s.entries {
        if now.Sub(e.FetchedAt) >= s.ttl {
            delete(s.entries, k)
        }
    }
    return s, nil
}
```

## Wire-In to shell.go

Currently:

```go
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
```

Becomes:

```go
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
                // Fall back to direct client — log and continue.
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
```

`Model.LyricsCache *cache.Store` is added to the `Model` struct in `ui/shell.go`.

## File Size

200 entries × ~5 KB per entry (avg lyrics ~50 lines × 50 chars × JSON overhead) = ~1 MB worst case. Negligible.

## Tradeoffs

| Option | Pro | Con |
| --- | --- | --- |
| Plain JSON | Simple, debuggable, human-readable | Whole-file rewrite on every change |
| BoltDB / SQLite | Faster lookups, partial writes | New dep, more code, harder to inspect |
| Append-only log | Cheap writes | Reads require full scan, occasional compaction |
| **Chosen: JSON** | Zero deps, easy debugging | OK for 200-entry cap |

We chose JSON because the dataset is tiny (≤200 entries × 5 KB = 1 MB). The whole-file rewrite is fast enough and the human-readability is a debugging win.

## Open Questions

None. The cache layer is well-scoped and the existing Client is a stable interface.