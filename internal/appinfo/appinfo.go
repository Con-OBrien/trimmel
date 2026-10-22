// Package appinfo reports where Trimmel is installed and whether a newer
// release exists. It backs the Settings tab.
package appinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"trimmel/internal/appdir"
)

const (
	// SiteURL is the public download page; releases are described by
	// <SiteURL>/downloads/version.json.
	SiteURL    = "https://www.trimmel.app"
	versionURL = SiteURL + "/downloads/version.json"
)

// Install describes how the running copy was installed.
type Install struct {
	Installed    bool   `json:"installed"`    // run from a folder registered by setup
	Dir          string `json:"dir"`          // folder of the running exe
	Exe          string `json:"exe"`          // full path of the running exe
	HasUninstall bool   `json:"hasUninstall"` // setup's uninstall.exe is present
	SettingsDir  string `json:"settingsDir"`
}

// Update is the result of a release check.
type Update struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Date      string `json:"date"`
	URL       string `json:"url"`
}

// SettingsDir is the per-user folder where the app keeps its settings.
func SettingsDir() string { return appdir.Dir() }

// LegacySettingsDir is the folder PC Optimizer 0.1.x used. It is kept after
// migration, so "delete my settings" has to remove it too.
func LegacySettingsDir() string { return appdir.LegacyDir() }

// Current describes the running copy.
func Current() (Install, error) {
	exe, err := os.Executable()
	if err != nil {
		return Install{}, err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	in := Install{Dir: filepath.Dir(exe), Exe: exe, SettingsDir: SettingsDir()}
	if reg := registeredDir(); reg != "" && strings.EqualFold(filepath.Clean(reg), in.Dir) {
		in.Installed = true
		_, err := os.Stat(UninstallerPath(in.Dir))
		in.HasUninstall = err == nil
	}
	return in, nil
}

// UninstallerPath is setup's copy of itself inside the install folder.
func UninstallerPath(dir string) string { return filepath.Join(dir, "uninstall.exe") }

// CheckUpdate compares current with the latest published release.
func CheckUpdate(ctx context.Context, current string) (Update, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if err != nil {
		return Update{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Update{}, fmt.Errorf("couldn't reach the update server. Check your connection")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Update{}, fmt.Errorf("the update server answered %s", resp.Status)
	}
	var v struct {
		Version string `json:"version"`
		Date    string `json:"date"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&v); err != nil || v.Version == "" {
		return Update{}, fmt.Errorf("the update information was not readable")
	}
	return Update{
		Current:   current,
		Latest:    v.Version,
		Available: len(parts(current)) > 0 && newer(v.Version, current),
		Date:      v.Date,
		URL:       SiteURL + "/#download",
	}, nil
}

// newer reports whether version a is later than b (dotted numbers, e.g. 0.2.1).
func newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parts(v string) []int {
	var out []int
	for _, p := range strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
