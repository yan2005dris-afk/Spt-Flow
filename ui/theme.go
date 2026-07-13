package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Header        lipgloss.ANSIColor
	Footer        lipgloss.ANSIColor
	LyricActive   lipgloss.ANSIColor
	LyricInactive lipgloss.ANSIColor
	LyricPlain    lipgloss.ANSIColor
	Visualizer    lipgloss.ANSIColor
	Error         lipgloss.ANSIColor
	Waiting       lipgloss.ANSIColor
}

var DefaultTheme = Theme{
	Header:        lipgloss.ANSIColor(15),
	Footer:        lipgloss.ANSIColor(12),
	LyricActive:   lipgloss.ANSIColor(12),
	LyricInactive: lipgloss.ANSIColor(15),
	LyricPlain:    lipgloss.ANSIColor(7),
	Visualizer:    lipgloss.ANSIColor(8),
	Error:         lipgloss.ANSIColor(9),
	Waiting:       lipgloss.ANSIColor(7),
}
