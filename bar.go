package main

import (
	"strings"
	"unicode/utf8"
)

var eighths = []rune(" ▏▎▍▌▋▊▉")

// bar splits a width-cell bar into the filled part and the empty track.
// frac is clamped to 0..1; a non-zero value always shows at least 1/8 of a cell.
func bar(frac float64, width int) (fill, track string) {
	frac = min(max(frac, 0), 1)
	eights := int(frac*float64(width*8) + 0.5)
	if frac > 0 && eights == 0 {
		eights = 1
	}
	fill = strings.Repeat("█", eights/8)
	if rem := eights % 8; rem > 0 {
		fill += string(eighths[rem])
	}
	return fill, strings.Repeat("░", width-utf8.RuneCountInString(fill))
}
