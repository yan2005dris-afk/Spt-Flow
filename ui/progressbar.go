package ui

import (
	"fmt"
	"strings"
	"time"
)

func renderProgressBar(width int, pos, dur time.Duration) string {
	if dur <= 0 {
		return strings.Repeat("░", width)
	}
	ratio := float64(pos) / float64(dur)
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(width))
	empty := width - filled
	if empty < 0 {
		empty = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func formatDuration(d time.Duration) string {
	s := int(d.Seconds())
	m := s / 60
	s = s % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}