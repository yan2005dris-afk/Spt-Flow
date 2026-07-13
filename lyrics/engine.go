package lyrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LyricsLine struct {
	Timestamp time.Duration
	Content   string
}

type Lyrics struct {
	Lines  []LyricsLine
	Synced bool
}

var lrcRegex = regexp.MustCompile(`\[(\d{2,}):(\d{2})(?:\.(\d{2,3}))?\]`)

// ParseLRC parses an LRC lyrics string. If synced lyrics are found, it returns
// the lines sorted by timestamp with synced=true. If no synced lines are found,
// it treats each non-empty line as plain text (timestamp=0) and returns synced=false.
func ParseLRC(lrcText string) (Lyrics, bool) {
	var lines []LyricsLine
	var rawLines []string
	scanner := bufio.NewScanner(strings.NewReader(lrcText))

	hasSynced := false
	for scanner.Scan() {
		line := scanner.Text()
		rawLines = append(rawLines, line)

		matches := lrcRegex.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}

		hasSynced = true
		contentIdx := 0
		for _, m := range matches {
			contentIdx += len(m[0])
		}
		content := strings.TrimSpace(line[contentIdx:])

		for _, m := range matches {
			min, _ := strconv.Atoi(m[1])
			sec, _ := strconv.Atoi(m[2])
			msVal := 0
			if len(m) > 3 && m[3] != "" {
				msStr := m[3]
				if len(msStr) == 2 {
					msStr += "0"
				} else if len(msStr) == 1 {
					msStr += "00"
				}
				msVal, _ = strconv.Atoi(msStr)
			}
			dur := time.Duration(min)*time.Minute + time.Duration(sec)*time.Second + time.Duration(msVal)*time.Millisecond
			lines = append(lines, LyricsLine{Timestamp: dur, Content: content})
		}
	}

	if !hasSynced {
		// Plain text fallback
		lines = nil
		for _, line := range rawLines {
			trimmed := strings.TrimSpace(line)
			lines = append(lines, LyricsLine{Timestamp: 0, Content: trimmed})
		}
		return Lyrics{Lines: lines, Synced: false}, false
	}

	sort.Slice(lines, func(i, j int) bool { return lines[i].Timestamp < lines[j].Timestamp })
	return Lyrics{Lines: lines, Synced: true}, true
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://lrclib.net"
	}
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type TrackResponse struct {
	SyncedLyrics string  `json:"syncedLyrics"`
	PlainLyrics  string  `json:"plainLyrics"`
	Duration     float64 `json:"duration"`
}

func (c *Client) FetchLyrics(title, artist, album string, duration time.Duration) (Lyrics, error) {
	// 1. Try exact lookup via /api/get
	u, err := url.Parse(c.BaseURL + "/api/get")
	if err != nil {
		return Lyrics{}, err
	}
	q := u.Query()
	q.Set("track_name", title)
	q.Set("artist_name", artist)
	if album != "" {
		q.Set("album_name", album)
	}
	durSeconds := int(duration.Seconds())
	if durSeconds > 0 {
		q.Set("duration", strconv.Itoa(durSeconds))
	}
	u.RawQuery = q.Encode()

	resp, err := c.HTTPClient.Get(u.String())
	if err == nil && resp.StatusCode == http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		var track TrackResponse
		if err := json.NewDecoder(resp.Body).Decode(&track); err == nil {
			return c.processResponse(track)
		}
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	// 2. Fall back to search via /api/search
	searchURL, err := url.Parse(c.BaseURL + "/api/search")
	if err != nil {
		return Lyrics{}, err
	}
	sq := searchURL.Query()
	sq.Set("q", fmt.Sprintf("%s %s", artist, title))
	searchURL.RawQuery = sq.Encode()

	sresp, err := c.HTTPClient.Get(searchURL.String())
	if err != nil {
		return Lyrics{}, err
	}
	defer func() { _ = sresp.Body.Close() }()

	if sresp.StatusCode != http.StatusOK {
		return Lyrics{}, fmt.Errorf("lyrics not found (status %d)", sresp.StatusCode)
	}

	var searchResults []TrackResponse
	if err := json.NewDecoder(sresp.Body).Decode(&searchResults); err != nil {
		return Lyrics{}, err
	}

	if len(searchResults) == 0 {
		return Lyrics{}, fmt.Errorf("no lyrics found in search results")
	}

	// Pick the track with the closest duration
	targetSec := duration.Seconds()
	bestIdx := 0
	minDiff := math.MaxFloat64
	for i, t := range searchResults {
		diff := math.Abs(t.Duration - targetSec)
		if diff < minDiff {
			minDiff = diff
			bestIdx = i
		}
	}

	return c.processResponse(searchResults[bestIdx])
}

func (c *Client) processResponse(track TrackResponse) (Lyrics, error) {
	if track.SyncedLyrics != "" {
		lyrics, synced := ParseLRC(track.SyncedLyrics)
		if synced {
			return lyrics, nil
		}
	}
	if track.PlainLyrics != "" {
		lyrics, _ := ParseLRC(track.PlainLyrics)
		return lyrics, nil
	}
	return Lyrics{}, fmt.Errorf("track has empty lyrics")
}
