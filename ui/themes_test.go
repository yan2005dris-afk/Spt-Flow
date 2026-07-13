package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestThemes_AllNamesRegistered(t *testing.T) {
	for _, name := range ThemeOrder {
		if _, ok := Themes[name]; !ok {
			t.Errorf("Theme %q is in ThemeOrder but not in Themes map", name)
		}
	}
}

func TestThemes_AllFieldsPopulated(t *testing.T) {
	colorFields := []string{
		"Header", "Footer", "LyricActive", "LyricInactive", "LyricPlain",
		"Visualizer", "VisualizerBackground", "Error", "Waiting",
		"MenuTitle", "MenuOption", "MenuSelected", "MenuDim", "MenuBorder",
	}
	for _, name := range ThemeOrder {
		theme := Themes[name]
		for _, field := range colorFields {
			if isColorZero(getColor(theme, field)) {
				t.Errorf("Theme %q has zero %s color", name, field)
			}
		}
	}
}

func TestCycleTheme(t *testing.T) {
	tests := []struct {
		current string
		want    string
	}{
		{"default", "catppuccin-mocha"},
		{"catppuccin-mocha", "gruvbox-dark"},
		{"gruvbox-dark", "default"},
		{"unknown", "default"}, // fallback
	}

	for _, tt := range tests {
		got := CycleTheme(tt.current)
		if got != tt.want {
			t.Errorf("CycleTheme(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

// isColorZero reports whether c is a zero lipgloss.Color (empty string).
func isColorZero(c lipgloss.Color) bool {
	return c == lipgloss.Color("")
}

// getColor returns the value of a color field by name using reflection-like logic.
func getColor(theme Theme, name string) lipgloss.Color {
	switch name {
	case "Header":
		return theme.Header
	case "Footer":
		return theme.Footer
	case "LyricActive":
		return theme.LyricActive
	case "LyricInactive":
		return theme.LyricInactive
	case "LyricPlain":
		return theme.LyricPlain
	case "Visualizer":
		return theme.Visualizer
	case "VisualizerBackground":
		return theme.VisualizerBackground
	case "Error":
		return theme.Error
	case "Waiting":
		return theme.Waiting
	case "MenuTitle":
		return theme.MenuTitle
	case "MenuOption":
		return theme.MenuOption
	case "MenuSelected":
		return theme.MenuSelected
	case "MenuDim":
		return theme.MenuDim
	case "MenuBorder":
		return theme.MenuBorder
	default:
		return lipgloss.Color("")
	}
}
