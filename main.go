package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	refreshEvery = 2 * time.Second
	nameWidth    = 34
)

var (
	boldSt = lipgloss.NewStyle().Bold(true)
	dimSt  = lipgloss.NewStyle().Faint(true)
	selSt  = lipgloss.NewStyle().Reverse(true)
	warnSt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
)

var sortNames = []string{"cpu", "mem", "age"}

type sampleMsg struct {
	procs []Proc
	err   error
}

type killMsg struct {
	sent, failed int
	sig          syscall.Signal
	err          error // first error
}

type pendingKill struct {
	row Row
	sig syscall.Signal
}

type model struct {
	all      []Proc
	view     []Row
	expanded map[string]bool
	cursor   int
	sortKey  int
	filter   int // 0 = all apps, i = appNames[i-1]
	confirm  *pendingKill
	status   string
	w, h     int
	ramMB    float64 // installed RAM, 0 if unknown
	ncpu     int
	hist     map[string][]float64 // "App/cpu" and "App/mem" samples, newest last
	beat     bool                 // flips on every sample: the heartbeat
}

func doSample() tea.Msg {
	p, err := Sample()
	return sampleMsg{p, err}
}

func killCmd(pids []int, sig syscall.Signal) tea.Cmd {
	return func() tea.Msg {
		msg := killMsg{sig: sig}
		for _, pid := range pids {
			if err := syscall.Kill(pid, sig); err != nil {
				msg.failed++
				if msg.err == nil {
					msg.err = fmt.Errorf("kill %d: %w", pid, err)
				}
			} else {
				msg.sent++
			}
		}
		return msg
	}
}

// pushHistory returns a copy of the history with the new CPU and memory sums of each app.
func (m model) pushHistory(procs []Proc) map[string][]float64 {
	next := make(map[string][]float64, len(m.hist))
	for k, v := range m.hist {
		next[k] = v
	}
	for _, app := range appNames {
		var cpu, mem float64
		for _, p := range procs {
			if p.App == app {
				cpu += p.CPU
				mem += p.MemMB
			}
		}
		for k, v := range map[string]float64{app + "/cpu": cpu, app + "/mem": mem} {
			h := append(append([]float64{}, next[k]...), v)
			next[k] = h[max(len(h)-histLen, 0):]
		}
	}
	return next
}

func (m model) Init() tea.Cmd { return doSample }

func (m model) selectedKey() string {
	if m.cursor < len(m.view) {
		return m.view[m.cursor].Key
	}
	return ""
}

// rebuild regroups the rows and keeps the cursor on the row with keepKey.
func (m model) rebuild(keepKey string) model {
	app := ""
	if m.filter > 0 {
		app = appNames[m.filter-1]
	}
	m.view = buildRows(m.all, app, sortNames[m.sortKey], m.expanded)
	m.cursor = min(m.cursor, max(len(m.view)-1, 0))
	for i, r := range m.view {
		if r.Key == keepKey {
			m.cursor = i
		}
	}
	return m
}

// withExpanded returns a copy of the model with one group opened or closed.
func (m model) withExpanded(key string, open bool) model {
	next := make(map[string]bool, len(m.expanded)+1)
	for k, v := range m.expanded {
		next[k] = v
	}
	next[key] = open
	m.expanded = next
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case sampleMsg:
		if msg.err != nil {
			m.status = "ps error: " + msg.err.Error()
		} else {
			m.all = msg.procs
			m.hist = m.pushHistory(msg.procs)
			m.beat = !m.beat
			m = m.rebuild(m.selectedKey())
		}
		return m, tea.Tick(refreshEvery, func(time.Time) tea.Msg { return doSample() })
	case killMsg:
		m.status = fmt.Sprintf("sent %v to %d process(es)", msg.sig, msg.sent)
		if msg.failed > 0 {
			m.status += fmt.Sprintf(", %d failed (%v)", msg.failed, msg.err)
		}
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if c := m.confirm; c != nil {
		m.confirm = nil
		if msg.String() == "y" {
			return m, killCmd(c.row.PIDs, c.sig)
		}
		m.status = "cancelled"
		return m, nil
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up":
		m.cursor = max(m.cursor-1, 0)
	case "down":
		m.cursor = min(m.cursor+1, max(len(m.view)-1, 0))
	case "right", "enter", " ":
		if r := m.currentRow(); r != nil && r.IsGroup {
			open := msg.String() == "right" || !r.Expanded
			return m.withExpanded(r.Key, open).rebuild(r.Key), nil
		}
	case "left":
		if r := m.currentRow(); r != nil {
			key := r.Key
			if r.Parent != "" {
				key = r.Parent
			}
			if r.IsGroup || r.Parent != "" {
				return m.withExpanded(key, false).rebuild(key), nil
			}
		}
	case "s":
		m.sortKey = (m.sortKey + 1) % len(sortNames)
		return m.rebuild(m.selectedKey()), nil
	case "f":
		m.filter = (m.filter + 1) % (len(appNames) + 1)
		return m.rebuild(m.selectedKey()), nil
	case "k", "K":
		if r := m.currentRow(); r != nil {
			sig := syscall.SIGTERM
			if msg.String() == "K" {
				sig = syscall.SIGKILL
			}
			m.confirm = &pendingKill{*r, sig}
		}
	}
	return m, nil
}

