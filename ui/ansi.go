package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// lipglossToAnsi converts a lipgloss.Color (string) to its raw ANSI escape
// sequence for foreground color. Supports both hex colors ("#rrggbb") and
// ANSI numbers ("15", "12", "256:N").
func lipglossToAnsi(c lipgloss.Color) string {
	s := string(c)
	if strings.HasPrefix(s, "#") {
		return hexToAnsi(s)
	}
	// ANSI 16-color
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 15 {
		if n < 8 {
			return fmt.Sprintf("\x1b[%dm", 30+n)
		}
		return fmt.Sprintf("\x1b[%dm", 90+n-8)
	}
	// 256-color: "256:N"
	if strings.HasPrefix(s, "256:") {
		if n, err := strconv.Atoi(strings.TrimPrefix(s, "256:")); err == nil {
			return fmt.Sprintf("\x1b[38;5;%dm", n)
		}
	}
	// Fallback
	return "\x1b[93m" // bright yellow
}

// hexToAnsi converts "#rrggbb" to a 24-bit ANSI foreground escape.
func hexToAnsi(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "\x1b[93m"
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}