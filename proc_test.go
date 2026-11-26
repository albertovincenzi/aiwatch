package main

import (
	"testing"
	"time"
)

func TestParseEtime(t *testing.T) {
	cases := map[string]time.Duration{
		"00:15":       15 * time.Second,
		"01:23:11":    time.Hour + 23*time.Minute + 11*time.Second,
		"03-22:58:34": 3*24*time.Hour + 22*time.Hour + 58*time.Minute + 34*time.Second,
	}
	for in, want := range cases {
		got, err := parseEtime(in)
		if err != nil || got != want {
			t.Errorf("parseEtime(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseEtime("abc"); err == nil {
		t.Error("parseEtime(abc) should fail")
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"/Users/a/.local/bin/claude":                                   "Claude",
		"/Applications/Claude.app/Contents/Frameworks/Claude Helper":   "Claude",
		"/Applications/ChatGPT.app/Contents/MacOS/ChatGPT":             "ChatGPT",
		"/Applications/ChatGPT.app/Contents/Resources/codex-cli/codex": "Codex",
		"/usr/bin/node": "",
	}
	for in, want := range cases {
		if got := classify(in); got != want {
			t.Errorf("classify(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestParsePS(t *testing.T) {
	out := "  4387   63040  99.1 03-23:06:36 /Applications/ChatGPT.app/Helpers/Codex (Aperitif)\n" +
		"   100    2048   0.0       00:05 /usr/bin/node\n" +
		"garbage\n"
	got := parsePS(out)
	if len(got) != 1 {
		t.Fatalf("want 1 proc, got %d: %+v", len(got), got)
	}
	p := got[0]
	if p.PID != 4387 || p.App != "Codex" || p.CPU != 99.1 || p.MemMB != 63040.0/1024 || p.Path != "/Applications/ChatGPT.app/Helpers/Codex (Aperitif)" {
		t.Errorf("unexpected proc: %+v", p)
	}
}

func TestSampleLive(t *testing.T) {
	procs, err := Sample()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range procs {
		if p.PID <= 0 || p.App == "" || p.Age < 0 {
			t.Errorf("bad proc: %+v", p)
		}
	}
	t.Logf("%d monitored processes", len(procs))
}
