//go:build windows

package oldver

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

	"golang.org/x/sys/windows/registry"
)

var roots = []struct {
	tag  string
	root registry.Key
	path string
}{
	{"lm", registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{"lm32", registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{"cu", registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

func read() []entry {
	var out []entry
	for _, r := range roots {
		key, err := registry.OpenKey(r.root, r.path, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, _ := key.ReadSubKeyNames(-1)
		key.Close()
		for _, n := range names {
			sub, err := registry.OpenKey(r.root, r.path+`\`+n, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			e := entry{id: r.tag + "|" + n}
			e.name, _, _ = sub.GetStringValue("DisplayName")
			e.version, _, _ = sub.GetStringValue("DisplayVersion")
			e.publisher, _, _ = sub.GetStringValue("Publisher")
			e.uninstall, _, _ = sub.GetStringValue("UninstallString")
			loc, _, _ := sub.GetStringValue("InstallLocation")
			e.location = strings.Trim(strings.TrimSpace(loc), "\"")
			if kb, _, err := sub.GetIntegerValue("EstimatedSize"); err == nil {
				e.size = kb * 1024
			}
			sysComp, _, _ := sub.GetIntegerValue("SystemComponent")
			if p, _, _ := sub.GetStringValue("ParentKeyName"); p != "" {
				e.hasParent = true
			}
			sub.Close()
			if e.name != "" && sysComp == 0 {
				out = append(out, e)
			}
		}
	}
	return out
}

// Scan lists older duplicates. Many installers don't record a size, so those
// are measured from the install folder.
func Scan() ([]Old, error) {
	entries := read()
	loc := map[string]string{}
	for _, e := range entries {
		loc[e.id] = e.location
	}
	olds := find(entries)
	var wg sync.WaitGroup
	for i := range olds {
		if olds[i].Size != 0 || loc[olds[i].ID] == "" {
			continue
		}
		wg.Add(1)
		go func(o *Old, dir string) {
			defer wg.Done()
			o.Size = dirSize(dir)
		}(&olds[i], loc[olds[i].ID])
	}
	wg.Wait()
	sort.Slice(olds, func(i, j int) bool {
		if olds[i].Size != olds[j].Size {
			return olds[i].Size > olds[j].Size
		}
		return olds[i].Name < olds[j].Name
	})
	return olds, nil
}

func dirSize(root string) (n uint64) {
	filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return nil
		}
		if info, err := d.Info(); err == nil {
			n += uint64(info.Size())
		}
		return nil
	})
	return
}

// Uninstall runs the program's own uninstaller and waits for it to exit. The
// UI is the vendor's, so the user sees and confirms what is removed. The
// command is looked up from the registry by ID and must still be flagged as
// an older version, so callers can't make this run arbitrary commands.
func Uninstall(id string) error {
	entries := read()
	var target *entry
	for _, o := range find(entries) {
		if o.ID != id {
			continue
		}
		for i := range entries {
			if entries[i].id == id {
				target = &entries[i]
			}
		}
	}
	if target == nil {
		return errors.New("that program is no longer listed as an older version")
	}
	if exe := uninstallerExe(target.uninstall); exe != "" {
		if _, err := os.Stat(exe); err != nil {
			return fmt.Errorf("the uninstaller is missing (%s). Windows still lists %s, but it looks already removed. "+
				"This is a leftover entry; it can be cleared from Settings > Apps if it bothers you", exe, target.name)
		}
	}
	// "start" goes through ShellExecute, so uninstallers that need admin show
	// the normal UAC prompt.
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CmdLine:    `cmd.exe /S /C "start "" /wait ` + quoteCommand(target.uninstall) + `"`,
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("uninstaller failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// uninstallerExe extracts the program path from an UninstallString, or ""
// when it isn't a plain file path (msiexec and the like are always present).
func uninstallerExe(cmdline string) string {
	s := strings.TrimSpace(cmdline)
	var p string
	if strings.HasPrefix(s, `"`) {
		if end := strings.Index(s[1:], `"`); end >= 0 {
			p = s[1 : 1+end]
		}
	} else if i := strings.Index(strings.ToLower(s), ".exe"); i >= 0 {
		p = s[:i+4]
	}
	if p == "" || strings.EqualFold(filepath.Base(p), "msiexec.exe") || !filepath.IsAbs(p) {
		return ""
	}
	return p
}

// quoteCommand wraps the program path in quotes when the registry stored it
// bare. Many installers write unquoted paths containing spaces, which the
// shell would otherwise split at the first space.
func quoteCommand(cmdline string) string {
	s := strings.TrimSpace(cmdline)
	if strings.HasPrefix(s, `"`) {
		return s
	}
	if i := strings.Index(strings.ToLower(s), ".exe"); i >= 0 {
		return `"` + s[:i+4] + `"` + s[i+4:]
	}
	return s
}
