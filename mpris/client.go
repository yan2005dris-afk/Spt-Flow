package mpris

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

var ErrFlatpakSandbox = errors.New(
	"spotify runs in flatpak/snap sandbox; not compatible with this TUI",
)

var ErrLibrespotNotInstalled = errors.New("librespot not found in PATH")

var ErrSpotifyDesktopRunning = errors.New(
	"spotify desktop is running — please close it first",
)

type Track struct {
	ID       string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
}

type DBusConnection interface {
	Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call
	Object(dest string, path dbus.ObjectPath) DBusObject
	AddMatchSignal(options ...dbus.MatchOption) error
	Signal(ch chan<- *dbus.Signal)
}

type DBusObject interface {
	Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call
	GetProperty(p string) (dbus.Variant, error)
	SetProperty(p string, v dbus.Variant) error
}

type realDBusObject struct {
	obj dbus.BusObject
}

func (o *realDBusObject) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	return o.obj.Call(method, flags, args...)
}

func (o *realDBusObject) GetProperty(p string) (dbus.Variant, error) {
	return o.obj.GetProperty(p)
}

func (o *realDBusObject) SetProperty(p string, v dbus.Variant) error {
	return o.obj.SetProperty(p, v)
}

type realDBusConnection struct {
	conn *dbus.Conn
}

func (c *realDBusConnection) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	return c.conn.BusObject().Call(method, flags, args...)
}

func (c *realDBusConnection) Object(dest string, path dbus.ObjectPath) DBusObject {
	return &realDBusObject{obj: c.conn.Object(dest, path)}
}

func (c *realDBusConnection) AddMatchSignal(options ...dbus.MatchOption) error {
	return c.conn.AddMatchSignal(options...)
}

func (c *realDBusConnection) Signal(ch chan<- *dbus.Signal) {
	c.conn.Signal(ch)
}

type Client struct {
	conn       DBusConnection
	obj        DBusObject
	realConn   *dbus.Conn
	spotifyCmd *exec.Cmd // nil when TUI did not launch the process
}

func NewClient() (*Client, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	adapterConn := &realDBusConnection{conn: conn}
	obj := adapterConn.Object("org.mpris.MediaPlayer2.spotify", "/org/mpris/MediaPlayer2")
	return &Client{
		conn:       adapterConn,
		obj:        obj,
		realConn:   conn,
		spotifyCmd: nil,
	}, nil
}

func (c *Client) Close() error {
	// Always clean up the subprocess handle first so we don't leak a
	// Spotify that the TUI started but couldn't kill on exit.
	if c.spotifyCmd != nil {
		_ = c.KillSpotify()
	}
	if c.realConn != nil {
		return c.realConn.Close()
	}
	return nil
}

func (c *Client) IsRunning() bool {
	var owner string
	err := c.conn.Call("org.freedesktop.DBus.GetNameOwner", 0, "org.mpris.MediaPlayer2.spotify").Store(&owner)
	return err == nil && owner != ""
}

func (c *Client) GetPlaybackStatus() (string, error) {
	v, err := c.obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
	if err != nil {
		return "", err
	}
	status, ok := v.Value().(string)
	if !ok {
		return "", fmt.Errorf("unexpected status type: %T", v.Value())
	}
	return status, nil
}

func (c *Client) GetVolume() (float64, error) {
	v, err := c.obj.GetProperty("org.mpris.MediaPlayer2.Player.Volume")
	if err != nil {
		return 0, err
	}
	vol, ok := v.Value().(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected volume type: %T", v.Value())
	}
	return vol, nil
}

func (c *Client) GetPosition() (time.Duration, error) {
	v, err := c.obj.GetProperty("org.mpris.MediaPlayer2.Player.Position")
	if err != nil {
		return 0, err
	}
	// Position can be returned as int64 representing microseconds
	posVal, ok := v.Value().(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected position type: %T", v.Value())
	}
	return time.Duration(posVal) * time.Microsecond, nil
}

func (c *Client) GetTrack() (Track, error) {
	v, err := c.obj.GetProperty("org.mpris.MediaPlayer2.Player.Metadata")
	if err != nil {
		return Track{}, err
	}
	metaMap, ok := v.Value().(map[string]dbus.Variant)
	if !ok {
		return Track{}, fmt.Errorf("unexpected metadata type: %T", v.Value())
	}

	var track Track
	if idVar, ok := metaMap["mpris:trackid"]; ok {
		track.ID, _ = idVar.Value().(string)
	}
	if titleVar, ok := metaMap["xesam:title"]; ok {
		track.Title, _ = titleVar.Value().(string)
	}
	if albumVar, ok := metaMap["xesam:album"]; ok {
		track.Album, _ = albumVar.Value().(string)
	}
	if artistVar, ok := metaMap["xesam:artist"]; ok {
		switch artists := artistVar.Value().(type) {
		case []string:
			track.Artist = strings.Join(artists, ", ")
		case string:
			track.Artist = artists
		}
	}
	if lengthVar, ok := metaMap["mpris:length"]; ok {
		switch lenVal := lengthVar.Value().(type) {
		case int64:
			track.Duration = time.Duration(lenVal) * time.Microsecond
		case uint64:
			track.Duration = time.Duration(lenVal) * time.Microsecond
		}
	}

	return track, nil
}

