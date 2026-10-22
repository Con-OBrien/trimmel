package storage

import (
	"path/filepath"
	"strings"
	"time"
)

// Describe returns a short plain-English note for well-known Windows folders
// and files, so a drive listing explains itself without drilling down.
// It returns "" for anything it doesn't recognise.
func Describe(path string) string {
	name := strings.ToLower(filepath.Base(path))
	parent := strings.ToLower(filepath.Base(filepath.Dir(path)))

	// Context-dependent names first.
	if parent == "appdata" {
		switch name {
		case "local":
			return "App data and caches for this PC. Cleanup clears the safe parts."
		case "roaming":
			return "App settings that follow your account. Leave alone."
		case "locallow":
			return "Low-privilege app data. Leave alone."
		}
	}

	switch name {
	case "windows":
		return "Windows itself. Don't delete anything here."
	case "program files", "program files (x86)":
		return "Installed programs. Use Apps or Move apps to free space."
	case "programdata":
		return "Shared app data. Safe to leave."
	case "users":
		return "User accounts: documents, downloads and app data."
	case "$recycle.bin":
		return "The Recycle Bin. Emptying it (in Cleanup) frees this space."
	case "windows.old":
		return "A previous Windows version. Windows can remove it for you (see suggestions)."
	case "$windows.~bt", "$windows.~ws":
		return "Leftover Windows upgrade files. Windows' Disk Cleanup removes them."
	case "system volume information":
		return "System restore data, managed by Windows."
	case "pagefile.sys":
		return "Windows virtual memory. Needed; leave alone."
	case "hiberfil.sys":
		return "Hibernation file. Only needed for hibernate and Fast Startup."
	case "swapfile.sys":
		return "Windows app memory file. Leave alone."
	case "appdata":
		return "App settings and caches. Don't delete this folder."
	case "temp", "tmp":
		return "Temporary files. Cleanup removes old ones safely."
	case "downloads":
		return "Your downloads. Old installers often pile up here."
	case "videos", "pictures", "music", "documents", "desktop":
		return "Your own files."
	case "onedrive":
		return "Synced OneDrive files. Use OneDrive's \"Free up space\" to keep them online only."
	case "steamapps":
		return "Steam games. Steam can move them to another drive in its storage settings."
	case "node_modules":
		return "Project dependencies. They come back with npm install."
	case ".nuget", ".gradle", ".m2", ".cargo", ".npm":
		return "Developer tool cache. Downloaded again when needed."
	case "docker", ".docker":
		return "Docker data. Prune unused images inside Docker to free space."
	case "packages":
		if parent == "local" {
			return "Microsoft Store app data."
		}
	}
	return ""
}

// SizeOf measures one folder (or file) within budget. ok is false if the
// budget ran out, in which case the size is a lower bound.
func SizeOf(path string, budget time.Duration) (size uint64, ok bool) {
	ctx, cancel := contextWithBudget(budget)
	defer cancel()
	size, _, _ = measure(ctx, path)
	return size, ctx.Err() == nil
}
