package ui

import (
	"math"
)

type Visualizer struct {
	Width     int
	MaxHeight float64
	Heights   []float64
	Tick      int
}

func NewVisualizer(width int, maxHeight float64) *Visualizer {
	return &Visualizer{
		Width:     width,
		MaxHeight: maxHeight,
		Heights:   make([]float64, width),
		Tick:      0,
	}
}

func (v *Visualizer) Update(playing bool) {
	if playing {
		v.Tick++
		for i := 0; i < v.Width; i++ {
			fi := 0.05 + 0.15*(float64(i)/float64(v.Width))
			t := float64(v.Tick)
			hi := v.MaxHeight * (0.5 + 0.4*math.Sin(fi*t+float64(i)) + 0.1*math.Cos(fi*2.3*t))
			if hi < 0 {
				hi = 0
			}
			if hi > v.MaxHeight {
				hi = v.MaxHeight
			}
			v.Heights[i] = hi
		}
	} else {
		for i := 0; i < v.Width; i++ {
			v.Heights[i] = v.Heights[i] * 0.8
			if v.Heights[i] < 0.01 {
				v.Heights[i] = 0
			}
		}
	}
}

// Render returns the visualizer grid as a slice of lines, from top to bottom.
func (v *Visualizer) Render(height int) []string {
	if height <= 0 {
		return nil
	}

	blocks := []rune{' ', ' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	lines := make([]string, height)

	for y := height - 1; y >= 0; y-- {
		var sb stringsBuilder
		for x := 0; x < v.Width; x++ {
			h := v.Heights[x]
			// Scale height to grid height
			scaledH := h * (float64(height) / v.MaxHeight)
			cellH := scaledH - float64(y)

			if cellH >= 1.0 {
				sb.WriteRune('█')
			} else if cellH <= 0 {
				sb.WriteRune(' ')
			} else {
				idx := int(cellH * 8)
				if idx < 0 {
					idx = 0
				}
				if idx > 8 {
					idx = 8
				}
				sb.WriteRune(blocks[idx])
			}
		}
		lines[height-1-y] = sb.String()
	}
	return lines
}

// Simple custom builder to avoid string imports if possible, or we can just import strings.
type stringsBuilder struct {
	runes []rune
}

func (s *stringsBuilder) WriteRune(r rune) {
	s.runes = append(s.runes, r)
}

func (s *stringsBuilder) String() string {
	return string(s.runes)
}
