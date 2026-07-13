# Spec: Lyrics Cache

## Requirements

### REQ-1: Cache Storage Location

The cache file MUST be stored at `$XDG_CACHE_HOME/spt-flow/lyrics.json`. If `XDG_CACHE_HOME` is unset, MUST fall back to `~/.cache/spt-flow/lyrics.json`. The directory MUST be created with mode `0700` if it doesn't exist (user-only, since lyrics files may contain user preferences).

### REQ-2: Cache Key

The cache key MUST be the lowercase hex SHA-256 of `lowercase(trim(title))|lowercase(trim(artist))|lowercase(trim(album))`. Duration MUST NOT be part of the key — different versions of the same recording should share a cache entry.

### REQ-3: Cache Entry Schema

```json
{
  "<sha256-key>": {
    "title":       "string",
    "artist":      "string",
    "album":       "string",
    "fetchedAt":   "RFC3339 timestamp",
    "lastAccessed":"RFC3339 timestamp",
    "synced":      "boolean",
    "lines": [
      {"ts": "duration-nanos", "content": "string"}
    ]
  }
}
```

- `fetchedAt` is set on creation.
- `lastAccessed` is updated on every read.
- `ts` is `time.Duration` serialized as `int64` nanoseconds.
- `lines` may be empty when the lyrics were genuinely empty (a fetch returned zero matches).

### REQ-4: TTL = 7 days

An entry whose `fetchedAt` is older than 7×24h MUST be treated as a miss. The stale entry MAY be deleted at the same time.

### REQ-5: LRU Cap = 200 entries

When a new entry is added and the map size would exceed 200, the entry with the oldest `lastAccessed` MUST be evicted before insertion.

### REQ-6: Atomic Disk Writes

Writes MUST be atomic. The implementation MUST write to a temp file in the same directory, `fsync`, then `rename` over the target. A crash mid-write MUST NOT corrupt the existing cache.

### REQ-7: Concurrency

A single `sync.Mutex` MUST guard the in-memory map and the disk-write critical section. The TUI is single-user so no external locking.

### REQ-8: Failure Resilience

The following MUST NOT crash the TUI:
- Cache directory creation fails (permission denied).
- Cache file missing.
- Cache file contains invalid JSON.
- Disk write fails (full disk, permission revoked mid-session).
- Network fetch fails AFTER cache lookup.

In each case, the cache layer MUST log to stderr and fall through to the network path (or, on total disk failure, behave as if the cache is empty).

### REQ-9: Public API

```go
package cache

type Store struct { /* unexported */ }

func New(path string, client *lyrics.Client) (*Store, error)
func (s *Store) Get(title, artist, album string, duration time.Duration) (lyrics.Lyrics, error)
func (s *Store) Clear() error  // drops all entries + truncates file
func (s *Store) Size() int     // current entry count
```

`Store.Get` is the only call site in `ui/shell.go`; it replaces the direct call to `lyrics.Client.FetchLyrics`.

### REQ-10: Wire-In Point

`ui/shell.go::fetchLyricsCmd` MUST build a `*cache.Store` lazily and route `LyricsClient.FetchLyrics(...)` through `store.Get(...)`. The cache file path is resolved once at startup using `os.UserCacheDir() + "/spt-flow/lyrics.json"`.

## Scenarios

### SCN-1: First-time play (cache miss)

Given the cache file does not exist
And `FetchLyrics` would return synced lyrics for the track
When `store.Get(title, artist, album, dur)` is called
Then the network is hit exactly once
And the entry is persisted to disk
And `lastAccessed` is set to the current time
And the lyrics are returned.

### SCN-2: Second play within TTL (cache hit)

Given the cache contains an entry for `(title, artist, album)` fetched < 7 days ago
When `store.Get` is called
Then the network is NOT hit
And the returned lyrics equal the cached lyrics
And `lastAccessed` is updated to now.

### SCN-3: Stale entry (past TTL)

Given the cache contains an entry for `(title, artist, album)` fetched 8 days ago
When `store.Get` is called
Then the network is hit
And the old entry is overwritten
And the on-disk file is rewritten.

### SCN-4: Cap exceeded (LRU eviction)

Given the cache contains 200 entries
When a new `(title, artist, album)` is added
Then the entry with the oldest `lastAccessed` is removed first
And the new entry is added
And the file size stays bounded (~200 entries).

### SCN-5: Corrupt cache file

Given the cache file contains invalid JSON
When the TUI starts
Then a warning is logged to stderr
And the cache is treated as empty (in-memory map is initialized fresh)
And the file is overwritten on the next successful fetch.

### SCN-6: Disk write failure

Given the cache directory becomes read-only mid-session
When `store.Get` triggers a write
Then an error is logged to stderr
And the in-memory state is NOT updated (the fetch result is dropped, not cached)
And subsequent calls behave as if the cache is empty.

### SCN-7: Concurrent fetches

Given two goroutines call `store.Get(same_key, ...)` simultaneously
When both run
Then exactly one network call is made for that key
And both callers receive the same lyrics.

### SCN-8: Clear

When `store.Clear()` is called
Then the file is truncated to zero bytes
And the in-memory map is emptied
And the next call behaves as a cold cache.

### SCN-9: Network failure on miss

Given the cache is empty for the key
And the network fetch fails
When `store.Get` is called
Then the error is returned to the caller (TUI shows the error)
And nothing is written to the cache.