// Package advisor looks at one drive and says what the user could do about
// the space on it: run Cleanup, move or uninstall a big program, look through
// a pile of personal files, or handle a known Windows space hog. It only
// suggests; each suggestion points at the tab or folder where the user acts.
package advisor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"trimmel/internal/cleanup"
	"trimmel/internal/mover"
	"trimmel/internal/oldver"
	"trimmel/internal/storage"
)

const (
	minCleanup   = 100 << 20 // not worth a suggestion below this
	minApp       = 1 << 30
	minUninstall = 3 << 30
	minFolder    = 1 << 30
	minCache     = 500 << 20
	minSystem    = 2 << 30
	maxResults   = 10
	sizeBudget   = 25 * time.Second
)

type Suggestion struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Size   uint64 `json:"size"`   // space it could free or free up
	Action string `json:"action"` // "cleanup" | "move" | "uninstall" | "review" | "manual"
	View   string `json:"view"`   // tab to open, "" if none
	Path   string `json:"path"`   // folder to look through, "" if none
	Dest   string `json:"dest"`   // suggested drive for "move"
	Safety string `json:"safety"` // "safe" | "review" | "careful"
}

type Drive struct {
	Mount string // "D:"
	Free  uint64
}

type Options struct {
	Drive   string  // the drive being analysed, "C:"
	Others  []Drive // other drives, for "move" suggestions
	IsAdmin bool
}

// Suggest returns up to ten suggestions for opts.Drive, biggest first.
func Suggest(o Options) []Suggestion {
	var (
		mu  sync.Mutex
		out []Suggestion
		wg  sync.WaitGroup
	)
	add := func(s ...Suggestion) {
		mu.Lock()
		out = append(out, s...)
		mu.Unlock()
	}
	run := func(fn func() []Suggestion) {
		wg.Add(1)
		go func() { defer wg.Done(); add(fn()...) }()
	}

	run(func() []Suggestion { return cleanupSuggestions(o) })
	run(func() []Suggestion { return appSuggestions(o) })
	run(func() []Suggestion { return folderSuggestions(o) })
	run(func() []Suggestion { return systemSuggestions(o) })
	wg.Wait()

	return rank(out)
}

// rank sorts biggest first and keeps the top maxResults.
func rank(in []Suggestion) []Suggestion {
	sort.SliceStable(in, func(i, j int) bool { return in[i].Size > in[j].Size })
	if len(in) > maxResults {
		in = in[:maxResults]
	}
	return in
}

func systemDrive() string {
	if d := os.Getenv("SystemDrive"); d != "" {
		return strings.ToUpper(d)
	}
	return "C:"
}

func onDrive(path, drive string) bool {
	return strings.EqualFold(filepath.VolumeName(path), drive)
}

// ---- Cleanup ----

func cleanupSuggestions(o Options) []Suggestion {
	if !strings.EqualFold(o.Drive, systemDrive()) {
		return nil // Cleanup targets live on the system drive
	}
	var total uint64
	var parts []cleanup.TargetReport
	var bin *cleanup.TargetReport
	for _, r := range cleanup.Scan(o.IsAdmin) {
		r := r
		if !r.Available || r.Bytes == 0 {
			continue
		}
		if r.ID == "recycle-bin" {
			bin = &r
			continue
		}
		total += r.Bytes
		parts = append(parts, r)
	}
	var out []Suggestion
	if total >= minCleanup {
		sort.Slice(parts, func(i, j int) bool { return parts[i].Bytes > parts[j].Bytes })
		var names []string
		for i, p := range parts {
			if i == 3 {
				break
			}
			names = append(names, fmt.Sprintf("%s %s", p.Name, HumanBytes(p.Bytes)))
		}
		out = append(out, Suggestion{
			ID: "cleanup", Title: "Run Cleanup", Size: total, Action: "cleanup", View: "cleanup", Safety: "safe",
			Detail: "Temporary files and caches Windows and apps rebuild when needed: " + strings.Join(names, ", ") + ".",
		})
	}
	if bin != nil && bin.Bytes >= minCleanup {
		out = append(out, Suggestion{
			ID: "recycle-bin", Title: "Empty the Recycle Bin", Size: bin.Bytes, Action: "cleanup", View: "cleanup", Safety: "careful",
			Detail: "Files you deleted earlier, across all drives. Emptying it can't be undone, so check it first.",
		})
	}
	return out
}

