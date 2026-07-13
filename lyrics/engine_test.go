package lyrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseLRC_Synced(t *testing.T) {
	lrcText := `[00:12.34]Line 1
[00:15.50]Line 2
[00:10.05]Line 0 (unsorted)`

	lyrics, synced := ParseLRC(lrcText)
	if !synced {
		t.Fatal("Expected lyrics to be parsed as synced")
	}

	if len(lyrics.Lines) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(lyrics.Lines))
	}

	// Verify sorting
	expectedTimes := []time.Duration{
		10*time.Second + 50*time.Millisecond,
		12*time.Second + 340*time.Millisecond,
		15*time.Second + 500*time.Millisecond,
	}
	expectedContents := []string{
		"Line 0 (unsorted)",
		"Line 1",
		"Line 2",
	}

	for i, line := range lyrics.Lines {
		if line.Timestamp != expectedTimes[i] {
			t.Errorf("Line %d: expected timestamp %v, got %v", i, expectedTimes[i], line.Timestamp)
		}
		if line.Content != expectedContents[i] {
			t.Errorf("Line %d: expected content %q, got %q", i, expectedContents[i], line.Content)
		}
	}
}

func TestParseLRC_MultipleTimestamps(t *testing.T) {
	lrcText := `[00:01.00][00:02.00]Repeated line`
	lyrics, synced := ParseLRC(lrcText)
	if !synced {
		t.Fatal("Expected synced to be true")
	}
	if len(lyrics.Lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lyrics.Lines))
	}
	if lyrics.Lines[0].Timestamp != 1*time.Second || lyrics.Lines[1].Timestamp != 2*time.Second {
		t.Errorf("Expected timestamps 1s and 2s, got %v and %v", lyrics.Lines[0].Timestamp, lyrics.Lines[1].Timestamp)
	}
	if lyrics.Lines[0].Content != "Repeated line" || lyrics.Lines[1].Content != "Repeated line" {
		t.Errorf("Expected contents 'Repeated line', got %q and %q", lyrics.Lines[0].Content, lyrics.Lines[1].Content)
	}
}

func TestParseLRC_PlainFallback(t *testing.T) {
	plainText := `This is a song
Without any timestamps
Just plain text`
	lyrics, synced := ParseLRC(plainText)
	if synced {
		t.Fatal("Expected synced to be false for plain text")
	}
	if len(lyrics.Lines) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(lyrics.Lines))
	}
	for i, line := range lyrics.Lines {
		if line.Timestamp != 0 {
			t.Errorf("Line %d: expected timestamp 0, got %v", i, line.Timestamp)
		}
	}
	expected := []string{
		"This is a song",
		"Without any timestamps",
		"Just plain text",
	}
	for i, line := range lyrics.Lines {
		if line.Content != expected[i] {
			t.Errorf("Line %d: expected %q, got %q", i, expected[i], line.Content)
		}
	}
}

func TestParseLRC_MetadataTags(t *testing.T) {
	lrcText := `[ti:Test Title]
[ar:Test Artist]
[00:05.10]First line
[00:08.20]Second line`
	lyrics, synced := ParseLRC(lrcText)
	if !synced {
		t.Fatal("Expected synced to be true")
	}
	// Metadata tags should not be captured in the lines if we only match timestamps.
	// So we should have exactly 2 lines.
	if len(lyrics.Lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lyrics.Lines))
	}
	if lyrics.Lines[0].Content != "First line" || lyrics.Lines[1].Content != "Second line" {
		t.Errorf("Expected contents 'First line' and 'Second line', got %q and %q", lyrics.Lines[0].Content, lyrics.Lines[1].Content)
	}
}

func TestParseLRC_NoMs(t *testing.T) {
	lrcText := `[01:05]Line without milliseconds`
	lyrics, synced := ParseLRC(lrcText)
	if !synced {
		t.Fatal("Expected synced to be true")
	}
	if len(lyrics.Lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lyrics.Lines))
	}
	expectedDur := 1*time.Minute + 5*time.Second
	if lyrics.Lines[0].Timestamp != expectedDur {
		t.Errorf("Expected timestamp %v, got %v", expectedDur, lyrics.Lines[0].Timestamp)
	}
}

func TestFetchLyrics_SuccessGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/get" {
			if r.URL.Query().Get("track_name") != "Song" || r.URL.Query().Get("artist_name") != "Artist" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"id": 1234,
				"name": "Song",
				"artistName": "Artist",
				"albumName": "Album",
				"duration": 180,
				"syncedLyrics": "[00:10.00]Hello world\n[00:15.00]Goodbye world",
				"plainLyrics": "Hello world\nGoodbye world"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL)
	lyrics, err := client.FetchLyrics("Song", "Artist", "Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !lyrics.Synced {
		t.Fatal("Expected lyrics to be synced")
	}
	if len(lyrics.Lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lyrics.Lines))
	}
	if lyrics.Lines[0].Content != "Hello world" {
		t.Errorf("Expected line 0 Content 'Hello world', got %q", lyrics.Lines[0].Content)
	}
}

func TestFetchLyrics_FallbackToSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/get" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api/search" {
			q := r.URL.Query().Get("q")
			if q != "Artist Song" && q != "Song Artist" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[
				{
					"id": 5678,
					"name": "Song",
					"artistName": "Artist",
					"albumName": "Album",
					"duration": 182,
					"syncedLyrics": "[00:12.00]Search synced line",
					"plainLyrics": "Search plain line"
				}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL)
	lyrics, err := client.FetchLyrics("Song", "Artist", "Album", 180*time.Second)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !lyrics.Synced {
		t.Fatal("Expected lyrics to be synced")
	}
	if len(lyrics.Lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lyrics.Lines))
	}
	if lyrics.Lines[0].Content != "Search synced line" {
		t.Errorf("Expected Content 'Search synced line', got %q", lyrics.Lines[0].Content)
	}
}


