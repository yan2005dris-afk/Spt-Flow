package mpris

import (
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// Mock DBusObject
type mockDBusObject struct {
	properties map[string]interface{}
	calls      []string
}

func (m *mockDBusObject) GetProperty(p string) (dbus.Variant, error) {
	val, ok := m.properties[p]
	if !ok {
		return dbus.Variant{}, errors.New("property not found")
	}
	return dbus.MakeVariant(val), nil
}

func (m *mockDBusObject) SetProperty(p string, v dbus.Variant) error {
	m.properties[p] = v.Value()
	return nil
}

func (m *mockDBusObject) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	m.calls = append(m.calls, method)
	return &dbus.Call{Done: make(chan *dbus.Call, 1)}
}

// Mock DBusConnection
type mockDBusConnection struct {
	ownerName string
	ownerErr  error
	obj       *mockDBusObject
	signals   chan<- *dbus.Signal
}

func (m *mockDBusConnection) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	c := &dbus.Call{Done: make(chan *dbus.Call, 1)}
	if method == "org.freedesktop.DBus.GetNameOwner" {
		c.Body = []interface{}{m.ownerName}
		c.Err = m.ownerErr
	}
	return c
}

func (m *mockDBusConnection) Object(dest string, path dbus.ObjectPath) DBusObject {
	return m.obj
}

func (m *mockDBusConnection) AddMatchSignal(options ...dbus.MatchOption) error {
	return nil
}

func (m *mockDBusConnection) Signal(ch chan<- *dbus.Signal) {
	m.signals = ch
}

func TestMprisClient_IsRunning(t *testing.T) {
	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{
		ownerName: "org.mpris.MediaPlayer2.spotify",
		obj:       mockObj,
	}

	client := &Client{conn: mockConn, obj: mockObj}
	if !client.IsRunning() {
		t.Error("Expected IsRunning to return true when owner exists")
	}

	mockConn.ownerName = ""
	mockConn.ownerErr = errors.New("no owner")
	if client.IsRunning() {
		t.Error("Expected IsRunning to return false when owner does not exist")
	}
}

func TestMprisClient_GetTrackAndState(t *testing.T) {
	metadata := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant("spotify:track:abcdef"),
		"mpris:length":  dbus.MakeVariant(int64(180000000)), // 180 seconds in microseconds
		"xesam:title":   dbus.MakeVariant("Song Title"),
		"xesam:artist":  dbus.MakeVariant([]string{"Artist One", "Artist Two"}),
		"xesam:album":   dbus.MakeVariant("Album Name"),
	}

	mockObj := &mockDBusObject{
		properties: map[string]interface{}{
			"org.mpris.MediaPlayer2.Player.PlaybackStatus": "Playing",
			"org.mpris.MediaPlayer2.Player.Metadata":       metadata,
			"org.mpris.MediaPlayer2.Player.Volume":         0.8,
		},
	}
	mockConn := &mockDBusConnection{
		ownerName: "org.mpris.MediaPlayer2.spotify",
		obj:       mockObj,
	}

	client := &Client{conn: mockConn, obj: mockObj}
	track, err := client.GetTrack()
	if err != nil {
		t.Fatalf("Unexpected error getting track: %v", err)
	}

	if track.ID != "spotify:track:abcdef" {
		t.Errorf("Expected ID 'spotify:track:abcdef', got %q", track.ID)
	}
	if track.Title != "Song Title" {
		t.Errorf("Expected Title 'Song Title', got %q", track.Title)
	}
	if track.Artist != "Artist One, Artist Two" {
		t.Errorf("Expected Artist 'Artist One, Artist Two', got %q", track.Artist)
	}
	if track.Album != "Album Name" {
		t.Errorf("Expected Album 'Album Name', got %q", track.Album)
	}
	if track.Duration != 180*time.Second {
		t.Errorf("Expected Duration 180s, got %v", track.Duration)
	}

	status, err := client.GetPlaybackStatus()
	if err != nil {
		t.Fatalf("Unexpected error getting playback status: %v", err)
	}
	if status != "Playing" {
		t.Errorf("Expected status 'Playing', got %q", status)
	}

	volume, err := client.GetVolume()
	if err != nil {
		t.Fatalf("Unexpected error getting volume: %v", err)
	}
	if volume != 0.8 {
		t.Errorf("Expected volume 0.8, got %v", volume)
	}
}

func TestMprisClient_PlaybackCommands(t *testing.T) {
	mockObj := &mockDBusObject{
		properties: make(map[string]interface{}),
	}
	mockConn := &mockDBusConnection{
		ownerName: "org.mpris.MediaPlayer2.spotify",
		obj:       mockObj,
	}

	client := &Client{conn: mockConn, obj: mockObj}

	_ = client.PlayPause()
	_ = client.Next()
	_ = client.Previous()

	expectedCalls := []string{
		"org.mpris.MediaPlayer2.Player.PlayPause",
		"org.mpris.MediaPlayer2.Player.Next",
		"org.mpris.MediaPlayer2.Player.Previous",
	}

	if len(mockObj.calls) != len(expectedCalls) {
		t.Fatalf("Expected %d calls, got %d", len(expectedCalls), len(mockObj.calls))
	}

	for i, call := range mockObj.calls {
		if call != expectedCalls[i] {
			t.Errorf("Call %d: expected %q, got %q", i, expectedCalls[i], call)
		}
	}
}
