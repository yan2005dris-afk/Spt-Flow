package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Missing(t *testing.T) {
	// Use a temp config dir that definitely has no config file.
	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.Theme != "default" {
		t.Errorf("Expected theme 'default' for missing file, got %q", cfg.Theme)
	}
}

func TestLoadConfig_Corrupt(t *testing.T) {
	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	// Write corrupt JSON.
	dir := filepath.Join(tmp, "spt-flow")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig should not return error for corrupt file, got: %v", err)
	}
	if cfg.Theme != "default" {
		t.Errorf("Expected theme 'default' for corrupt file, got %q", cfg.Theme)
	}
}

func TestLoadConfig_UnknownTheme(t *testing.T) {
	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	dir := filepath.Join(tmp, "spt-flow")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"theme":"bogus"}`), 0644); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig should not return error for unknown theme, got: %v", err)
	}
	if cfg.Theme != "default" {
		t.Errorf("Expected theme 'default' for unknown theme, got %q", cfg.Theme)
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	dir := filepath.Join(tmp, "spt-flow")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"theme":"gruvbox-dark"}`), 0644); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.Theme != "gruvbox-dark" {
		t.Errorf("Expected theme 'gruvbox-dark', got %q", cfg.Theme)
	}
}

func TestConfig_Save(t *testing.T) {
	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := &Config{Theme: "catppuccin-mocha"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load it back.
	cfg2, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig after Save failed: %v", err)
	}
	if cfg2.Theme != "catppuccin-mocha" {
		t.Errorf("Expected 'catppuccin-mocha' after round-trip, got %q", cfg2.Theme)
	}
}

func TestConfig_Save_PreservesUnknownFields(t *testing.T) {
	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	dir := filepath.Join(tmp, "spt-flow")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("cannot create config dir: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"theme":"default","volume":80,"nested":{"enabled":true}}`), 0644); err != nil {
		t.Fatalf("cannot write config: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	cfg.Theme = "gruvbox-dark"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read saved config: %v", err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("saved config is invalid JSON: %v", err)
	}

	var theme string
	if err := json.Unmarshal(saved["theme"], &theme); err != nil || theme != "gruvbox-dark" {
		t.Errorf("Expected theme 'gruvbox-dark', got %q (error: %v)", theme, err)
	}
	var volume int
	if err := json.Unmarshal(saved["volume"], &volume); err != nil || volume != 80 {
		t.Errorf("Expected preserved volume 80, got %d (error: %v)", volume, err)
	}
	var nested map[string]bool
	if err := json.Unmarshal(saved["nested"], &nested); err != nil || !nested["enabled"] {
		t.Errorf("Expected preserved nested field, got %v (error: %v)", nested, err)
	}
}

func TestConfig_Save_ReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod restrictions don't apply to root")
	}

	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}

	// Make the directory read-only.
	dir := filepath.Join(tmp, "spt-flow")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("Chmod() = %v", err)
	}
	defer func() {
		_ = os.Chmod(dir, 0755)
		_ = os.RemoveAll(tmp)
	}()

	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := &Config{Theme: "gruvbox-dark"}
	err = cfg.Save()
	if err == nil {
		t.Error("Expected Save to fail on read-only directory, but it succeeded")
	}
	// Should not crash — in-memory state still has the change.
	if cfg.Theme != "gruvbox-dark" {
		t.Errorf("In-memory theme should still be 'gruvbox-dark', got %q", cfg.Theme)
	}
}