// ---- Programs ----

func appSuggestions(o Options) []Suggestion {
	var out []Suggestion
	moved := map[string]bool{}

	if installs, err := mover.Scan(); err == nil {
		n := 0
		for _, in := range installs {
			if n == 5 {
				break
			}
			if !strings.EqualFold(in.Drive, o.Drive) || in.MovedTo != "" || !in.Movable || in.Size < minApp {
				continue
			}
			dest := bestDest(o.Others, in.Size)
			if dest == "" {
				continue
			}
			n++
			moved[strings.ToLower(in.Name)] = true
			out = append(out, Suggestion{
				ID: "move:" + in.Path, Title: fmt.Sprintf("Move %s to %s", in.Name, dest), Size: in.Size,
				Action: "move", View: "move", Path: in.Path, Dest: dest, Safety: "safe",
				Detail: fmt.Sprintf("Uses %s on %s. A link is left behind so it keeps working, and you can move it back.", HumanBytes(in.Size), o.Drive),
			})
		}
	}

	apps := oldver.ListApps()
	n := 0
	for _, a := range apps { // already largest first
		if n == 3 {
			break
		}
		if a.Shared || a.Size < minUninstall || moved[strings.ToLower(a.Name)] {
			continue
		}
		if a.Drive != "" && !strings.EqualFold(a.Drive, o.Drive) {
			continue
		}
		if a.Drive == "" && !strings.EqualFold(o.Drive, systemDrive()) {
			continue
		}
		n++
		out = append(out, Suggestion{
			ID: "uninstall:" + a.ID, Title: fmt.Sprintf("Uninstall %s if you no longer use it", a.Name), Size: a.Size,
			Action: "uninstall", View: "apps", Safety: "review",
			Detail: fmt.Sprintf("Takes %s. Worth removing only if you've stopped using it; its own uninstaller asks you to confirm.", HumanBytes(a.Size)),
		})
	}
	return out
}

