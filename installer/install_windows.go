//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	appExe       = "trimmel.exe"
	uninstallExe = "uninstall.exe"
	appName      = "Trimmel"
	publisher    = "Trimmel"
	uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\Trimmel`

	// PC Optimizer 0.1.x, the app's name before Trimmel. Setup finds and
	// replaces it: same app, new name.
	legacyExe          = "pcoptimizer.exe"
	legacyAppName      = "PC Optimizer"
	legacyUninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\PCOptimizer`
)

//go:embed payload/trimmel.exe
var payload []byte

type emitFn func(stage string, percent float64)

func payloadSizeMB() int { return (len(payload) + 1<<20 - 1) >> 20 }

func defaultInstallDir() string {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, appName)
}

// registeredDir reads the install folder from an uninstall key, but only if
// the named exe is really there.
func registeredDir(key, exe string) string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(dir, exe)); err != nil {
		return ""
	}
	return dir
}

// existingInstallDir returns the folder of a current install, or "". A
// PC Optimizer 0.1.x install counts: setup upgrades it in place of a fresh install.
func existingInstallDir() string {
	if d := registeredDir(uninstallKey, appExe); d != "" {
		return d
	}
	return registeredDir(legacyUninstallKey, legacyExe)
}

// legacyInstallDir returns the folder of an old PC Optimizer install, or "".
func legacyInstallDir() string { return registeredDir(legacyUninstallKey, legacyExe) }

