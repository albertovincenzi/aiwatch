package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type rgb struct{ r, g, b int }

var (
	cGreen  = rgb{80, 220, 130}
	cYellow = rgb{245, 205, 70}
	cRed    = rgb{245, 85, 85}
)

func (c rgb) color() lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b))
}

func lerp(a, b rgb, t float64) rgb {
	mix := func(x, y int) int { return x + int(float64(y-x)*t+0.5) }
	return rgb{mix(a.r, b.r), mix(a.g, b.g), mix(a.b, b.b)}
}

// heat maps 0..1 to green, yellow, red.
func heat(t float64) rgb {
	t = min(max(t, 0), 1)
	if t < 0.5 {
		return lerp(cGreen, cYellow, t*2)
	}
	return lerp(cYellow, cRed, (t-0.5)*2)
}

// gradBar is a bar whose cells go from green to red along its length.
func gradBar(frac float64, width int) string {
	fill, track := bar(frac, width)
	var b strings.Builder
	for i, r := range []rune(fill) {
		st := lipgloss.NewStyle().Foreground(heat(float64(i) / float64(max(width-1, 1))).color())
		b.WriteString(st.Render(string(r)))
	}
	return b.String() + dimSt.Render(track)
}

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// spark draws the last width values as a sparkline scaled to ceil, left-padded with spaces.
func spark(vals []float64, ceil float64, width int) string {
	if len(vals) > width {
		vals = vals[len(vals)-width:]
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", width-len(vals)))
	for _, v := range vals {
		i := 0
		if ceil > 0 {
			i = int(min(max(v/ceil, 0), 1)*7 + 0.5)
		}
		b.WriteRune(sparkRunes[i])
	}
	return b.String()
}

func gradientText(s string, a, b rgb) string {
	r := []rune(s)
	var out strings.Builder
	for i, ch := range r {
		t := float64(i) / float64(max(len(r)-1, 1))
		out.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lerp(a, b, t).color()).Render(string(ch)))
	}
	return out.String()
}

type seg struct {
	frac  float64
	color lipgloss.Color
}

// stackedBar draws one bar split between segments, with a dim track for the rest.
func stackedBar(segs []seg, width int) string {
	var b strings.Builder
	used := 0
	for _, s := range segs {
		n := int(s.frac*float64(width) + 0.5)
		if s.frac > 0 && n == 0 {
			n = 1
		}
		n = min(n, width-used)
		b.WriteString(lipgloss.NewStyle().Foreground(s.color).Render(strings.Repeat("█", n)))
		used += n
	}
	return b.String() + dimSt.Render(strings.Repeat("░", width-used))
}