// bestDest picks the other drive with room for size bytes that can safely
// hold a moved program, or "".
func bestDest(others []Drive, size uint64) string {
	best, bestFree := "", uint64(0)
	for _, d := range others {
		if d.Free < size+size/20+(100<<20) || d.Free <= bestFree {
			continue
		}
		if mover.CheckDest(d.Mount+`\`) != nil {
			continue
		}
		best, bestFree = d.Mount, d.Free
	}
	return best
}

// ---- Personal files and developer caches ----

type folderRule struct {
	id, title, detail string
	path              func(profile, local string) string
	min               uint64
	action, safety    string
}

var folderRules = []folderRule{
	{"downloads", "Look through your Downloads", "Old installers, ISOs and zip files pile up here. Open the biggest ones and delete what you don't need.",
		func(p, _ string) string { return filepath.Join(p, "Downloads") }, minFolder, "review", "review"},
	{"videos", "Look through your Videos", "Video files are usually the biggest personal files. Delete or move the ones you no longer watch.",
		func(p, _ string) string { return filepath.Join(p, "Videos") }, minFolder, "review", "review"},
	{"pictures", "Look through your Pictures", "Photos and screenshots add up. Back them up, then move or delete the extras.",
		func(p, _ string) string { return filepath.Join(p, "Pictures") }, minFolder, "review", "review"},
	{"documents", "Look through your Documents", "Check for large files you no longer need, such as old project exports and disk images.",
		func(p, _ string) string { return filepath.Join(p, "Documents") }, minFolder, "review", "review"},
	{"desktop", "Look through your Desktop", "Files on the Desktop count against this drive too.",
		func(p, _ string) string { return filepath.Join(p, "Desktop") }, minFolder, "review", "review"},

	{"nuget", "Developer cache: NuGet packages", "Downloaded again by Visual Studio and dotnet when needed. Close them, then delete the folder.",
		func(p, _ string) string { return filepath.Join(p, ".nuget", "packages") }, minCache, "review", "safe"},
	{"gradle", "Developer cache: Gradle", "Rebuilt on the next build. Close your IDE, then delete the folder.",
		func(p, _ string) string { return filepath.Join(p, ".gradle", "caches") }, minCache, "review", "safe"},
	{"maven", "Developer cache: Maven repository", "Downloaded again by the next build.",
		func(p, _ string) string { return filepath.Join(p, ".m2", "repository") }, minCache, "review", "safe"},
	{"cargo", "Developer cache: Rust crates", "Downloaded again by cargo when needed.",
		func(p, _ string) string { return filepath.Join(p, ".cargo", "registry") }, minCache, "review", "safe"},
	{"npm", "Developer cache: npm", "Run `npm cache clean --force`, or delete the folder. npm downloads packages again.",
		func(_, l string) string { return filepath.Join(l, "npm-cache") }, minCache, "review", "safe"},
	{"pip", "Developer cache: pip", "Run `pip cache purge`. Packages download again when needed.",
		func(_, l string) string { return filepath.Join(l, "pip", "Cache") }, minCache, "review", "safe"},
	{"yarn", "Developer cache: Yarn", "Run `yarn cache clean`. Packages download again when needed.",
		func(_, l string) string { return filepath.Join(l, "Yarn", "Cache") }, minCache, "review", "safe"},
	{"docker", "Docker data", "Images and containers Docker keeps. Use `docker system prune` inside Docker to remove unused ones.",
		func(_, l string) string { return filepath.Join(l, "Docker") }, minFolder, "review", "careful"},
}

func folderSuggestions(o Options) []Suggestion {
	profile, local := os.Getenv("USERPROFILE"), os.Getenv("LOCALAPPDATA")
	if profile == "" {
		return nil
	}
	var (
		mu  sync.Mutex
		out []Suggestion
		wg  sync.WaitGroup
	)
	for _, r := range folderRules {
		p := r.path(profile, local)
		if !onDrive(p, o.Drive) {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		wg.Add(1)
		go func(r folderRule, p string) {
			defer wg.Done()
			size, _ := storage.SizeOf(p, sizeBudget)
			if size < r.min {
				return
			}
			detail := r.detail
			if r.safety == "review" {
				if d := bestDest(o.Others, size); d != "" {
					detail += fmt.Sprintf(" You could also move it to %s.", d)
				}
			}
			mu.Lock()
			out = append(out, Suggestion{
				ID: r.id, Title: r.title, Detail: detail, Size: size,
				Action: r.action, Path: p, Safety: r.safety,
			})
			mu.Unlock()
		}(r, p)
	}
	wg.Wait()
	return out
}

// ---- Known Windows space hogs ----

func systemSuggestions(o Options) []Suggestion {
	sys := systemDrive()
	if !strings.EqualFold(o.Drive, sys) {
		return nil
	}
	var out []Suggestion
	root := sys + `\`

	if fi, err := os.Stat(filepath.Join(root, "hiberfil.sys")); err == nil && uint64(fi.Size()) >= minSystem {
		out = append(out, Suggestion{
			ID: "hiberfil", Title: "Hibernation file", Size: uint64(fi.Size()), Action: "manual", Safety: "careful",
			Detail: "Windows reserves this space for hibernate and Fast Startup. If you never hibernate, run `powercfg /h off` in an administrator terminal to free it. This also turns off Fast Startup.",
		})
	}
	for _, name := range []string{"Windows.old", "$Windows.~BT"} {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		size, _ := storage.SizeOf(p, sizeBudget)
		if size < minCleanup {
			continue
		}
		out = append(out, Suggestion{
			ID: strings.ToLower(name), Title: "Leftover from a Windows upgrade", Size: size, Action: "manual", Path: p, Safety: "careful",
			Detail: "Remove it with Windows itself: Settings, System, Storage, Temporary files, then tick \"Previous Windows installation(s)\". Windows also deletes it automatically after about 10 days.",
		})
	}
	return out
}

// HumanBytes formats a size like "1.4 GB".
func HumanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
