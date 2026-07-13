package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Header               lipgloss.ANSIColor
	Footer               lipgloss.ANSIColor
	LyricActive          lipgloss.ANSIColor
	LyricInactive        lipgloss.ANSIColor
	LyricPlain           lipgloss.ANSIColor
	Visualizer           lipgloss.ANSIColor
	VisualizerBackground lipgloss.ANSIColor
	Error                lipgloss.ANSIColor
	Waiting              lipgloss.ANSIColor
	MenuTitle            lipgloss.ANSIColor
	MenuOption           lipgloss.ANSIColor
	MenuSelected         lipgloss.ANSIColor
	MenuDim              lipgloss.ANSIColor
	MenuBorder           lipgloss.ANSIColor
}

var DefaultTheme = Theme{
	Header:               lipgloss.ANSIColor(15),
	Footer:               lipgloss.ANSIColor(12),
	LyricActive:          lipgloss.ANSIColor(12),
	LyricInactive:        lipgloss.ANSIColor(15),
	LyricPlain:           lipgloss.ANSIColor(7),
	Visualizer:           lipgloss.ANSIColor(12),
	VisualizerBackground: lipgloss.ANSIColor(10),
	Error:                lipgloss.ANSIColor(9),
	Waiting:              lipgloss.ANSIColor(7),
	MenuTitle:            lipgloss.ANSIColor(15),
	MenuOption:           lipgloss.ANSIColor(7),
	MenuSelected:         lipgloss.ANSIColor(12),
	MenuDim:              lipgloss.ANSIColor(8),
	MenuBorder:           lipgloss.ANSIColor(8),
}
