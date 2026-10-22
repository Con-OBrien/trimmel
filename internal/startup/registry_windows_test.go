//go:build windows

package startup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}

func findItem(t *testing.T, id string) (Item, bool) {
	t.Helper()
	items, err := List()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

func TestRunKeyEntryCanBeSwitchedOffAndOn(t *testing.T) {
	name := uniqueName("TrimmelTest")
	run, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		t.Skipf("cannot open the user Run key: %v", err)
	}
	defer run.Close()
	if err := run.SetStringValue(name, `C:\Windows\System32\notepad.exe`); err != nil {
		t.Skipf("cannot write the user Run key: %v", err)
	}
	t.Cleanup(func() {
		if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE); err == nil {
			_ = k.DeleteValue(name)
			k.Close()
		}
		if k, err := registry.OpenKey(registry.CURRENT_USER, approvedBase+"Run", registry.SET_VALUE); err == nil {
			_ = k.DeleteValue(name)
			k.Close()
		}
	})

	id := "hkcu-run|" + name
	it, ok := findItem(t, id)
	if !ok {
		t.Fatalf("entry %s not listed", id)
	}
	if !it.Enabled || it.Scope != "user" || it.RequiresAdmin || it.Command == "" {
		t.Errorf("new entry should be enabled, user scope, no admin needed: %+v", it)
	}

	if err := SetEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if it, _ := findItem(t, id); it.Enabled {
		t.Error("entry should be disabled")
	}
	if err := SetEnabled(id, true); err != nil {
		t.Fatal(err)
	}
	if it, _ := findItem(t, id); !it.Enabled {
		t.Error("entry should be enabled again")
	}
}

func TestFolderEntriesSkipDirectoriesAndDesktopIni(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"App.lnk", "desktop.ini", "tool.cmd"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := source{folder: func() (string, error) { return dir, nil }}
	got, err := s.entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["App.lnk"] == "" || got["tool.cmd"] == "" {
		t.Errorf("unexpected entries: %v", got)
	}
}

func TestFolderEntriesMissingFolderIsAnError(t *testing.T) {
	s := source{folder: func() (string, error) { return filepath.Join(t.TempDir(), "nope"), nil }}
	if _, err := s.entries(); err == nil {
		t.Error("a missing folder should be reported")
	}
}

func TestListIsSortedAndAdvised(t *testing.T) {
	items, err := List()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(items); i++ {
		if strings.ToLower(items[i-1].Name) > strings.ToLower(items[i].Name) {
			t.Fatalf("not sorted by name: %q before %q", items[i-1].Name, items[i].Name)
		}
	}
	for _, it := range items {
		if it.Impact == "" {
			t.Errorf("%s has no impact rating", it.ID)
		}
	}
}
