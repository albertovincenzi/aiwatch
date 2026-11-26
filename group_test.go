package main

import (
	"testing"
	"time"
)

func TestBuildRows(t *testing.T) {
	procs := []Proc{
		{PID: 1, App: "Claude", CPU: 10, MemMB: 100, Age: time.Hour, Path: "/a/Claude Helper"},
		{PID: 2, App: "Claude", CPU: 5, MemMB: 200, Age: 3 * time.Hour, Path: "/b/Claude Helper"},
		{PID: 3, App: "Codex", CPU: 50, MemMB: 10, Age: time.Minute, Path: "/c/codex"},
	}
	t.Run("collapsed groups aggregate and single processes are leaves", func(t *testing.T) {
		rows := buildRows(procs, "", "cpu", nil)
		if len(rows) != 2 {
			t.Fatalf("want 2 rows, got %+v", rows)
		}
		if rows[0].Name != "codex" || rows[0].IsGroup || rows[0].PID != 3 {
			t.Errorf("first row should be the codex leaf: %+v", rows[0])
		}
		g := rows[1]
		if !g.IsGroup || g.Count != 2 || g.CPU != 15 || g.MemMB != 300 || g.Age != 3*time.Hour || len(g.PIDs) != 2 {
			t.Errorf("bad group aggregate: %+v", g)
		}
	})
	t.Run("expanded group lists children sorted by key", func(t *testing.T) {
		rows := buildRows(procs, "Claude", "mem", map[string]bool{"Claude|Claude Helper": true})
		if len(rows) != 3 || rows[1].PID != 2 || rows[2].PID != 1 || rows[1].Parent != "Claude|Claude Helper" {
			t.Errorf("unexpected rows: %+v", rows)
		}
	})
}

func TestNameAndDescribe(t *testing.T) {
	cases := []struct{ comm, name, desc string }{
		{"/Applications/ChatGPT.app/Contents/Frameworks/Codex Framework.framework/Helpers/Codex (Aperitif Renderer).app/Contents/MacOS/Codex (Aperitif Renderer)", "Codex (Aperitif Renderer)", "Codex window (Electron renderer)"},
		{"npm exec mcp-remote@0.16.1 https://docs.smartness.com/api/mcp?p=codex-new --header X", "npm exec mcp-remote@0.16.1", "MCP bridge to a remote server"},
		{"/Users/a/.local/bin/claude", "claude", "Claude Code session"},
		{"/usr/bin/unknown-codex-thing", "unknown-codex-thing", ""},
	}
	for _, c := range cases {
		if got := nameOf(c.comm); got != c.name {
			t.Errorf("nameOf(%q) = %q; want %q", c.comm, got, c.name)
		}
		if got := describe(c.comm); got != c.desc {
			t.Errorf("describe(%q) = %q; want %q", c.comm, got, c.desc)
		}
	}
}
