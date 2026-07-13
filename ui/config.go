package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const configFileName = "config.json"

// Config holds user-visible settings.
type Config struct {
	Theme string `json:"theme"`
	raw   map[string]json.RawMessage
}

// LoadConfig reads the config file from XDG_CONFIG_HOME/spt-flow/config.json
// (or ~/.config/spt-flow/config.json as fallback). Missing, corrupt, or
// unknown-theme files return a default Config{Theme:"default"}; no error is
// surfaced to the user.
func LoadConfig() (*Config, error) {
	fallback := &Config{Theme: "default", raw: make(map[string]json.RawMessage)}

	base, err := os.UserConfigDir()
	if err != nil {
		return fallback, nil
	}
	path := filepath.Join(base, "spt-flow", configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		fmt.Fprintf(os.Stderr, "config: read failed: %v\n", err)
		return fallback, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "config: corrupt file, using default: %v\n", err)
		return fallback, nil
	}
	if raw == nil {
		fmt.Fprintln(os.Stderr, "config: corrupt file, using default: expected JSON object")
		return fallback, nil
	}

	theme := "default"
	if rawTheme, ok := raw["theme"]; ok {
		if err := json.Unmarshal(rawTheme, &theme); err != nil {
			fmt.Fprintf(os.Stderr, "config: corrupt theme, using default: %v\n", err)
			return fallback, nil
		}
	}

	if _, ok := Themes[theme]; !ok {
		fmt.Fprintf(os.Stderr, "config: unknown theme %q, using default\n", theme)
		theme = "default"
	}

	return &Config{Theme: theme, raw: raw}, nil
}

// Save writes Config atomically to XDG_CONFIG_HOME/spt-flow/config.json
// (or ~/.config/spt-flow/config.json). Errors are logged to stderr.
func (c *Config) Save() error {
	base, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot determine config dir: %v\n", err)
		return err
	}
	dir := filepath.Join(base, "spt-flow")
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot create config dir: %v\n", err)
		return err
	}
	path := filepath.Join(dir, configFileName)

	raw := make(map[string]json.RawMessage, len(c.raw)+1)
	for key, value := range c.raw {
		raw[key] = value
	}
	theme, err := json.Marshal(c.Theme)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot encode theme: %v\n", err)
		return err
	}
	raw["theme"] = theme

	tmp, err := os.CreateTemp(dir, "config-*.json.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot create temp file: %v\n", err)
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(raw); err != nil {
		_ = tmp.Close()
		fmt.Fprintf(os.Stderr, "config: write failed: %v\n", err)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		fmt.Fprintf(os.Stderr, "config: sync failed: %v\n", err)
		return err
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "config: close failed: %v\n", err)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		fmt.Fprintf(os.Stderr, "config: rename failed: %v\n", err)
		return err
	}
	c.raw = raw
	return nil
}
