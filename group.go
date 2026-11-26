package main

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

// Row is one visible line: a group of processes with the same app and
// executable, a single process (leaf) or a child of an expanded group.
type Row struct {
	Key, Parent, App, Name string
	IsGroup, Expanded      bool
	PID                    int   // set on leaf and child rows
	PIDs                   []int // every PID the row stands for: the kill targets
	Count                  int
	CPU, MemMB             float64
	Age                    time.Duration // oldest process of the row
	Desc, Path             string        // role guess and full path of the first process
}

func procRow(p Proc, parent string) Row {
	return Row{Key: "pid:" + strconv.Itoa(p.PID), Parent: parent, App: p.App, Name: nameOf(p.Path), Desc: describe(p.Path), Path: p.Path,
		PID: p.PID, PIDs: []int{p.PID}, Count: 1, CPU: p.CPU, MemMB: p.MemMB, Age: p.Age}
}

func sortRows(rows []Row, sortKey string) {
	sort.SliceStable(rows, func(i, j int) bool {
		switch sortKey {
		case "mem":
			return rows[i].MemMB > rows[j].MemMB
		case "age":
			return rows[i].Age > rows[j].Age
		}
		return rows[i].CPU > rows[j].CPU
	})
}

// buildRows groups procs by app and executable name, sorts groups and children
// by sortKey, and lists the children of the groups marked in expanded.
// An empty app means no filter. A group of one process is shown as a leaf.
func buildRows(procs []Proc, app, sortKey string, expanded map[string]bool) []Row {
	members := map[string][]Proc{}
	var heads []Row
	for _, p := range procs {
		if app != "" && p.App != app {
			continue
		}
		key := p.App + "|" + nameOf(p.Path)
		if _, seen := members[key]; !seen {
			heads = append(heads, Row{Key: key, App: p.App, Name: nameOf(p.Path), Desc: describe(p.Path), Path: p.Path, IsGroup: true})
		}
		members[key] = append(members[key], p)
	}
	for i, h := range heads {
		ps := members[h.Key]
		if len(ps) == 1 {
			heads[i] = procRow(ps[0], "")
			continue
		}
		for _, p := range ps {
			h.Count++
			h.CPU += p.CPU
			h.MemMB += p.MemMB
			h.Age = max(h.Age, p.Age)
			h.PIDs = append(h.PIDs, p.PID)
		}
		h.Expanded = expanded[h.Key]
		heads[i] = h
	}
	sortRows(heads, sortKey)

	rows := make([]Row, 0, len(procs))
	for _, h := range heads {
		rows = append(rows, h)
		if !h.IsGroup || !h.Expanded {
			continue
		}
		kids := make([]Row, 0, h.Count)
		for _, p := range members[h.Key] {
			kids = append(kids, procRow(p, h.Key))
		}
		sortRows(kids, sortKey)
		rows = append(rows, kids...)
	}
	return rows
}

// label names the row for the kill confirmation.
func (r Row) label() string {
	if r.IsGroup {
		return fmt.Sprintf("%d processes of %s", r.Count, r.Name)
	}
	return fmt.Sprintf("%s PID %d", r.Name, r.PID)
}
