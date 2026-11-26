package main

import (
	"testing"
	"unicode/utf8"
)

func TestBar(t *testing.T) {
	cases := []struct {
		frac       float64
		fill, rest string
	}{
		{0, "", "░░░░"},
		{0.5, "██", "░░"},
		{0.625, "██▌", "░"},
		{1, "████", ""},
		{7, "████", ""},     // clamped
		{-1, "", "░░░░"},    // clamped
		{0.001, "▏", "░░░"}, // tiny but visible
	}
	for _, c := range cases {
		fill, rest := bar(c.frac, 4)
		if fill != c.fill || rest != c.rest || utf8.RuneCountInString(fill+rest) != 4 {
			t.Errorf("bar(%v, 4) = %q,%q; want %q,%q", c.frac, fill, rest, c.fill, c.rest)
		}
	}
}
