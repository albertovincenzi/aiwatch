package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Proc is one running process that belongs to a monitored app.
type Proc struct {
	PID   int
	App   string
	CPU   float64
	MemMB float64
	Age   time.Duration
	Path  string
}

var appNames = []string{"Claude", "Codex", "ChatGPT"}

// classify matches the executable path only, so node/MCP children are ignored.
// shortcut: Codex helpers inside ChatGPT.app count as Codex; split them if that matters.
func classify(comm string) string {
	l := strings.ToLower(comm)
	switch {
	case strings.Contains(l, "codex"):
		return "Codex"
	case strings.Contains(l, "chatgpt"):
		return "ChatGPT"
	case strings.Contains(l, "claude"):
		return "Claude"
	}
	return ""
}

// parseEtime reads the ps elapsed time: [[dd-]hh:]mm:ss.
func parseEtime(s string) (time.Duration, error) {
	days := 0
	if d, rest, ok := strings.Cut(s, "-"); ok {
		n, err := strconv.Atoi(d)
		if err != nil {
			return 0, err
		}
		days, s = n, rest
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("bad etime %q", s)
	}
	secs := 0
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, err
		}
		secs = secs*60 + n
	}
	return time.Duration(days*86400+secs) * time.Second, nil
}

// parsePS parses "pid rss pcpu etime comm..." lines; unparsable lines are skipped.
func parsePS(out string) []Proc {
	var procs []Proc
	for _, line := range strings.Split(out, "\n") {
		rest := strings.TrimSpace(line)
		var f [4]string
		ok := true
		for i := range f {
			var found bool
			f[i], rest, found = strings.Cut(rest, " ")
			rest = strings.TrimSpace(rest)
			if !found {
				ok = false
				break
			}
		}
		app := classify(rest)
		if !ok || app == "" {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		rss, e2 := strconv.ParseFloat(f[1], 64)
		cpu, e3 := strconv.ParseFloat(f[2], 64)
		age, e4 := parseEtime(f[3])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			continue
		}
		procs = append(procs, Proc{PID: pid, App: app, CPU: cpu, MemMB: rss / 1024, Age: age, Path: rest})
	}
	return procs
}

// Sample runs ps once and returns the monitored processes.
func Sample() ([]Proc, error) {
	cmd := exec.Command("ps", "-Ao", "pid=,rss=,pcpu=,etime=,comm=")
	cmd.Env = append(os.Environ(), "LC_ALL=C") // the it_IT locale prints "97,5"
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parsePS(string(out)), nil
}

// nameOf gives a short name: the file name for a path, else the first words of a process title
// (npm sets titles such as "npm exec mcp-remote@0.16.1 https://...").
func nameOf(comm string) string {
	if strings.HasPrefix(comm, "/") {
		return filepath.Base(comm)
	}
	f := strings.Fields(comm)
	return strings.Join(f[:min(len(f), 3)], " ")
}

// roles maps a lowercase path fragment to a plain description. First match wins.
// shortcut: guessed from the path only, nothing is verified at run time; extend the list as needed.
var roles = []struct{ frag, desc string }{
	{"aperitif renderer", "Codex window (Electron renderer)"},
	{"aperitif", "Codex Electron helper (GPU/network)"},
	{"sparkle", "Auto-updater"},
	{"bare-modifier-monitor", "ChatGPT hotkey monitor (double Command)"},
	{"skycomputeruseservice", "Codex Computer Use service"},
	{"codex-code-mode-host", "Codex code-mode sandbox host"},
	{"node_repl", "Node REPL for computer use"},
	{"mcp-remote", "MCP bridge to a remote server"},
	{"chatgpt for chrome", "ChatGPT Chrome extension host"},
	{"app-server-daemon", "Codex app-server daemon"},
	{"artifact-sessions", "Codex artifact runtime (node)"},
	{"codexcli.app", "Codex agent CLI"},
	{"bin/codex", "Codex agent CLI"},
	{"claude helper (renderer)", "Claude Desktop window"},
	{"claude helper", "Claude Desktop helper"},
	{"claude-code/", "Claude Code bundled by Desktop"},
	{"claude.app/contents/macos", "Claude Desktop app"},
	{".local/bin/claude", "Claude Code session"},
	{"chatgpt.app/contents/macos/chatgpt", "ChatGPT desktop app"},
}

func describe(comm string) string {
	l := strings.ToLower(comm)
	for _, r := range roles {
		if strings.Contains(l, r.frag) {
			return r.desc
		}
	}
	return ""
}