func (c *Client) PlayPause() error {
	return c.obj.Call("org.mpris.MediaPlayer2.Player.PlayPause", 0).Err
}

func (c *Client) Next() error {
	return c.obj.Call("org.mpris.MediaPlayer2.Player.Next", 0).Err
}

func (c *Client) Previous() error {
	return c.obj.Call("org.mpris.MediaPlayer2.Player.Previous", 0).Err
}

func (c *Client) SetVolume(volume float64) error {
	return c.obj.SetProperty("org.mpris.MediaPlayer2.Player.Volume", dbus.MakeVariant(volume))
}

func (c *Client) Watch() (chan *dbus.Signal, error) {
	err := c.conn.AddMatchSignal(
		dbus.WithMatchSender("org.mpris.MediaPlayer2.spotify"),
		dbus.WithMatchObjectPath("/org/mpris/MediaPlayer2"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	)
	if err != nil {
		return nil, err
	}
	ch := make(chan *dbus.Signal, 100)
	c.conn.Signal(ch)
	return ch, nil
}

// LaunchSpotify asks D-Bus to activate the Spotify MPRIS service. Returns
// ErrFlatpakSandbox when the running Spotify is sandboxed and unreachable
// from the user's session bus; the caller MUST stop polling on this error.
func (c *Client) LaunchSpotify() error {
	if os.Getenv("FLATPAK_ID") != "" && !c.IsRunning() {
		return ErrFlatpakSandbox
	}
	if os.Getenv("SNAP_NAME") != "" && !c.IsRunning() {
		return ErrFlatpakSandbox
	}
	call := c.conn.Call(
		"org.freedesktop.DBus.StartServiceByName",
		0,
		"org.mpris.MediaPlayer2.spotify",
		uint32(0),
	)
	return call.Err
}

// KillSpotify sends SIGTERM to the TUI-launched Spotify process group.
// Negative PID addresses the whole pgid (requires Setpgid: true on Cmd).
// Safe to call when spotifyCmd is nil — no-op.
func (c *Client) KillSpotify() error {
	if c.spotifyCmd == nil || c.spotifyCmd.Process == nil {
		return nil
	}
	return syscall.Kill(-c.spotifyCmd.Process.Pid, syscall.SIGTERM)
}

// SetSpotifyCmd stores the exec.Cmd for a Spotify process started by main.go.
// This allows KillSpotify to terminate the process group on shutdown.
func (c *Client) SetSpotifyCmd(cmd *exec.Cmd) {
	c.spotifyCmd = cmd
}

// ConfigPath returns the default librespot config path.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "tui-spotify", "librespot.conf")
}

// CacheDir returns the default librespot cache directory.
func CacheDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "librespot")
}

// IsSpotifyDesktopRunning reports whether org.mpris.MediaPlayer2.spotify
// is owned by Spotify desktop (not librespot). This is only safe to call
// BEFORE launching librespot.
func (c *Client) IsSpotifyDesktopRunning() bool {
	var owner string
	err := c.conn.Call("org.freedesktop.DBus.GetNameOwner", 0, "org.mpris.MediaPlayer2.spotify").Store(&owner)
	return err == nil && owner != ""
}

// LaunchLibrespot starts the librespot binary with OAuth and returns the
// cmd so the caller can store it via SetSpotifyCmd. cfg may be empty for
// the user-default ~/.config/librespot.conf. Returns ErrLibrespotNotInstalled
// or ErrSpotifyDesktopRunning on the documented conditions.
func (c *Client) LaunchLibrespot(cfg string) error {
	path, err := exec.LookPath("librespot")
	if err != nil {
		return ErrLibrespotNotInstalled
	}
	if c.IsSpotifyDesktopRunning() {
		return ErrSpotifyDesktopRunning
	}

	cache := CacheDir()
	args := []string{
		"--name", "Spt-Flow",
		"--enable-oauth",
		"--cache", cache,
	}
	if cfg != "" {
		args = append(args, "--config", cfg)
	}

	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start librespot: %w", err)
	}
	c.SetSpotifyCmd(cmd)
	time.Sleep(100 * time.Millisecond)
	return nil
}
