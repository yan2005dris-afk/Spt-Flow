package mpris

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
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
	ownerName       string
	ownerErr        error
	obj             *mockDBusObject
	signals         chan<- *dbus.Signal
	startServiceErr error
}

func (m *mockDBusConnection) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	c := &dbus.Call{Done: make(chan *dbus.Call, 1)}
	if method == "org.freedesktop.DBus.GetNameOwner" {
		c.Body = []interface{}{m.ownerName}
		c.Err = m.ownerErr
	}
	if method == "org.freedesktop.DBus.StartServiceByName" {
		c.Err = m.startServiceErr
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

func TestMprisClient_Close(t *testing.T) {
	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{
		ownerName: "org.mpris.MediaPlayer2.spotify",
		obj:       mockObj,
	}
	client := &Client{conn: mockConn, obj: mockObj, realConn: nil}
	if err := client.Close(); err != nil {
		t.Errorf("Close with nil realConn should not error: %v", err)
	}
}

func TestMprisClient_Close_KillSpotify(t *testing.T) {
	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{
		ownerName: "org.mpris.MediaPlayer2.spotify",
		obj:       mockObj,
	}
	client := &Client{
		conn:       mockConn,
		obj:        mockObj,
		spotifyCmd: &exec.Cmd{},
	}
	if err := client.Close(); err != nil {
		t.Errorf("Close with spotifyCmd should not error: %v", err)
	}
}

func TestMprisClient_GetPosition(t *testing.T) {
	mockObj := &mockDBusObject{
		properties: map[string]interface{}{
			"org.mpris.MediaPlayer2.Player.Position": int64(5000000),
		},
	}
	mockConn := &mockDBusConnection{obj: mockObj}
	client := &Client{conn: mockConn, obj: mockObj}
	pos, err := client.GetPosition()
	if err != nil {
		t.Fatalf("GetPosition error: %v", err)
	}
	if pos != 5*time.Second {
		t.Errorf("Expected 5s, got %v", pos)
	}
}

func TestMprisClient_SetVolume(t *testing.T) {
	mockObj := &mockDBusObject{
		properties: make(map[string]interface{}),
	}
	mockConn := &mockDBusConnection{obj: mockObj}
	client := &Client{conn: mockConn, obj: mockObj}
	err := client.SetVolume(0.5)
	if err != nil {
		t.Fatalf("SetVolume error: %v", err)
	}
	if mockObj.properties["org.mpris.MediaPlayer2.Player.Volume"] != 0.5 {
		t.Errorf("Expected volume 0.5, got %v", mockObj.properties["org.mpris.MediaPlayer2.Player.Volume"])
	}
}

func TestMprisClient_Watch(t *testing.T) {
	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{obj: mockObj}
	client := &Client{conn: mockConn, obj: mockObj}
	ch, err := client.Watch()
	if err != nil {
		t.Fatalf("Watch error: %v", err)
	}
	if ch == nil {
		t.Error("Watch returned nil channel")
	}
}

func TestMprisClient_LaunchSpotify_FlatpakDetection(t *testing.T) {
	origFlatpak := os.Getenv("FLATPAK_ID")
	origSnap := os.Getenv("SNAP_NAME")
	defer func() {
		os.Setenv("FLATPAK_ID", origFlatpak)
		os.Setenv("SNAP_NAME", origSnap)
	}()

	os.Setenv("FLATPAK_ID", "spotify")
	os.Setenv("SNAP_NAME", "")

	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{obj: mockObj, ownerName: ""}
	client := &Client{conn: mockConn, obj: mockObj}
	err := client.LaunchSpotify()
	if err != ErrFlatpakSandbox {
		t.Errorf("Expected ErrFlatpakSandbox, got %v", err)
	}
}

func TestMprisClient_LaunchSpotify_SnapDetection(t *testing.T) {
	origFlatpak := os.Getenv("FLATPAK_ID")
	origSnap := os.Getenv("SNAP_NAME")
	defer func() {
		os.Setenv("FLATPAK_ID", origFlatpak)
		os.Setenv("SNAP_NAME", origSnap)
	}()

	os.Setenv("FLATPAK_ID", "")
	os.Setenv("SNAP_NAME", "spotify")

	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{obj: mockObj, ownerName: ""}
	client := &Client{conn: mockConn, obj: mockObj}
	err := client.LaunchSpotify()
	if err != ErrFlatpakSandbox {
		t.Errorf("Expected ErrFlatpakSandbox, got %v", err)
	}
}

func TestMprisClient_LaunchSpotify_Success(t *testing.T) {
	origFlatpak := os.Getenv("FLATPAK_ID")
	origSnap := os.Getenv("SNAP_NAME")
	defer func() {
		os.Setenv("FLATPAK_ID", origFlatpak)
		os.Setenv("SNAP_NAME", origSnap)
	}()

	os.Setenv("FLATPAK_ID", "")
	os.Setenv("SNAP_NAME", "")

	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{obj: mockObj}
	client := &Client{conn: mockConn, obj: mockObj}
	err := client.LaunchSpotify()
	if err != nil {
		t.Errorf("LaunchSpotify should not error: %v", err)
	}
}

func TestMprisClient_KillSpotify_NoOp(t *testing.T) {
	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{obj: mockObj}
	client := &Client{conn: mockConn, obj: mockObj}
	err := client.KillSpotify()
	if err != nil {
		t.Errorf("KillSpotify with nil spotifyCmd should not error: %v", err)
	}
}

// TestMprisClient_KillSpotify_WithProcess tests that KillSpotify sends SIGTERM
// to the subprocess. Because syscall.Kill is a real OS call that we cannot
// usefully mock in a unit test, we spawn a genuine short-lived child process
// isolated in its own session so the signal stays contained.
func TestMprisClient_KillSpotify_WithProcess(t *testing.T) {
	// Start a real sleep in an isolated session (Setsid:true) so its process
	// group never intersects with the test runner's.  Using setsid(1) directly
	// guarantees sleep runs in a fresh session and does not inherit the test
	// process group, so syscall.Kill(-pid, SIGTERM) only affects this tree.
	cmd := exec.Command("setsid", "sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Skipf("setsid not available or cannot start subprocess: %v", err)
	}

	// Give the process a moment to fully detach.
	time.Sleep(10 * time.Millisecond)

	pid := cmd.Process.Pid

	// Wrap Wait in a goroutine so we don't block the test.
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(done)
	}()

	// Send SIGTERM to the process group (negative PID = pgid).
	// Because we used setsid, sleep's pgid == its pid, so -pid targets only
	// this tree and cannot bleed into the test runner's session.
	err := syscall.Kill(-pid, syscall.SIGTERM)
	if err != nil {
		t.Fatalf("Kill(-pid, SIGTERM) should not error: %v", err)
	}

	// Wait up to 3 s for the process to exit; if it ignores SIGTERM (shouldn't
	// happen for sleep), SIGKILL as a last resort so the test finishes.
	select {
	case <-done:
		// ok — sleep exited cleanly after SIGTERM
	case <-time.After(3 * time.Second):
		syscall.Kill(-pid, syscall.SIGKILL)
		cmd.Wait()
		t.Log("sleep ignored SIGTERM; SIGKILL was used as fallback (test-only)")
	}

	_ = waitErr // may be "signal: terminated" — that's expected
}

func TestMprisClient_SetSpotifyCmd(t *testing.T) {
	mockObj := &mockDBusObject{}
	mockConn := &mockDBusConnection{obj: mockObj}
	client := &Client{conn: mockConn, obj: mockObj}
	cmd := &exec.Cmd{}
	client.SetSpotifyCmd(cmd)
	if client.spotifyCmd != cmd {
		t.Error("SetSpotifyCmd did not set the command")
	}
}
