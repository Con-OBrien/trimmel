//go:build windows

package mover

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// Leftovers lists folders under "<drive>\Moved Programs" that no installed
// program links to: the remains of a failed, interrupted or undone move.
func Leftovers() ([]Leftover, error) {
	linked := linkedTargets()
	var out []Leftover
	for _, hold := range holdFolders() {
		entries, err := os.ReadDir(hold)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(hold, e.Name())
			if !e.IsDir() || linked[strings.ToLower(p)] {
				continue
			}
			size, _, _ := dirStats(p)
			out = append(out, Leftover{Path: p, Drive: filepath.VolumeName(p), Size: size})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out, nil
}

// DeleteLeftover removes one leftover folder. It must sit directly inside a
// "Moved Programs" folder and must not be linked from any installed program.
func DeleteLeftover(path string) error {
	if !busy.TryLock() {
		return errBusy
	}
	defer busy.Unlock()

	path = filepath.Clean(path)
	if !strings.EqualFold(filepath.Base(filepath.Dir(path)), holdFolder) {
		return errors.New("that isn't a leftover copy")
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.IsDir() || isLink(fi) {
		return errors.New("that folder no longer exists")
	}
	if linkedTargets()[strings.ToLower(path)] {
		return errors.New("an installed program still uses that folder")
	}
	if procs := runningFrom(path); len(procs) > 0 {
		return fmt.Errorf("close these first: %s", strings.Join(procs, ", "))
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("couldn't delete everything: %w", err)
	}
	os.Remove(filepath.Dir(path)) // the "Moved Programs" folder, only if now empty
	return nil
}

func linkedTargets() map[string]bool {
	t := map[string]bool{}
	for _, in := range enumerate() {
		if in.MovedTo != "" {
			t[strings.ToLower(in.MovedTo)] = true
		}
	}
	return t
}

func holdFolders() []string {
	parts, _ := disk.Partitions(false)
	var out []string
	for _, p := range parts {
		out = append(out, filepath.Join(p.Mountpoint+`\`, holdFolder))
	}
	return out
}
