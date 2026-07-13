package ui

import "github.com/charmbracelet/lipgloss"

// Themes is the registry of all available palettes.
var Themes = map[string]Theme{
	"default":          DefaultTheme,
	"catppuccin-mocha": CatppuccinMocha,
	"gruvbox-dark":     GruvboxDark,
}

// ThemeOrder is the canonical list of palette names, used for cycling.
var ThemeOrder = []string{"default", "catppuccin-mocha", "gruvbox-dark"}

// CatppuccinMocha is the Catppuccin Mocha palette.
var CatppuccinMocha = Theme{
	Header:               lipgloss.Color("#cdd6f4"),
	Footer:               lipgloss.Color("#89b4fa"),
	LyricActive:          lipgloss.Color("#cba6f7"),
	LyricInactive:        lipgloss.Color("#cdd6f4"),
	LyricPlain:           lipgloss.Color("#a6adc8"),
	Visualizer:           lipgloss.Color("#a6e3a1"),
	VisualizerBackground: lipgloss.Color("#f9e2af"),
	Error:                lipgloss.Color("#f38ba8"),
	Waiting:              lipgloss.Color("#a6adc8"),
	MenuTitle:            lipgloss.Color("#cba6f7"),
	MenuOption:           lipgloss.Color("#cdd6f4"),
	MenuSelected:         lipgloss.Color("#fab387"),
	MenuDim:              lipgloss.Color("#585b70"),
	MenuBorder:           lipgloss.Color("#45475a"),
}

// GruvboxDark is the Gruvbox dark palette.
var GruvboxDark = Theme{
	Header:               lipgloss.Color("#ebdbb2"),
	Footer:               lipgloss.Color("#83a598"),
	LyricActive:          lipgloss.Color("#d3869b"),
	LyricInactive:        lipgloss.Color("#ebdbb2"),
	LyricPlain:           lipgloss.Color("#a89984"),
	Visualizer:           lipgloss.Color("#b8bb26"),
	VisualizerBackground: lipgloss.Color("#fabd2f"),
	Error:                lipgloss.Color("#fb4934"),
	Waiting:              lipgloss.Color("#a89984"),
	MenuTitle:            lipgloss.Color("#fabd2f"),
	MenuOption:           lipgloss.Color("#ebdbb2"),
	MenuSelected:         lipgloss.Color("#fe8019"),
	MenuDim:              lipgloss.Color("#665c54"),
	MenuBorder:           lipgloss.Color("#504945"),
}

// CycleTheme returns the next palette name in ThemeOrder, wrapping to the first.
func CycleTheme(current string) string {
	for i, name := range ThemeOrder {
		if name == current {
			return ThemeOrder[(i+1)%len(ThemeOrder)]
		}
	}
	return ThemeOrder[0] // fallback to default
}

// ThemeName returns the name of the given theme by reverse-looking it up
// in the Themes registry. Returns "default" if not found.
func (t Theme) Name() string {
	for name, theme := range Themes {
		if theme == t {
			return name
		}
	}
	return "default"
}
