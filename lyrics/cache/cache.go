package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	Title        string        `json:"title"`
	Artist       string        `json:"artist"`
	Album        string        `json:"album"`
	FetchedAt    time.Time     `json:"fetchedAt"`
	LastAccessed time.Time     `json:"lastAccessed"`
	Lyrics       lyrics.Lyrics `json:"lyrics"`
}

type Store struct {
	mu      sync.Mutex
	client  *lyrics.Client
	path    string
	ttl     time.Duration
	cap     int
	entries map[string]Entry
}

func New(path string, client *lyrics.Client) (*Store, error) {
	s := &Store{
		client:  client,
		path:    path,
		ttl:     DefaultTTL,
		cap:     DefaultCap,
		entries: make(map[string]Entry),
	}

	if err := s.loadFromDisk(); err != nil {
		fmt.Fprintf(os.Stderr, "cache: load failed: %v\n", err)
	}

	return s, nil
}

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

	// 3. Persist. Add to map, attempt write, roll back on failure
	// (spec SCN-6: disk failure must drop the fetch result, not leave
	// in-memory state ahead of disk).
	entry := Entry{
		Title:        title,
		Artist:       artist,
		Album:        album,
		FetchedAt:    now,
		LastAccessed: now,
		Lyrics:       l,
	}
	_, isNew := s.entries[k]
	s.entries[k] = entry
	s.evictIfOver()
	if err := s.writeAtomic(); err != nil {
		fmt.Fprintf(os.Stderr, "cache: write failed, dropping entry: %v\n", err)
		// Roll back: remove the new entry. If we updated an existing entry,
		// there's no way to recover the prior state, so we delete to keep
		// memory and disk consistent.
		delete(s.entries, k)
		_ = isNew // marker used only for clarity
	}
	return l, nil
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make(map[string]Entry)

	// Write empty JSON object to file
	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = f.WriteString("{}\n")
	if err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (s *Store) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func key(title, artist, album string) string {
	raw := strings.ToLower(strings.TrimSpace(title)) + "|" +
		strings.ToLower(strings.TrimSpace(artist)) + "|" +
		strings.ToLower(strings.TrimSpace(album))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

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
	for len(s.entries) > s.cap {
		delete(s.entries, sorted[0].k)
		sorted = sorted[1:]
	}
}

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
	defer os.Remove(tmpName)

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

func (s *Store) loadFromDisk() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &s.entries); err != nil {
		fmt.Fprintf(os.Stderr, "cache: corrupt file %q, ignoring: %v\n", s.path, err)
		s.entries = make(map[string]Entry)
		return nil
	}

	// Drop already-stale entries on load.
	now := time.Now()
	for k, e := range s.entries {
		if now.Sub(e.FetchedAt) >= s.ttl {
			delete(s.entries, k)
		}
	}
	return nil
}
