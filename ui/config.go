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
}

// LoadConfig reads the config file from XDG_CONFIG_HOME/spt-flow/config.json
// (or ~/.config/spt-flow/config.json as fallback). Missing, corrupt, or
// unknown-theme files return a default Config{Theme:"default"}; no error is
// surfaced to the user.
func LoadConfig() (*Config, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return &Config{Theme: "default"}, nil
	}
	path := filepath.Join(base, "spt-flow", configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Theme: "default"}, nil
		}
		fmt.Fprintf(os.Stderr, "config: read failed: %v\n", err)
		return &Config{Theme: "default"}, nil
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		fmt.Fprintf(os.Stderr, "config: corrupt file, using default: %v\n", err)
		return &Config{Theme: "default"}, nil
	}

	if _, ok := Themes[c.Theme]; !ok {
		fmt.Fprintf(os.Stderr, "config: unknown theme %q, using default\n", c.Theme)
		return &Config{Theme: "default"}, nil
	}

	return &c, nil
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

	tmp, err := os.CreateTemp(dir, "config-*.json.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot create temp file: %v\n", err)
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "config: write failed: %v\n", err)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
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
	return nil
}
