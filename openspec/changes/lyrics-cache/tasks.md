# Tasks: Lyrics Cache

## Phase 1: Package Skeleton

- [ ] 1.1 Create `lyrics/cache/cache.go` with `Entry`, `Store`, `New()` skeleton (compiles, returns empty store).
- [ ] 1.2 Add `key(title, artist, album string) string` helper that computes the SHA-256 hex.
- [ ] 1.3 Add `lyrics.Lyrics` import path check — confirm `lyrics.Lyrics` JSON-marshals correctly (it has no `json:` tags so default field names are used; verify with a quick test).

## Phase 2: Disk I/O

- [ ] 2.1 Implement `(*Store).writeAtomic()` — temp file + fsync + rename.
- [ ] 2.2 Implement `loadFromDisk()` helper called by `New()`.
- [ ] 2.3 Handle missing file (cold start) and corrupt JSON (start fresh + log).

## Phase 3: Get Flow

- [ ] 3.1 Implement `(*Store).Get(title, artist, album, duration)` with hit / miss / stale branches.
- [ ] 3.2 Update `LastAccessed` on hit.
- [ ] 3.3 Delete stale entry on read.
- [ ] 3.4 On miss → call `client.FetchLyrics`, persist on success, swallow write errors (log only).

## Phase 4: LRU Eviction

- [ ] 4.1 Implement `evictIfOver()` — sort by `LastAccessed`, drop oldest until under cap.
- [ ] 4.2 Call it after every successful insert.

## Phase 5: Wire-In

- [ ] 5.1 Add `LyricsCache *cache.Store` to `Model` struct in `ui/shell.go`.
- [ ] 5.2 Update `fetchLyricsCmd` to lazily build a cache store at `~/.cache/spt-flow/lyrics.json`.
- [ ] 5.3 Fall back to direct client when cache init fails.

## Phase 6: Tests

- [ ] 6.1 `cache_test.go::TestStore_Get_ColdStart` — empty cache, network stub returns lyrics, entry persisted.
- [ ] 6.2 `TestStore_Get_CacheHit` — pre-seed cache, second call returns without hitting stub.
- [ ] 6.3 `TestStore_Get_Stale` — pre-seed with `FetchedAt` > TTL, second call re-fetches.
- [ ] 6.4 `TestStore_Get_LRUEviction` — fill to cap, add one more, oldest dropped.
- [ ] 6.5 `TestStore_New_CorruptFile` — write garbage to path, New() recovers, logs, returns empty store.
- [ ] 6.6 `TestStore_New_MissingFile` — no file at path, New() returns empty store without error.
- [ ] 6.7 `TestStore_Get_Key` — same title/artist/album regardless of casing / spacing hits the same key.
- [ ] 6.8 `TestStore_Get_ConcurrentSameKey` — N goroutines hit same key, exactly 1 network call.
- [ ] 6.9 `TestStore_Clear` — truncates file and empties in-memory map.

## Phase 7: Verification

- [ ] 7.1 `go build .` clean.
- [ ] 7.2 `go test ./...` all green (skip KillSpotify subprocess test).
- [ ] 7.3 `go vet ./...` clean.
- [ ] 7.4 `gofmt -d .` no diff.
- [ ] 7.5 Manual: play a track twice; second play should not show the loading message.

## Phase 8: Documentation

- [ ] 8.1 Update `README.md` with a "Lyrics Cache" section: location, TTL, manual delete command.
- [ ] 8.2 Note the cache file in the help overlay (`?`) if there's space.

## Estimate

~300 lines across 4 files. Fits comfortably under the 400-line review budget.