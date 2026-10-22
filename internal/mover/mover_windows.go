//go:build windows

package mover

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	minListSize = 50 << 20 // hide installs smaller than 50 MB
	holdFolder  = "Moved Programs"
	oldSuffix   = ".pcopt-old"
)

var busy sync.Mutex

// ---- Scanning ----

var uninstallKeys = []struct {
	root registry.Key
	path string
}{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// enumerate lists installed programs with an install folder, without sizes.
func enumerate() []Install {
	seen := map[string]bool{}
	var found []Install

	for _, k := range uninstallKeys {
		key, err := registry.OpenKey(k.root, k.path, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, _ := key.ReadSubKeyNames(-1)
		key.Close()
		for _, n := range names {
			sub, err := registry.OpenKey(k.root, k.path+`\`+n, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			name, _, _ := sub.GetStringValue("DisplayName")
			loc, _, _ := sub.GetStringValue("InstallLocation")
			pub, _, _ := sub.GetStringValue("Publisher")
			sysComp, _, _ := sub.GetIntegerValue("SystemComponent")
			sub.Close()

			loc = strings.Trim(strings.TrimSpace(loc), `"`)
			if name == "" || loc == "" || !filepath.IsAbs(loc) {
				continue
			}
			loc = filepath.Clean(loc)
			id := strings.ToLower(loc)
			if seen[id] {
				continue
			}
			fi, err := os.Lstat(loc)
			if err != nil || !(fi.IsDir() || isLink(fi)) {
				continue
			}
			seen[id] = true

			in := Install{Name: name, Publisher: pub, Path: loc, Drive: filepath.VolumeName(loc), Movable: true}
			if isLink(fi) {
				in.MovedTo = linkTarget(loc)
				if _, err := os.Stat(loc); err != nil {
					in.Broken = true // the drive holding the program is gone
				}
			} else if reason := blockReason(loc, pub, sysComp != 0); reason != "" {
				in.Movable, in.Reason = false, reason
			}
			found = append(found, in)
		}
	}

	return found
}

// Scan lists installed programs with a known install folder, largest first.
func Scan() ([]Install, error) {
	found := enumerate()

	// Measure folder sizes concurrently; this is the slow part.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range found {
		wg.Add(1)
		sem <- struct{}{}
		go func(in *Install) {
			defer wg.Done()
			defer func() { <-sem }()
			if !in.Movable && in.MovedTo == "" {
				return // not offered, so not worth walking (a drive-root location would walk the whole disk)
			}
			target := in.Path
			if in.MovedTo != "" {
				target = in.MovedTo
			}
			if in.Broken {
				return
			}
			in.Size, _, _ = dirStats(target)
		}(&found[i])
	}
	wg.Wait()

	out := found[:0]
	for _, in := range found {
		if in.Size >= minListSize || in.MovedTo != "" {
			out = append(out, in)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out, nil
}

func blockReason(path, publisher string, systemComponent bool) string {
	lp := strings.ToLower(path)
	under := func(root string) bool {
		root = strings.ToLower(filepath.Clean(root))
		return root != "." && (lp == root || strings.HasPrefix(lp, root+`\`))
	}
	switch {
	case systemComponent:
		return "System component"
	case filepath.Dir(path) == path || filepath.Dir(path) == filepath.VolumeName(path)+`\`:
		return "Drive root. Moving it would move everything on the drive"
	case under(os.Getenv("SystemRoot")):
		return "Part of Windows"
	case strings.Contains(lp, `\windowsapps`):
		return "Store app, managed by Windows"
	case strings.Contains(lp, `\common files`), strings.Contains(lp, `\package cache`):
		return "Shared component used by other programs"
	case strings.Contains(strings.ToLower(publisher), "microsoft"):
		return "Managed by Microsoft; moving can break updates"
	case lp == strings.ToLower(os.Getenv("ProgramFiles")), lp == strings.ToLower(os.Getenv("ProgramFiles(x86)")),
		lp == strings.ToLower(os.Getenv("USERPROFILE")):
		return "Folder shared by many programs"
	}
	if exe, err := os.Executable(); err == nil && under(path) && strings.HasPrefix(strings.ToLower(exe), lp+`\`) {
		return "This is the running PC Optimizer"
	}
	return ""
}

// ---- Moving ----

// verifyMovable checks the folder against the installed-program list, with the
// same rules the scan used (publisher, system component), so a direct call
// can't move an arbitrary folder. Tests replace it.
var verifyMovable = func(src string) error {
	for _, in := range enumerate() {
		if strings.EqualFold(in.Path, src) {
			if !in.Movable {
				return errors.New("this install can't be moved: " + in.Reason)
			}
			return nil
		}
	}
	return errors.New("that folder isn't a program Windows lists as installed")
}

func Move(src, destRoot string, emit func(Progress)) (Result, error) {
	if !busy.TryLock() {
		return Result{}, errBusy
	}
	defer busy.Unlock()

	src = filepath.Clean(src)
	name := filepath.Base(src)
	step := func(stage string, pct float64) { emit(Progress{Name: name, Stage: stage, Percent: pct}) }
	step("checking", 0)

	fi, err := os.Lstat(src)
	if err != nil || !fi.IsDir() || isLink(fi) {
		return Result{}, errors.New("that folder isn't a regular install folder (already moved, or missing)")
	}
	if err := verifyMovable(src); err != nil {
		return Result{}, err
	}
	vol := filepath.VolumeName(destRoot)
	if vol == "" || strings.EqualFold(vol, filepath.VolumeName(src)) {
		return Result{}, errors.New("choose a different drive")
	}
	destRoot = vol + `\`
	if _, err := os.Stat(destRoot); err != nil {
		return Result{}, fmt.Errorf("drive %s isn't available", vol)
	}
	if err := checkDestVolume(destRoot); err != nil {
		return Result{}, err
	}
	dest, err := chooseDest(filepath.Join(destRoot, holdFolder, name), src)
	if err != nil {
		return Result{}, err
	}
	if procs := runningFrom(src); len(procs) > 0 {
		return Result{}, fmt.Errorf("close these first: %s", strings.Join(procs, ", "))
	}

	size, files, links := dirStatsLinks(src)
	if links {
		return Result{}, errors.New("this install contains links to other folders, which can't be moved safely")
	}
	if u, err := disk.Usage(destRoot); err != nil || u.Free < size+size/50+(100<<20) {
		return Result{}, fmt.Errorf("not enough free space on %s (needs %s)", vol, human(size))
	}

	// Copy.
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return Result{}, err
	}
	if err := robocopy(src, dest, size, func(p float64) { step("copying", p) }); err != nil {
		os.RemoveAll(dest)
		return Result{}, err
	}

	// Verify before touching the original.
	step("verifying", 100)
	if dsize, dfiles, _ := dirStats(dest); dsize != size || dfiles != files {
		os.RemoveAll(dest)
		return Result{}, fmt.Errorf("copy didn't match the original (%d of %d files); nothing was changed", dfiles, files)
	}

	// Switch the original over to a junction.
	step("switching", 100)
	old := src + oldSuffix
	for i := 2; exists(old); i++ { // a leftover from an earlier move; don't clobber it
		old = fmt.Sprintf("%s%s%d", src, oldSuffix, i)
	}
	if err := os.Rename(src, old); err != nil {
		os.RemoveAll(dest)
		return Result{}, fmt.Errorf("couldn't take over the original folder (is something using it, or do you need admin rights?): %w", err)
	}
	if err := makeJunction(src, dest); err != nil {
		os.Rename(old, src)
		os.RemoveAll(dest)
		return Result{}, fmt.Errorf("couldn't create the link: %w", err)
	}
	if _, err := os.ReadDir(src); err != nil {
		os.Remove(src)
		os.Rename(old, src)
		os.RemoveAll(dest)
		return Result{}, fmt.Errorf("link check failed, restored the original: %w", err)
	}

	// Done and verified; reclaim the space.
	step("cleaning", 100)
	res := Result{Dest: dest, Bytes: size}
	if err := os.RemoveAll(old); err != nil {
		res.Warning = fmt.Sprintf("Moved, but part of the old copy couldn't be deleted. Remove %s yourself to free the space.", old)
	}
	step("done", 100)
	return res, nil
}

// MoveBack undoes a move: replaces the junction with the real folder again.
func MoveBack(src string, emit func(Progress)) (Result, error) {
	if !busy.TryLock() {
		return Result{}, errBusy
	}
	defer busy.Unlock()

	src = filepath.Clean(src)
	name := filepath.Base(src)
	step := func(stage string, pct float64) { emit(Progress{Name: name, Stage: stage, Percent: pct}) }
	step("checking", 0)

	fi, err := os.Lstat(src)
	if err != nil || !isLink(fi) {
		return Result{}, errors.New("that install isn't currently moved")
	}
	target := linkTarget(src)
	if target == "" {
		return Result{}, errors.New("couldn't read where the install was moved to")
	}
	if _, err := os.Stat(target); err != nil {
		return Result{}, fmt.Errorf("the drive holding this program (%s) isn't available. Reconnect it, then try again", filepath.VolumeName(target))
	}
	if procs := runningFrom(target); len(procs) > 0 {
		return Result{}, fmt.Errorf("close these first: %s", strings.Join(procs, ", "))
	}
	size, files, _ := dirStats(target)
	if u, err := disk.Usage(filepath.VolumeName(src) + `\`); err != nil || u.Free < size+size/50+(100<<20) {
		return Result{}, fmt.Errorf("not enough free space on %s (needs %s)", filepath.VolumeName(src), human(size))
	}

	if err := os.Remove(src); err != nil { // removes the link only, not the target
		return Result{}, fmt.Errorf("couldn't remove the link: %w", err)
	}
	restore := func() { os.RemoveAll(src); makeJunction(src, target) }

	if err := robocopy(target, src, size, func(p float64) { step("copying", p) }); err != nil {
		restore()
		return Result{}, err
	}
	step("verifying", 100)
	if dsize, dfiles, _ := dirStats(src); dsize != size || dfiles != files {
		restore()
		return Result{}, fmt.Errorf("copy didn't match (%d of %d files); the install is still on the other drive", dfiles, files)
	}

	step("cleaning", 100)
	res := Result{Dest: src, Bytes: size}
	if err := os.RemoveAll(target); err != nil {
		res.Warning = fmt.Sprintf("Moved back, but couldn't delete %s. Remove it yourself to free the space.", target)
	}
	os.Remove(filepath.Dir(target)) // the "Moved Programs" folder, only if now empty
	step("done", 100)
	return res, nil
}

// ---- helpers ----

// checkDestVolume refuses drives a moved program can't safely live on: ones
// that can disappear (removable, network) and ones that can't hold the
// original permissions (anything but NTFS or ReFS).
func checkDestVolume(root string) error {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	switch windows.GetDriveType(p) {
	case windows.DRIVE_REMOVABLE, windows.DRIVE_CDROM:
		return fmt.Errorf("%s is a removable drive. The program would stop working whenever it is unplugged", root)
	case windows.DRIVE_REMOTE:
		return fmt.Errorf("%s is a network drive. The program would stop working whenever the network is unavailable", root)
	}
	var fsName [windows.MAX_PATH + 1]uint16
	if err := windows.GetVolumeInformation(p, nil, 0, nil, nil, nil, &fsName[0], uint32(len(fsName))); err != nil {
		return fmt.Errorf("couldn't read drive %s: %w", root, err)
	}
	if fs := windows.UTF16ToString(fsName[:]); !strings.EqualFold(fs, "NTFS") && !strings.EqualFold(fs, "ReFS") {
		return fmt.Errorf("%s is formatted as %s, which can't keep the program's file permissions. Use an NTFS drive", root, fs)
	}
	return nil
}

func robocopy(src, dst string, total uint64, progress func(float64)) error {
	cmd := exec.Command("robocopy", src, dst, "/E", "/COPY:DATSO", "/DCOPY:DAT", "/XJ",
		"/R:1", "/W:1", "/MT:8", "/NFL", "/NDL", "/NJH", "/NJS", "/NP")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		t := time.NewTicker(700 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if total > 0 {
					done, _, _ := dirStats(dst)
					progress(min(99, float64(done)/float64(total)*100))
				}
			}
		}
	}()

	out, err := cmd.CombinedOutput()
	close(stop)
	<-finished

	// Robocopy exit codes: 0-7 are success variants, 8+ means failures.
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ee.ExitCode() >= 8 {
			return fmt.Errorf("copy failed: %s", lastLine(string(out)))
		}
		return nil
	}
	return err
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return "unknown error"
}

func makeJunction(link, target string) error {
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", lastLine(string(out)))
	}
	return nil
}

func isLink(fi fs.FileInfo) bool {
	return fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

func linkTarget(p string) string {
	t, err := os.Readlink(p)
	if err != nil {
		return ""
	}
	return filepath.Clean(strings.TrimPrefix(t, `\??\`))
}

func dirStats(root string) (size uint64, files int, err error) {
	size, files, _ = dirStatsLinks(root)
	return
}

// dirStatsLinks totals regular files and reports whether any link was found.
func dirStatsLinks(root string) (size uint64, files int, links bool) {
	filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			links = true
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			size += uint64(info.Size())
			files++
		}
		return nil
	})
	return
}

// runningFrom lists processes whose executable lives under dir.
func runningFrom(dir string) []string {
	prefix := strings.ToLower(filepath.Clean(dir)) + `\`
	procs, err := process.Processes()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, p := range procs {
		exe, err := p.Exe()
		if err != nil || !strings.HasPrefix(strings.ToLower(exe), prefix) {
			continue
		}
		n := filepath.Base(exe)
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

func human(n uint64) string {
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

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// chooseDest picks the folder to copy into. If it already exists and holds
// only files that also exist in src, it is the remains of
// an interrupted move of this same install, so it is reused: robocopy skips
// what is already there and finishes the job, leaving a single copy. Anything
// else is some other program's folder, so a free name is used instead.
func chooseDest(dest, src string) (string, error) {
	for i := 1; i <= 20; i++ {
		cand := dest
		if i > 1 {
			cand = fmt.Sprintf("%s (%d)", dest, i)
		}
		fi, err := os.Lstat(cand)
		if err != nil {
			return cand, nil // free
		}
		if fi.IsDir() && !isLink(fi) && isPartialCopyOf(cand, src) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("%s and similar folders already exist; remove them and try again", dest)
}

func isPartialCopyOf(dest, src string) bool {
	ok := true
	filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !ok {
			return nil
		}
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			ok = false
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dest, p)
		// Sizes may differ (a file cut off mid-copy); robocopy recopies those.
		if _, err := os.Stat(filepath.Join(src, rel)); err != nil {
			ok = false
		}
		return nil
	})
	return ok
}
