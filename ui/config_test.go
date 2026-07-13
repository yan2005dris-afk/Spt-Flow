package ui

import (
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
	defer os.RemoveAll(tmp)

	oldConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigDir)

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
	defer os.RemoveAll(tmp)

	// Write corrupt JSON.
	dir := filepath.Join(tmp, "spt-flow")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not valid json"), 0644)

	oldConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigDir)

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
	defer os.RemoveAll(tmp)

	dir := filepath.Join(tmp, "spt-flow")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"theme":"bogus"}`), 0644)

	oldConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigDir)

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
	defer os.RemoveAll(tmp)

	dir := filepath.Join(tmp, "spt-flow")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"theme":"gruvbox-dark"}`), 0644)

	oldConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigDir)

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
	defer os.RemoveAll(tmp)

	oldConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigDir)

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

func TestConfig_Save_ReadOnly(t *testing.T) {
	tmp, err := os.MkdirTemp("", "spt-flow-test-*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	// Make the directory read-only.
	dir := filepath.Join(tmp, "spt-flow")
	os.MkdirAll(dir, 0755)
	os.Chmod(dir, 0555)

	oldConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigDir)

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
