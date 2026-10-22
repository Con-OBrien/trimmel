// Package cleanup finds and removes junk files: temp folders, browser caches,
// update caches, crash dumps and the Recycle Bin.
package cleanup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// target describes one category of junk. Roots returns the directories (or
// individual files) to clear; the roots themselves are never deleted.
type target struct {
	ID            string
	Name          string
	Description   string
	RequiresAdmin bool
	MinAge        time.Duration // skip files modified more recently than this
	Roots         func() []string
	// Guard, if set, is checked before scanning or cleaning. A non-nil error
	// means the target is unsafe right now and explains why.
	Guard func() error
	// Special targets (Recycle Bin) supply their own scan and clean.
	ScanFn  func() (bytes uint64, files int64, err error)
	CleanFn func() (bytes uint64, files int64, err error)
}

type TargetReport struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	RequiresAdmin bool   `json:"requiresAdmin"`
	Available     bool   `json:"available"` // false if admin needed and missing
	Bytes         uint64 `json:"bytes"`
	Files         int64  `json:"files"`
	Error         string `json:"error,omitempty"`
}

type CleanResult struct {
	ID         string `json:"id"`
	FreedBytes uint64 `json:"freedBytes"`
	Deleted    int64  `json:"deleted"`
	Skipped    int64  `json:"skipped"` // locked or in-use files
	Error      string `json:"error,omitempty"`
}

var errNeedsAdmin = errors.New("needs admin rights")

// Scan measures every target without deleting anything.
func Scan(isAdmin bool) []TargetReport {
	ts := targets()
	out := make([]TargetReport, 0, len(ts))
	for _, t := range ts {
		r := TargetReport{
			ID: t.ID, Name: t.Name, Description: t.Description,
			RequiresAdmin: t.RequiresAdmin, Available: isAdmin || !t.RequiresAdmin,
		}
		if !r.Available {
			r.Error = errNeedsAdmin.Error()
			out = append(out, r)
			continue
		}
		if t.Guard != nil {
			if gerr := t.Guard(); gerr != nil {
				r.Available, r.Error = false, gerr.Error()
				out = append(out, r)
				continue
			}
		}
		var err error
		if t.ScanFn != nil {
			r.Bytes, r.Files, err = t.ScanFn()
		} else {
			r.Bytes, r.Files = walk(t, false, nil)
		}
		if err != nil {
			r.Error = err.Error()
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out
}

// Clean deletes eligible files in the selected targets.
func Clean(ids []string, isAdmin bool) []CleanResult {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []CleanResult
	for _, t := range targets() {
		if !want[t.ID] {
			continue
		}
		r := CleanResult{ID: t.ID}
		switch {
		case t.RequiresAdmin && !isAdmin:
			r.Error = errNeedsAdmin.Error()
		case t.Guard != nil && t.Guard() != nil:
			r.Error = t.Guard().Error()
		case t.CleanFn != nil:
			var err error
			r.FreedBytes, r.Deleted, err = t.CleanFn()
			if err != nil {
				r.Error = err.Error()
			}
		default:
			r.FreedBytes, r.Deleted = walk(t, true, &r.Skipped)
		}
		out = append(out, r)
	}
	return out
}

// walk sums (and optionally deletes) eligible files under the target roots.
// Symlinks and junctions are never followed, so a link inside a temp folder
// cannot lead the cleaner into real user data.
func walk(t target, del bool, skipped *int64) (bytes uint64, files int64) {
	cutoff := time.Now().Add(-t.MinAge)

	for _, root := range t.Roots() {
		if unsafeRoot(root) {
			continue
		}
		info, err := os.Lstat(root)
		if err != nil || info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			continue
		}
		if !info.IsDir() {
			handleFile(root, info, cutoff, del, &bytes, &files, skipped)
			continue
		}

		var dirs []string
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable entry: skip it, keep walking
			}
			if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if path != root {
					if di, err := d.Info(); err == nil && di.ModTime().Before(cutoff) {
						dirs = append(dirs, path)
					}
				}
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return nil
			}
			handleFile(path, fi, cutoff, del, &bytes, &files, skipped)
			return nil
		})

		if del {
			// Deepest first, so parents empty out before we try them.
			// os.Remove only removes empty directories.
			for i := len(dirs) - 1; i >= 0; i-- {
				_ = os.Remove(dirs[i])
			}
		}
	}
	return bytes, files
}

func handleFile(path string, fi fs.FileInfo, cutoff time.Time, del bool, bytes *uint64, files *int64, skipped *int64) {
	if fi.ModTime().After(cutoff) || keepFile(path) {
		return
	}
	if del {
		if err := os.Remove(path); err != nil {
			if skipped != nil {
				*skipped++
			}
			return
		}
	}
	*bytes += uint64(fi.Size())
	*files++
}

// globAll expands several glob patterns, ignoring malformed ones.
func globAll(patterns ...string) []string {
	var out []string
	for _, p := range patterns {
		m, _ := filepath.Glob(p)
		out = append(out, m...)
	}
	return out
}

// unsafeRoot reports whether a cleanup root is somewhere that must never be
// swept: a drive root, the Windows folder or the profile root. This guards
// against a TEMP variable pointing somewhere unexpected (or being unset, which
// makes os.TempDir fall back to the Windows folder).
func unsafeRoot(root string) bool {
	root = filepath.Clean(root)
	if filepath.Dir(root) == root {
		return true
	}
	for _, env := range []string{"SystemRoot", "SystemDrive", "USERPROFILE", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"} {
		if v := os.Getenv(env); v != "" && strings.EqualFold(filepath.Clean(v), root) {
			return true
		}
	}
	return false
}

// keepFile reports whether a file looks like live document state that an
// open application may still depend on, however old it is: Office lock files
// (~$name) and Word AutoRecover/backup files.
func keepFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(name, "~$") {
		return true
	}
	switch filepath.Ext(name) {
	case ".asd", ".wbk":
		return true
	}
	return false
}
