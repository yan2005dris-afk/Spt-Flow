# Proposal: Lyrics Cache

## Intent

Every time a track plays, `lyrics.Client.FetchLyrics` makes a network call to lrclib.net. Users with a stable music library hit the same endpoint for the same songs repeatedly — wasted bandwidth, slower UI transitions (the loading state lingers while the request round-trips), and a dependency on a third-party API being up.

This change adds a **local on-disk cache** layer that sits in front of `FetchLyrics`. The first time a song is requested we go to the network and write the result to `~/.cache/spt-flow/lyrics.json`. Subsequent requests for the same song (same title+artist+album) read from disk and skip the network entirely.

The user-visible promise: the first time you play a track, the lyrics take the usual 1-3 seconds. The next time you play that track — same album or a re-listen later in the week — they appear instantly.

## Scope

### In Scope

- New package `lyrics/cache` with a `Store` type that wraps the existing `Client`.
- Cache file at `~/.cache/spt-flow/lyrics.json` (XDG cache home).
- Cache key: SHA-256 hex of `title|artist|album` (lowercase, trimmed). Duration is intentionally **not** part of the key — duration can vary across releases of the same recording, and we want the cache to hit even when the user has different versions.
- TTL: **7 days** from the time of the successful fetch. After 7 days the entry is considered stale and we re-fetch.
- LRU eviction: cap the file at **200 entries**; when adding a new entry would exceed the cap, drop the entry with the oldest `lastAccessed` timestamp.
- Concurrency: a single `sync.Mutex` guards in-memory map + file writes. No external locking needed (single-user TUI).
- Failure modes (any of these must NOT crash the TUI):
  - Cache directory doesn't exist → create it.
  - Cache file is missing or corrupt → start fresh.
  - Disk full / permission denied → log to stderr, fall through to network fetch.
  - JSON unmarshal error → log, drop corrupt entries, start fresh.
- `ui/shell.go` change: `fetchLyricsCmd` builds a `*cache.Store` and routes through it. Cache file path is `~/.cache/spt-flow/lyrics.json` resolved at startup.
- Tests: `lyrics/cache/cache_test.go` covering the cache hit / miss / eviction / TTL / corruption-recovery / file-IO failure paths. Tests must NOT touch the real network — they use a stub HTTP server or skip the network path entirely.

### Out of Scope

- **Synced lyrics delta sync** — the cache stores the full response, not diffs.
- **Multiple cache backends** (BoltDB, SQLite, Redis). v1 is plain JSON for simplicity; a future change can swap the storage layer.
- **Persistent display of cache status** — no UI indicator showing "served from cache". v1 is invisible to the user.
- **Per-track metadata version tracking** — we don't bump versions when lrclib.net changes its API.
- **Manual cache invalidation command** — a future menu option can add `Clear lyrics cache`; not part of v1.
- **Sharing cache across users** — single-user file in `~/.cache`; multi-user goes to a system path later.

## Approach

Layer the cache **in front of** the existing `lyrics.Client`. Don't modify `FetchLyrics` itself — wrap it.

```go
type Store struct {
    client *Client          // network fallback
    mu     sync.Mutex
    path   string           // ~/.cache/spt-flow/lyrics.json
    ttl    time.Duration    // 7 days
    cap    int              // 200 entries
    entries map[string]Entry
}
```

The flow:
1. `Store.Get(title, artist, album, duration)` is the public API.
2. Compute SHA-256 key from `title|artist|album`.
3. Acquire mutex; check `entries[key]`.
4. **Cache hit + fresh** (within TTL): update `LastAccessed`, return cached lyrics.
5. **Cache hit + stale** (past TTL): delete entry, fall through to network.
6. **Cache miss**: call `client.FetchLyrics(...)`, store result on success, return.
7. Persist to disk after every successful network fetch (atomic write: tmp file + rename).

Atomic write is required because the TUI could be killed mid-write — a partial JSON file would corrupt the cache and we'd lose everything on next load.

The mutex is held during the disk write so concurrent fetches for the same key don't both hit the network. The whole operation is short enough (a few ms on modern disks) that blocking the TUI is fine.