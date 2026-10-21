//go:build windows

package cleanup

import (
	"os"
	"path/filepath"
	"time"
)

const day = 24 * time.Hour

func targets() []target {
	local := os.Getenv("LOCALAPPDATA")
	winDir := os.Getenv("SystemRoot")
	if winDir == "" {
		winDir = `C:\Windows`
	}

	chromium := func(base string) func() []string {
		return func() []string {
			profiles := filepath.Join(local, base, "User Data", "*")
			return globAll(
				filepath.Join(profiles, "Cache"),
				filepath.Join(profiles, "Code Cache"),
				filepath.Join(profiles, "GPUCache"),
			)
		}
	}

	return []target{
		{
			ID: "user-temp", Name: "Your temp files",
			Description: "Leftovers from installers and apps in %TEMP%. Files from the last 24 hours are kept.",
			MinAge:      day,
			Roots:       func() []string { return []string{os.TempDir()} },
		},
		{
			ID: "windows-temp", Name: "Windows temp files",
			Description:   "System-wide temp folder. Files from the last 24 hours are kept.",
			RequiresAdmin: true, MinAge: day,
			Roots: func() []string { return []string{filepath.Join(winDir, "Temp")} },
		},
		{
			ID: "wu-cache", Name: "Windows Update downloads",
			Description:   "Update packages that are already installed. Windows re-downloads anything it still needs.",
			RequiresAdmin: true, MinAge: day,
			Roots: func() []string { return []string{filepath.Join(winDir, "SoftwareDistribution", "Download")} },
		},
		{
			ID: "chrome-cache", Name: "Chrome cache",
			Description: "Cached web content for all Chrome profiles. Close Chrome first for best results.",
			Roots:       chromium(filepath.Join("Google", "Chrome")),
		},
		{
			ID: "edge-cache", Name: "Edge cache",
			Description: "Cached web content for all Edge profiles. Close Edge first for best results.",
			Roots:       chromium(filepath.Join("Microsoft", "Edge")),
		},
		{
			ID: "firefox-cache", Name: "Firefox cache",
			Description: "Cached web content for all Firefox profiles.",
			Roots: func() []string {
				return globAll(filepath.Join(local, "Mozilla", "Firefox", "Profiles", "*", "cache2"))
			},
		},
		{
			ID: "crash-dumps", Name: "Crash dumps",
			Description: "Memory dumps written when apps crash. Only useful for debugging those crashes.",
			Roots:       func() []string { return []string{filepath.Join(local, "CrashDumps")} },
		},
		{
			ID: "recycle-bin", Name: "Recycle Bin",
			Description: "Empties the Recycle Bin on all drives. This cannot be undone.",
			ScanFn:      queryRecycleBin,
			CleanFn:     emptyRecycleBin,
		},
	}
}
