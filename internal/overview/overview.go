// Package overview collects a one-shot snapshot of system health.
package overview

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type Snapshot struct {
	Hostname      string   `json:"hostname"`
	OS            string   `json:"os"`
	UptimeSeconds uint64   `json:"uptimeSeconds"`
	CPU           CPUInfo  `json:"cpu"`
	Memory        MemInfo  `json:"memory"`
	Disks         []Disk   `json:"disks"`
	Warnings      []string `json:"warnings"` // non-fatal collection errors
}

type CPUInfo struct {
	Model        string  `json:"model"`
	Cores        int     `json:"cores"`
	Threads      int     `json:"threads"`
	UsagePercent float64 `json:"usagePercent"`
}

type MemInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"usedPercent"`
}

type Disk struct {
	Mount       string  `json:"mount"`
	FSType      string  `json:"fsType"`
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
}

// Collect gathers the snapshot. It blocks ~300ms to sample CPU usage.
// Individual failures are recorded in Warnings rather than failing the call.
func Collect() (Snapshot, error) {
	var s Snapshot

	if h, err := host.Info(); err == nil {
		s.Hostname = h.Hostname
		s.OS = h.Platform + " " + h.PlatformVersion
		s.UptimeSeconds = h.Uptime
	} else {
		s.Warnings = append(s.Warnings, "host: "+err.Error())
	}

	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		s.CPU.Model = infos[0].ModelName
	}
	s.CPU.Cores, _ = cpu.Counts(false)
	s.CPU.Threads, _ = cpu.Counts(true)
	if pct, err := cpu.Percent(300*time.Millisecond, false); err == nil && len(pct) > 0 {
		s.CPU.UsagePercent = pct[0]
	} else if err != nil {
		s.Warnings = append(s.Warnings, "cpu: "+err.Error())
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		s.Memory = MemInfo{Total: vm.Total, Used: vm.Used, Available: vm.Available, UsedPercent: vm.UsedPercent}
	} else {
		s.Warnings = append(s.Warnings, "memory: "+err.Error())
	}

	parts, err := disk.Partitions(false)
	if err != nil {
		s.Warnings = append(s.Warnings, "disks: "+err.Error())
	}
	for _, p := range parts {
		u, err := disk.Usage(p.Mountpoint)
		if err != nil || u.Total == 0 {
			continue // empty card readers, unmounted optical drives
		}
		s.Disks = append(s.Disks, Disk{
			Mount: p.Mountpoint, FSType: p.Fstype,
			Total: u.Total, Free: u.Free, UsedPercent: u.UsedPercent,
		})
	}

	return s, nil
}