func startMenuLinkFor(name string) string {
	return filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`, name+".lnk")
}

func desktopLinkFor(name string) string {
	pub := os.Getenv("PUBLIC")
	if pub == "" {
		pub = `C:\Users\Public`
	}
	return filepath.Join(pub, "Desktop", name+".lnk")
}

func startMenuLink() string { return startMenuLinkFor(appName) }
func desktopLink() string   { return desktopLinkFor(appName) }

// killApp ends any running copy of the app that lives in one of dirs, so its
// exe can be replaced or removed. Other programs with the same name are left alone.
func killApp(dirs ...string) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snap)

	want := map[string]bool{}
	for _, d := range dirs {
		if d != "" {
			want[strings.ToLower(filepath.Join(d, appExe))] = true
			want[strings.ToLower(filepath.Join(d, legacyExe))] = true
		}
	}

	killed := false
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if name := windows.UTF16ToString(e.ExeFile[:]); !strings.EqualFold(name, appExe) && !strings.EqualFold(name, legacyExe) {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, e.ProcessID)
		if err != nil {
			continue
		}
		buf := make([]uint16, windows.MAX_PATH)
		n := uint32(len(buf))
		if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil && want[strings.ToLower(windows.UTF16ToString(buf[:n]))] {
			if windows.TerminateProcess(h, 1) == nil {
				_, _ = windows.WaitForSingleObject(h, 3000)
				killed = true
			}
		}
		windows.CloseHandle(h)
	}
	if killed {
		time.Sleep(300 * time.Millisecond)
	}
}

// removeAtReboot schedules a path for deletion at the next restart. Used for the
// running uninstaller, which cannot delete itself.
func removeAtReboot(path string) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_ = windows.MoveFileEx(p, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
}

// removeSavedSignIn deletes the sign-in token the app keeps in Windows
// Credential Manager (generic credentials named "<service>:<user>", the same
// target go-keyring uses). It only reaches the account running the uninstaller.
func removeSavedSignIn() {
	del := windows.NewLazySystemDLL("advapi32.dll").NewProc("CredDeleteW")
	for _, service := range []string{"trimmel", "pcoptimizer"} {
		target, err := windows.UTF16PtrFromString(service + ":oidc-refresh-token")
		if err != nil {
			continue
		}
		const credTypeGeneric = 1
		_, _, _ = del.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	}
}

func writeFile(path string, data []byte) error {
	var err error
	for i := 0; i < 5; i++ {
		if err = os.WriteFile(path, data, 0o755); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	var out *os.File
	for i := 0; i < 5; i++ {
		if out, err = os.Create(dst); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func makeShortcut(link, target, dir string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err == nil {
		defer ole.CoUninitialize()
	}
	unk, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return err
	}
	defer unk.Release()
	sh, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer sh.Release()
	v, err := oleutil.CallMethod(sh, "CreateShortcut", link)
	if err != nil {
		return err
	}
	sc := v.ToIDispatch()
	defer sc.Release()
	for k, val := range map[string]string{"TargetPath": target, "WorkingDirectory": dir, "IconLocation": target + ",0", "Description": appName} {
		if _, err := oleutil.PutProperty(sc, k, val); err != nil {
			return err
		}
	}
	_, err = oleutil.CallMethod(sc, "Save")
	return err
}

func install(o Options, emit emitFn) (err error) {
	dir := filepath.Clean(strings.TrimSpace(o.Dir))
	if dir == "" || dir == "." || !filepath.IsAbs(dir) {
		return fmt.Errorf("choose a full folder path, such as %s", defaultInstallDir())
	}
	// Never install straight into a drive root or a system folder.
	if vol := filepath.VolumeName(dir); dir == vol+`\` || strings.EqualFold(dir, os.Getenv("SystemRoot")) {
		return fmt.Errorf("pick a dedicated folder, not %s", dir)
	}

	old := existingInstallDir()
	emit("Closing Trimmel if it is running…", 4)
	killApp(dir, old)

	// If setup fails part-way, undo what it added so no half install is left.
	// An install that was already in place keeps working: its files are only
	// removed when they live in a different folder from the new one.
	sameDir := old != "" && strings.EqualFold(filepath.Clean(old), dir)
	defer func() {
		if err == nil || sameDir {
			return
		}
		removeInstallFiles(dir)
		if old == "" {
			_ = os.Remove(startMenuLink())
			_ = os.Remove(desktopLink())
			_ = registry.DeleteKey(registry.LOCAL_MACHINE, uninstallKey)
		}
	}()

	emit("Creating the install folder…", 10)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("couldn't create %s: %w", dir, err)
	}

	emit("Copying Trimmel…", 25)
	if err := writeFile(filepath.Join(dir, appExe), payload); err != nil {
		return fmt.Errorf("couldn't write the app (is it still open?): %w", err)
	}

	emit("Adding the uninstaller…", 60)
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Dir(self), dir) {
		if err := copyFile(self, filepath.Join(dir, uninstallExe)); err != nil {
			return fmt.Errorf("couldn't add the uninstaller: %w", err)
		}
	}

	emit("Creating shortcuts…", 75)
	exe := filepath.Join(dir, appExe)
	if err := makeShortcut(startMenuLink(), exe, dir); err != nil {
		return fmt.Errorf("couldn't create the Start Menu shortcut: %w", err)
	}
	if o.DesktopLink {
		if err := makeShortcut(desktopLink(), exe, dir); err != nil {
			return fmt.Errorf("couldn't create the desktop shortcut: %w", err)
		}
	} else {
		_ = os.Remove(desktopLink())
	}

	emit("Registering with Windows…", 90)
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, uninstallKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("couldn't register the app: %w", err)
	}
	defer k.Close()
	for name, val := range map[string]string{
		"DisplayName":     appName,
		"DisplayVersion":  version,
		"Publisher":       publisher,
		"InstallLocation": dir,
		"DisplayIcon":     exe,
		"UninstallString": `"` + filepath.Join(dir, uninstallExe) + `" --uninstall`,
	} {
		if err := k.SetStringValue(name, val); err != nil {
			return err
		}
	}
	_ = k.SetDWordValue("NoModify", 1)
	_ = k.SetDWordValue("NoRepair", 1)
	_ = k.SetDWordValue("EstimatedSize", uint32(len(payload)/1024*2))

	// Moving to a new folder: clear the old copy now that the new one works.
	if old != "" && !strings.EqualFold(filepath.Clean(old), dir) {
		emit("Removing the old copy…", 96)
		removeInstallFiles(old)
	}
	// Upgrading from PC Optimizer 0.1.x: remove its entry, shortcuts and exe.
	removeLegacy(dir)

	emit("All done", 100)
	return nil
}

func uninstall(o Options, emit emitFn) error {
	dir := existingInstallDir()
	if dir == "" {
		self, _ := os.Executable()
		dir = filepath.Dir(self)
	}

	emit("Closing Trimmel…", 10)
	killApp(dir)

	emit("Removing shortcuts…", 35)
	_ = os.Remove(startMenuLink())
	_ = os.Remove(desktopLink())

	emit("Removing Trimmel from Windows…", 55)
	_ = registry.DeleteKey(registry.LOCAL_MACHINE, uninstallKey)
	removeLegacy("")

	if o.RemoveConfig {
		emit("Deleting your settings…", 70)
		_ = os.RemoveAll(filepath.Join(os.Getenv("APPDATA"), "trimmel"))
		_ = os.RemoveAll(filepath.Join(os.Getenv("APPDATA"), "pcoptimizer"))
		removeSavedSignIn()
	}

	emit("Deleting program files…", 85)
	removeInstallFiles(dir)
	emit("All done", 100)
	return nil
}

// removeInstallFiles deletes what setup put in dir. The running uninstaller and
// the folder go at the next restart if they are still in use. Files the user
// added to the folder are kept.
func removeInstallFiles(dir string) {
	_ = os.Remove(filepath.Join(dir, appExe))
	_ = os.Remove(filepath.Join(dir, legacyExe))
	un := filepath.Join(dir, uninstallExe)
	if err := os.Remove(un); err != nil && !os.IsNotExist(err) {
		removeAtReboot(un)
	}
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		removeAtReboot(dir)
	}
}

// removeLegacy takes PC Optimizer 0.1.x off the machine: its Add/Remove entry,
// shortcuts, exe and folder. keep is the folder of the new install; if the old
// one is the same folder, only the old exe is removed. Settings are not
// touched here: the app copies them to the new folder on first start.
func removeLegacy(keep string) {
	old := legacyInstallDir()
	_ = os.Remove(startMenuLinkFor(legacyAppName))
	_ = os.Remove(desktopLinkFor(legacyAppName))
	_ = registry.DeleteKey(registry.LOCAL_MACHINE, legacyUninstallKey)
	if old == "" {
		return
	}
	if keep != "" && strings.EqualFold(filepath.Clean(old), filepath.Clean(keep)) {
		_ = os.Remove(filepath.Join(old, legacyExe))
		return
	}
	killApp(old)
	removeInstallFiles(old)
}
