//go:build windows

package oldver

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ListApps returns every program that has an uninstaller, largest first.
// Updates, hotfixes and system components are left out, as are our own entry.
func ListApps() []App {
	entries := read()
	var apps []App
	locs := map[string]string{}
	for _, e := range entries {
		if e.hasParent || e.uninstall == "" || strings.EqualFold(e.name, "PC Optimizer") || isUpdateName(e.name) {
			continue
		}
		apps = append(apps, App{
			ID: e.id, Name: e.name, Version: e.version, Publisher: e.publisher, Size: e.size,
			InstallDate: formatDate(e.date), Shared: isShared(e),
		})
		locs[e.id] = e.location
		apps[len(apps)-1].Drive = filepath.VolumeName(e.location)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range apps {
		loc := locs[apps[i].ID]
		if apps[i].Size != 0 || loc == "" || !measurable(loc) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(a *App, dir string) {
			defer wg.Done()
			defer func() { <-sem }()
			a.Size = dirSize(dir)
		}(&apps[i], loc)
	}
	wg.Wait()

	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Size != apps[j].Size {
			return apps[i].Size > apps[j].Size
		}
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps
}

// UninstallApp runs the program's own uninstaller. The command comes from the
// registry entry for id, never from the caller.
func UninstallApp(id string) error {
	for _, e := range read() {
		if e.id == id && e.uninstall != "" && !e.hasParent {
			target := e
			return launch(&target)
		}
	}
	return errors.New("that program is no longer listed")
}

func isShared(e entry) bool {
	ln := strings.ToLower(e.name)
	if strings.Contains(strings.ToLower(e.publisher), "microsoft") {
		return true
	}
	for _, x := range excluded {
		if strings.Contains(ln, x) {
			return true
		}
	}
	return false
}

func isUpdateName(name string) bool {
	ln := strings.ToLower(name)
	return strings.HasPrefix(ln, "update for ") || strings.HasPrefix(ln, "security update") ||
		strings.HasPrefix(ln, "hotfix") || strings.Contains(ln, "(kb")
}

// formatDate turns the registry's YYYYMMDD into YYYY-MM-DD, or "" if it isn't one.
func formatDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return ""
	}
	t, err := time.Parse("20060102", s)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}
