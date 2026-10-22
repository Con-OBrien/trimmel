// Package procs lists running processes with live CPU and memory usage.
package procs

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
)

type Process struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"` // share of total CPU, 0-100 like Task Manager
	MemoryRSS  uint64  `json:"memoryRss"`
	Protected  bool    `json:"protected"`
	RelatedTo  string  `json:"relatedTo"` // plain-English explanation of what it belongs to
	Vendor     string  `json:"vendor"`
	Path       string  `json:"path"`
}

// Processes that would crash or log out the session if killed.
var critical = map[string]bool{
	"system": true, "system idle process": true, "registry": true, "smss.exe": true,
	"csrss.exe": true, "wininit.exe": true, "winlogon.exe": true, "services.exe": true,
	"lsass.exe": true, "memory compression": true, "secure system": true, "dwm.exe": true,
	// Desktop shell: ending any of these takes out the taskbar, Start menu or input.
	"explorer.exe": true, "sihost.exe": true, "shellhost.exe": true, "ctfmon.exe": true,
	"startmenuexperiencehost.exe": true, "shellexperiencehost.exe": true, "searchhost.exe": true,
	"textinputhost.exe": true, "taskhostw.exe": true, "runtimebroker.exe": true,
	"svchost.exe": true, "fontdrvhost.exe": true, "audiodg.exe": true,
}

// Monitor keeps gopsutil Process handles between calls. Percent(0) measures
// CPU time since the previous call on the same handle, so reusing handles is
// what turns successive List calls into live usage figures.
type Monitor struct {
	mu      sync.Mutex
	handles map[int32]*process.Process
	meta    map[int32]meta  // static per-process info, looked up once
	vinfo   map[string]meta // version info per exe path
	threads float64
}

func NewMonitor() *Monitor {
	n, err := cpu.Counts(true)
	if err != nil || n < 1 {
		n = 1
	}
	return &Monitor{handles: map[int32]*process.Process{}, meta: map[int32]meta{}, vinfo: map[string]meta{}, threads: float64(n)}
}

func (m *Monitor) List() ([]Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pids, err := process.Pids()
	if err != nil {
		return nil, err
	}

	alive := make(map[int32]*process.Process, len(pids))
	alivePID := make(map[int32]meta, len(pids))
	out := make([]Process, 0, len(pids))
	self := int32(os.Getpid())

	for _, pid := range pids {
		if pid == 0 {
			continue
		}
		h, ok := m.handles[pid]
		if !ok {
			h, err = process.NewProcess(pid)
			if err != nil {
				continue // exited between Pids and NewProcess
			}
		}
		alive[pid] = h

		name, err := h.Name()
		if err != nil || name == "" {
			continue // usually access denied on protected processes
		}
		cpuPct, _ := h.Percent(0) // 0 on first sight, real value afterwards
		var rss uint64
		if mi, err := h.MemoryInfo(); err == nil {
			rss = mi.RSS
		}

		info, ok := m.meta[pid]
		if !ok {
			info = m.lookup(h, name)
		}
		alivePID[pid] = info

		out = append(out, Process{
			PID:        pid,
			Name:       name,
			CPUPercent: cpuPct / m.threads,
			MemoryRSS:  rss,
			Protected:  pid == self || pid <= 4 || critical[strings.ToLower(name)] || ownChild(h, name, self),
			RelatedTo:  info.relatedTo,
			Vendor:     info.vendor,
			Path:       info.path,
		})
	}

	m.handles = alive // drop handles for exited processes
	m.meta = alivePID
	return out, nil
}

// ownChild reports whether h is one of this app's own WebView2 helper
// processes. Ending one blanks the window.
func ownChild(h *process.Process, name string, self int32) bool {
	if !strings.EqualFold(name, "msedgewebview2.exe") {
		return false
	}
	cur := h
	for depth := 0; depth < 6; depth++ {
		ppid, err := cur.Ppid()
		if err != nil || ppid <= 0 {
			return false
		}
		if ppid == self {
			return true
		}
		if cur, err = process.NewProcess(ppid); err != nil {
			return false
		}
	}
	return false
}

// Kill ends a process. expectName is the name the user saw in the list; if
// the PID has since been reused by a different program, nothing is ended.
func (m *Monitor) Kill(pid int32, expectName string) error {
	self := int32(os.Getpid())
	if pid == self || pid <= 4 {
		return fmt.Errorf("refusing to end protected process %d", pid)
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found", pid)
	}
	name, err := p.Name()
	if err != nil {
		return fmt.Errorf("could not identify process %d, so it was left alone", pid)
	}
	if !strings.EqualFold(name, expectName) {
		return fmt.Errorf("process %d is now %s, not %s. The list was out of date, so nothing was ended", pid, name, expectName)
	}
	if critical[strings.ToLower(name)] {
		return fmt.Errorf("refusing to end %s: Windows depends on it", name)
	}
	if ownChild(p, name, self) {
		return fmt.Errorf("refusing to end %s: Trimmel's own window depends on it", name)
	}
	if err := p.Kill(); err != nil {
		return fmt.Errorf("could not end process %d: %w (it may need admin rights)", pid, err)
	}
	return nil
}

type meta struct{ relatedTo, vendor, path string }

// lookup resolves what a process belongs to: a curated explanation if we
// have one, else the exe's own description. Results are cached by exe path.
func (m *Monitor) lookup(h *process.Process, name string) meta {
	path, _ := h.Exe() // empty for protected processes
	vi, ok := m.vinfo[path]
	if !ok {
		if path != "" {
			desc, vendor := readVersionInfo(path)
			vi = meta{relatedTo: desc, vendor: vendor, path: path}
		}
		m.vinfo[path] = vi
	}
	out := meta{relatedTo: vi.relatedTo, vendor: vi.vendor, path: path}
	if k, ok := known[strings.ToLower(name)]; ok {
		out.relatedTo = k
	}
	return out
}
