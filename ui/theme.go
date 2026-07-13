package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Header               lipgloss.Color
	Footer               lipgloss.Color
	LyricActive          lipgloss.Color
	LyricInactive        lipgloss.Color
	LyricPlain           lipgloss.Color
	Visualizer           lipgloss.Color
	VisualizerBackground lipgloss.Color
	Error                lipgloss.Color
	Waiting              lipgloss.Color
	MenuTitle            lipgloss.Color
	MenuOption           lipgloss.Color
	MenuSelected         lipgloss.Color
	MenuDim              lipgloss.Color
	MenuBorder           lipgloss.Color
}

var DefaultTheme = Theme{
	Header:               lipgloss.Color("15"),
	Footer:               lipgloss.Color("12"),
	LyricActive:          lipgloss.Color("12"),
	LyricInactive:        lipgloss.Color("15"),
	LyricPlain:           lipgloss.Color("7"),
	Visualizer:           lipgloss.Color("12"),
	VisualizerBackground: lipgloss.Color("10"),
	Error:                lipgloss.Color("9"),
	Waiting:              lipgloss.Color("7"),
	MenuTitle:            lipgloss.Color("15"),
	MenuOption:           lipgloss.Color("7"),
	MenuSelected:         lipgloss.Color("12"),
	MenuDim:              lipgloss.Color("8"),
	MenuBorder:           lipgloss.Color("8"),
}