func (m model) currentRow() *Row {
	if m.cursor < len(m.view) {
		return &m.view[m.cursor]
	}
	return nil
}

func fmtMem(mb float64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", mb/1024)
	}
	return fmt.Sprintf("%.0f MB", mb)
}

func fmtAge(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%d d %d h", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
	}
	if d < time.Minute {
		return fmt.Sprintf("%d s", int(d.Seconds()))
	}
	return fmt.Sprintf("%d min %d s", int(d.Minutes()), int(d.Seconds())%60)
}

var appColors = map[string]lipgloss.Color{
	"Claude": "208", "Codex": "45", "ChatGPT": "42",
}

// level picks green, yellow or red by thresholds: value >= warn is yellow, >= bad is red.
func level(v, warn, bad float64) lipgloss.Style {
	switch {
	case v >= bad:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	case v >= warn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
}

func cpuSt(cpu float64) lipgloss.Style     { return level(cpu, 30, 80) }
func memSt(mb float64) lipgloss.Style      { return level(mb, 300, 1024) }
func ageSt(d time.Duration) lipgloss.Style { return level(d.Hours(), 24, 72) }

// cell pads before styling so ANSI codes do not break the column widths.
func cell(st lipgloss.Style, text string, width int, right bool) string {
	if right {
		return st.Render(fmt.Sprintf("%*s", width, text))
	}
	return st.Render(fmt.Sprintf("%-*s", width, text))
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

const (
	barWidth = 8
	histLen  = 20
	newAge   = 15 * time.Second // processes younger than this get a ✦
)

// bar renders a colored bar with a dim track.
func barCell(_ lipgloss.Style, frac float64) string { return gradBar(frac, barWidth) }

func summaryLine(name string, color lipgloss.Color, n int, s Proc, withAge bool, cpuFrac, memFrac float64, cpuSp, memSp string) string {
	line := cell(boldSt.Foreground(color), name, 8, false) + " " + fmt.Sprintf("%3d proc  CPU ", n) +
		cell(cpuSt(s.CPU), fmt.Sprintf("%.1f%%", s.CPU), 7, true) + " " + barCell(cpuSt(s.CPU), cpuFrac) + cpuSp +
		"  MEM " + cell(memSt(s.MemMB), fmtMem(s.MemMB), 8, true) + " " + barCell(memSt(s.MemMB), memFrac)
	if memFrac > 0 {
		line += fmt.Sprintf(" %4.1f%%", memFrac*100)
	}
	line += memSp
	if withAge && n > 0 {
		line += "  oldest " + cell(ageSt(s.Age), fmtAge(s.Age), 10, false)
	}
	return line + "\n"
}

func (m model) summary() string {
	var b strings.Builder
	var total Proc
	nTotal := 0
	var segs []seg
	allCores := float64(max(m.ncpu, 1) * 100)
	ram := m.ramMB
	if ram == 0 { // unknown RAM: scale the bar to the biggest app instead
		for _, p := range m.all {
			ram += p.MemMB
		}
	}
	for _, app := range appNames {
		var s Proc
		n := 0
		for _, p := range m.all {
			if p.App == app {
				n++
				s.CPU += p.CPU
				s.MemMB += p.MemMB
				s.Age = max(s.Age, p.Age)
			}
		}
		total.CPU, total.MemMB, nTotal = total.CPU+s.CPU, total.MemMB+s.MemMB, nTotal+n
		cpuSp, memSp := m.sparks(app)
		b.WriteString(summaryLine(app, appColors[app], n, s, true, s.CPU/allCores, safeDiv(s.MemMB, ram), cpuSp, memSp))
		segs = append(segs, seg{safeDiv(s.MemMB, ram), appColors[app]})
	}
	b.WriteString(summaryLine("TOTAL", "255", nTotal, total, false, total.CPU/allCores, safeDiv(total.MemMB, ram), "", ""))
	if m.ramMB > 0 {
		b.WriteString(fmt.Sprintf("\n%-8s %s %s\n", "RAM MAP", stackedBar(segs, 60), dimSt.Render(fmt.Sprintf("%s of %s used by these apps (%.1f%%)", fmtMem(total.MemMB), fmtMem(m.ramMB), safeDiv(total.MemMB, m.ramMB)*100))))
	}
	b.WriteString(dimSt.Render(fmt.Sprintf("bars: CPU = share of all %d cores, MEM = share of RAM", max(m.ncpu, 1))) + "\n")
	return b.String()
}

// sparks returns the CPU and memory trend of one app; empty on narrow terminals.
func (m model) sparks(app string) (cpuSp, memSp string) {
	if m.w < 150 {
		return "", ""
	}
	st := lipgloss.NewStyle().Foreground(appColors[app])
	cpu, mem := m.hist[app+"/cpu"], m.hist[app+"/mem"]
	return " " + st.Render(spark(cpu, max(100, maxOf(cpu)), histLen)), " " + st.Render(spark(shift(mem), maxOf(shift(mem)), histLen))
}

// shift subtracts the minimum so a memory trend shows its changes, not its level.
func shift(v []float64) []float64 {
	low := 0.0
	for i, x := range v {
		if i == 0 || x < low {
			low = x
		}
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x - low
	}
	return out
}

func maxOf(v []float64) float64 {
	top := 0.0
	for _, x := range v {
		top = max(top, x)
	}
	return top
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// rowName builds the tree label: ▸/▾ for groups, indent and PID for children.
func rowName(r Row) string {
	switch {
	case r.IsGroup && r.Expanded:
		return fmt.Sprintf("▾ %s (%d)", r.Name, r.Count)
	case r.IsGroup:
		return fmt.Sprintf("▸ %s (%d)", r.Name, r.Count)
	case r.Parent != "":
		return fmt.Sprintf("    PID %d", r.PID)
	}
	return fmt.Sprintf("  %s  PID %d", clip(r.Name, nameWidth-12), r.PID)
}

// renderRow draws one line. Row bars are relative to the biggest row on screen.
func (m model) renderRow(r Row, selected bool, cpuMax, memMax float64) string {
	name := clip(rowName(r), nameWidth)
	desc := clip(r.Desc, max(m.w-95, 10))
	cpu, mem, age := fmt.Sprintf("%.1f%%", r.CPU), fmtMem(r.MemMB), fmtAge(r.Age)
	ageStyle := ageSt(r.Age)
	if r.Age < newAge {
		age, ageStyle = "✦ "+age, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e4ff00"))
	}
	cf, mf := safeDiv(r.CPU, cpuMax), safeDiv(r.MemMB, memMax)
	if selected { // reverse video: plain text keeps the highlight readable
		cFill, cTrack := bar(cf, barWidth)
		mFill, mTrack := bar(mf, barWidth)
		return selSt.Render(fmt.Sprintf("%-*s %-8s %7s %s %9s %s  %-11s  %s", nameWidth, name, r.App,
			cpu, cFill+cTrack, mem, mFill+mTrack, age, desc))
	}
	nameSt := lipgloss.NewStyle().Foreground(appColors[r.App])
	switch {
	case r.IsGroup:
		nameSt = nameSt.Bold(true)
	case r.Parent != "":
		nameSt = dimSt
	}
	return cell(nameSt, name, nameWidth, false) + " " + cell(dimSt, r.App, 8, false) + " " +
		cell(cpuSt(r.CPU), cpu, 7, true) + " " + barCell(cpuSt(r.CPU), cf) + " " +
		cell(memSt(r.MemMB), mem, 9, true) + " " + barCell(memSt(r.MemMB), mf) + "  " +
		cell(ageStyle, age, 11, false) + "  " + dimSt.Render(desc)
}

func (m model) heartbeat() string {
	glyph := "○"
	if m.beat {
		glyph = "●"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#e4ff00")).Render(glyph)
}

func (m model) View() string {
	filter := "all"
	if m.filter > 0 {
		filter = appNames[m.filter-1]
	}
	var b strings.Builder
	b.WriteString(gradientText("aiwatch", rgb{255, 140, 60}, rgb{60, 220, 255}) + " " + m.heartbeat() + dimSt.Render(fmt.Sprintf("  sort:%s  filter:%s  every %s", sortNames[m.sortKey], filter, refreshEvery)) + "\n\n")
	b.WriteString(m.summary() + "\n")
	b.WriteString(boldSt.Render(fmt.Sprintf("%-*s %-8s %7s %*s %9s %*s  %-11s  %s", nameWidth, "NAME", "APP", "CPU", barWidth, "", "MEM", barWidth, "", "AGE", "WHAT IT IS")) + "\n")
	cpuMax, memMax := 100.0, 0.0 // 100% = one full core is the minimum CPU scale
	for _, r := range m.view {
		cpuMax, memMax = max(cpuMax, r.CPU), max(memMax, r.MemMB)
	}

	rows := max(m.h-16, 5)
	start := max(m.cursor-rows+1, 0)
	for i := start; i < len(m.view) && i < start+rows; i++ {
		b.WriteString(m.renderRow(m.view[i], i == m.cursor, cpuMax, memMax) + "\n")
	}

	b.WriteString("\n")
	if r := m.currentRow(); r != nil {
		b.WriteString(dimSt.Render(clip("path: "+r.Path, max(m.w, 40))) + "\n")
	}
	if c := m.confirm; c != nil {
		b.WriteString(warnSt.Render(fmt.Sprintf("Send %v to %s? [y/N]", c.sig, c.row.label())))
	} else {
		b.WriteString(dimSt.Render("↑↓ move  →/enter open  ← close  s sort  f filter  k SIGTERM  K SIGKILL  q quit  ") + m.status)
	}
	return b.String() + "\n"
}

func main() {
	if _, err := tea.NewProgram(model{ramMB: installedRAMMB(), ncpu: runtime.NumCPU()}, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// installedRAMMB reads hw.memsize; 0 means unknown and the bar is scaled to the apps' own total.
func installedRAMMB() float64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	bytes, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return bytes / (1024 * 1024)
}
