//go:build windows

package mover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanAndLeftoversAreReadOnlyAndSorted(t *testing.T) {
	installs, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(installs); i++ {
		if installs[i].Size > installs[i-1].Size {
			t.Error("installs must be sorted largest first")
		}
	}
	for _, in := range installs {
		if in.Path == "" || in.Name == "" {
			t.Errorf("bad install: %+v", in)
		}
		if !in.Movable && in.Reason == "" && in.MovedTo == "" {
			t.Errorf("%s is not offered but gives no reason", in.Name)
		}
	}
	left, err := Leftovers()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(left); i++ {
		if left[i].Size > left[i-1].Size {
			t.Error("leftovers must be sorted largest first")
		}
	}
}

func TestBlockReason(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	t.Setenv("ProgramFiles", `C:\Stuff\Programs`)
	t.Setenv("ProgramFiles(x86)", `C:\Program Files (x86)`)
	t.Setenv("USERPROFILE", `C:\Users\x`)
	cases := []struct {
		path, pub string
		sys       bool
		want      string
	}{
		{`C:\Apps\Thing`, "Acme", true, "System component"},
		{`C:\Thing`, "Acme", false, "Drive root"},
		{`C:\Windows\System32\x`, "Acme", false, "Part of Windows"},
		{`C:\Program Files\WindowsApps\Foo`, "Acme", false, "Store app"},
		{`C:\Program Files\Common Files\Foo`, "Acme", false, "Shared component"},
		{`C:\Package Cache\Foo`, "Acme", false, "Shared component"},
		{`C:\Apps\Office`, "Microsoft Corporation", false, "Microsoft"},
		{`C:\Stuff\Programs`, "Acme", false, "shared by many"},
		{`C:\Users\x`, "Acme", false, "shared by many"},
		{`D:\Games\Fun`, "Acme", false, ""},
	}
	for _, c := range cases {
		got := blockReason(c.path, c.pub, c.sys)
		if c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to contain %q", c.path, got, c.want)
		}
	}
	if exe, err := os.Executable(); err == nil {
		if got := blockReason(filepath.Dir(exe), "Acme", false); !strings.Contains(got, "running Trimmel") {
			t.Errorf("the folder of the running exe must be blocked, got %q", got)
		}
	}
}

func TestDeleteLeftoverRemovesOnlyHoldFolderChildren(t *testing.T) {
	root := t.TempDir()
	hold := filepath.Join(root, holdFolder)
	leftover := filepath.Join(hold, "Old App")
	if err := os.MkdirAll(leftover, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leftover, "a.dll"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	elsewhere := filepath.Join(root, "Documents")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DeleteLeftover(elsewhere); err == nil || !strings.Contains(err.Error(), "isn't a leftover") {
		t.Errorf("a folder outside Moved Programs must be refused, got %v", err)
	}
	if err := DeleteLeftover(filepath.Join(hold, "missing")); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("a missing folder must be reported, got %v", err)
	}
	if _, err := os.Stat(elsewhere); err != nil {
		t.Fatal("the refused folder must still exist")
	}

	if err := DeleteLeftover(leftover); err != nil {
		t.Fatalf("a real leftover should be removed: %v", err)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Error("the leftover should be gone")
	}
	if _, err := os.Stat(hold); !os.IsNotExist(err) {
		t.Error("the empty Moved Programs folder should be removed too")
	}
}

func TestDeleteLeftoverIsBusyDuringAMove(t *testing.T) {
	busy.Lock()
	defer busy.Unlock()
	if err := DeleteLeftover(`C:\x\` + holdFolder + `\y`); err != errBusy {
		t.Errorf("got %v, want errBusy", err)
	}
}

func TestHumanSizes(t *testing.T) {
	for in, want := range map[uint64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB"} {
		if got := human(in); got != want {
			t.Errorf("%d: got %q, want %q", in, got, want)
		}
	}
}

func TestLastLine(t *testing.T) {
	if got := lastLine("first\r\n  second  \r\n\r\n"); got != "second" {
		t.Errorf("got %q", got)
	}
	if got := lastLine("  \n "); got != "unknown error" {
		t.Errorf("got %q", got)
	}
}

func TestCheckDestWrapsVolumeCheck(t *testing.T) {
	if err := CheckDest(`C:\`); err != nil {
		t.Logf("system drive refused here: %v", err)
	}
	if err := CheckDest("bad\x00path"); err == nil {
		t.Error("an invalid path must be an error")
	}
	if err := CheckDest(`Q:\no-such-drive\`); err == nil {
		t.Log("an unmounted drive letter happened to exist")
	}
}
