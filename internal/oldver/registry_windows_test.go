//go:build windows

package oldver

import (
	"os"
	"strings"
	"testing"
)

func TestReadFindsInstalledPrograms(t *testing.T) {
	entries := read()
	if len(entries) == 0 {
		t.Skip("no uninstall entries on this machine")
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.name == "" || seen[e.id] || !strings.Contains(e.id, "|") {
			t.Fatalf("bad or duplicate entry: %+v", e)
		}
		seen[e.id] = true
	}
}

func TestScanAndListAppsAreConsistent(t *testing.T) {
	olds, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(olds); i++ {
		if olds[i].Size > olds[i-1].Size {
			t.Error("older versions must be sorted largest first")
		}
	}
	apps := ListApps()
	for i, a := range apps {
		if a.Name == "" || a.ID == "" || strings.EqualFold(a.Name, "Trimmel") || isUpdateName(a.Name) {
			t.Fatalf("unexpected app: %+v", a)
		}
		if i > 0 && a.Size > apps[i-1].Size {
			t.Error("apps must be sorted largest first")
		}
	}
}

func TestUnknownIDsAreRefused(t *testing.T) {
	if err := Uninstall("lm|{no-such-product}"); err == nil || !strings.Contains(err.Error(), "no longer listed") {
		t.Errorf("Uninstall: got %v", err)
	}
	if err := UninstallApp("lm|{no-such-product}"); err == nil || !strings.Contains(err.Error(), "no longer listed") {
		t.Errorf("UninstallApp: got %v", err)
	}
}

func TestLaunchReportsMissingUninstaller(t *testing.T) {
	err := launch(&entry{name: "Ghost", uninstall: `"C:\no\such\folder\unins000.exe" /S`})
	if err == nil || !strings.Contains(err.Error(), "uninstaller is missing") {
		t.Errorf("got %v", err)
	}
}

func TestLaunchRunsUninstallerAndReportsExit(t *testing.T) {
	cmd := os.Getenv("ComSpec")
	if cmd == "" {
		t.Skip("no ComSpec")
	}
	if err := launch(&entry{name: "Fine", uninstall: `"` + cmd + `" /C exit 0`}); err != nil {
		t.Errorf("a clean exit is success, got %v", err)
	}
}

func TestUninstallerExe(t *testing.T) {
	for in, want := range map[string]string{
		`"C:\Program Files\App\unins000.exe" /SILENT`: `C:\Program Files\App\unins000.exe`,
		`C:\Apps\x\uninstall.exe /S`:                  `C:\Apps\x\uninstall.exe`,
		`MsiExec.exe /X{ABC}`:                         "",
		`"C:\Windows\System32\msiexec.exe" /X{ABC}`:   "",
		`relative.exe`:                                "",
		``:                                            "",
		`"unterminated`:                               "",
	} {
		if got := uninstallerExe(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestDirSizeIgnoresLinksAndMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+`\a.bin`, make([]byte, 300), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := dirSize(dir); got != 300 {
		t.Errorf("got %d, want 300", got)
	}
	if got := dirSize(dir + `\missing`); got != 0 {
		t.Errorf("a missing folder is empty, got %d", got)
	}
}

func TestIsSharedAndExcluded(t *testing.T) {
	if !isShared(entry{name: "Whatever", publisher: "Microsoft Corporation"}) {
		t.Error("Microsoft programs are shared")
	}
	if isShared(entry{name: "Notepad Plus", publisher: "Don Ho"}) {
		t.Error("an ordinary program is not shared")
	}
}
