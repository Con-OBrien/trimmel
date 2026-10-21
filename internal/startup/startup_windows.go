//go:build windows

package startup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const approvedBase = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\`

// source is one place Windows reads startup entries from, paired with the
// StartupApproved key that holds its enabled/disabled flags.
type source struct {
	id       string
	label    string
	root     registry.Key
	runKey   string                 // registry Run key, or "" for a folder
	folder   func() (string, error) // startup folder, or nil for a Run key
	approved string                 // subkey under approvedBase
	scope    string
}

var sources = []source{
	{id: "hkcu-run", label: `Registry: HKCU\...\Run`, root: registry.CURRENT_USER,
		runKey: `Software\Microsoft\Windows\CurrentVersion\Run`, approved: "Run", scope: "user"},
	{id: "hklm-run", label: `Registry: HKLM\...\Run`, root: registry.LOCAL_MACHINE,
		runKey: `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, approved: "Run", scope: "machine"},
	{id: "hklm-run32", label: `Registry: HKLM\...\Run (32-bit)`, root: registry.LOCAL_MACHINE,
		runKey: `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, approved: "Run32", scope: "machine"},
	{id: "user-folder", label: "Startup folder (you)", root: registry.CURRENT_USER,
		folder:   func() (string, error) { return windows.KnownFolderPath(windows.FOLDERID_Startup, 0) },
		approved: "StartupFolder", scope: "user"},
	{id: "common-folder", label: "Startup folder (all users)", root: registry.LOCAL_MACHINE,
		folder:   func() (string, error) { return windows.KnownFolderPath(windows.FOLDERID_CommonStartup, 0) },
		approved: "StartupFolder", scope: "machine"},
}

func List() ([]Item, error) {
	var items []Item
	for _, s := range sources {
		entries, err := s.entries()
		if err != nil {
			continue // key or folder absent on this machine
		}
		flags := s.approvedFlags()
		for name, cmd := range entries {
			items = append(items, Item{
				ID:            s.id + "|" + name,
				Name:          displayName(name),
				Command:       cmd,
				Source:        s.label,
				Scope:         s.scope,
				Enabled:       isEnabled(flags[name]),
				RequiresAdmin: s.scope == "machine",
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items, nil
}

func SetEnabled(id string, enabled bool) error {
	srcID, name, ok := strings.Cut(id, "|")
	if !ok {
		return fmt.Errorf("bad startup item id %q", id)
	}
	var s *source
	for i := range sources {
		if sources[i].id == srcID {
			s = &sources[i]
		}
	}
	if s == nil {
		return fmt.Errorf("unknown startup source %q", srcID)
	}

	k, _, err := registry.CreateKey(s.root, approvedBase+s.approved, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return errors.New("this item applies to all users; restart as admin to change it")
		}
		return err
	}
	defer k.Close()
	return k.SetBinaryValue(name, approvedValue(enabled))
}

func (s source) entries() (map[string]string, error) {
	out := map[string]string{}
	if s.folder != nil {
		dir, err := s.folder()
		if err != nil {
			return nil, err
		}
		des, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, de := range des {
			if de.IsDir() || strings.EqualFold(de.Name(), "desktop.ini") {
				continue
			}
			out[de.Name()] = dir + `\` + de.Name()
		}
		return out, nil
	}

	k, err := registry.OpenKey(s.root, s.runKey, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()
	names, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		if v, _, err := k.GetStringValue(n); err == nil {
			out[n] = v
		}
	}
	return out, nil
}

func (s source) approvedFlags() map[string][]byte {
	out := map[string][]byte{}
	k, err := registry.OpenKey(s.root, approvedBase+s.approved, registry.QUERY_VALUE)
	if err != nil {
		return out
	}
	defer k.Close()
	names, _ := k.ReadValueNames(-1)
	for _, n := range names {
		if v, _, err := k.GetBinaryValue(n); err == nil {
			out[n] = v
		}
	}
	return out
}

// StartupApproved values are 12 bytes: a 4-byte state (0x02 or 0x06 enabled,
// 0x03 or 0x07 disabled, so the low bit means disabled) followed by the
// FILETIME of when it was disabled. A missing value means enabled.
func isEnabled(v []byte) bool {
	return len(v) == 0 || v[0]&1 == 0
}

func approvedValue(enabled bool) []byte {
	b := make([]byte, 12)
	if enabled {
		b[0] = 0x02
		return b
	}
	b[0] = 0x03
	ft := windows.NsecToFiletime(time.Now().UnixNano())
	binary.LittleEndian.PutUint32(b[4:], ft.LowDateTime)
	binary.LittleEndian.PutUint32(b[8:], ft.HighDateTime)
	return b
}

func displayName(name string) string {
	if i := strings.LastIndex(name, "."); i > 0 && strings.EqualFold(name[i:], ".lnk") {
		return name[:i]
	}
	return name
}
