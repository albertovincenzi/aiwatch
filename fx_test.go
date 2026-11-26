package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSpark(t *testing.T) {
	if got := spark([]float64{0, 0.5, 1}, 1, 5); got != "  ▁▅█" {
		t.Errorf("spark = %q", got)
	}
	if got := spark([]float64{1, 1, 1, 1}, 1, 2); utf8.RuneCountInString(got) != 2 {
		t.Errorf("spark must keep the last width values only: %q", got)
	}
}

func TestStackedBarKeepsWidth(t *testing.T) {
	segs := []seg{{0.6, "1"}, {0.3, "2"}, {0.2, "3"}} // sums above 1: must not overflow
	if got := stackedBar(segs, 10); utf8.RuneCountInString(got) != 10 || strings.Count(got, "░") != 0 {
		t.Errorf("stackedBar = %q", got)
	}
}
