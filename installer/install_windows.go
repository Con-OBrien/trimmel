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
	appExe       = "pcoptimizer.exe"
	uninstallExe = "uninstall.exe"
	appName      = "PC Optimizer"
	publisher    = "PC Optimizer"
	uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\PCOptimizer`
)

//go:embed payload/pcoptimizer.exe
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

// existingInstallDir returns the folder of a current install, or "".
func existingInstallDir() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(dir, appExe)); err != nil {
		return ""
	}
	return dir
}

func startMenuLink() string {
	return filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`, appName+".lnk")
}

func desktopLink() string {
	pub := os.Getenv("PUBLIC")
	if pub == "" {
		pub = `C:\Users\Public`
	}
	return filepath.Join(pub, "Desktop", appName+".lnk")
}

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
		}
	}

	killed := false
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if !strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), appExe) {
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

func install(o Options, emit emitFn) error {
	dir := filepath.Clean(strings.TrimSpace(o.Dir))
	if dir == "" || dir == "." || !filepath.IsAbs(dir) {
		return fmt.Errorf("choose a full folder path, such as %s", defaultInstallDir())
	}
	// Never install straight into a drive root or a system folder.
	if vol := filepath.VolumeName(dir); dir == vol+`\` || strings.EqualFold(dir, os.Getenv("SystemRoot")) {
		return fmt.Errorf("pick a dedicated folder, not %s", dir)
	}

	old := existingInstallDir()
	emit("Closing PC Optimizer if it is running…", 4)
	killApp(dir, old)

	emit("Creating the install folder…", 10)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("couldn't create %s: %w", dir, err)
	}

	emit("Copying PC Optimizer…", 25)
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

	emit("All done", 100)
	return nil
}

func uninstall(o Options, emit emitFn) error {
	dir := existingInstallDir()
	if dir == "" {
		self, _ := os.Executable()
		dir = filepath.Dir(self)
	}

	emit("Closing PC Optimizer…", 10)
	killApp(dir)

	emit("Removing shortcuts…", 35)
	_ = os.Remove(startMenuLink())
	_ = os.Remove(desktopLink())

	emit("Removing PC Optimizer from Windows…", 55)
	_ = registry.DeleteKey(registry.LOCAL_MACHINE, uninstallKey)

	if o.RemoveConfig {
		emit("Deleting your settings…", 70)
		_ = os.RemoveAll(filepath.Join(os.Getenv("APPDATA"), "pcoptimizer"))
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
	un := filepath.Join(dir, uninstallExe)
	if err := os.Remove(un); err != nil && !os.IsNotExist(err) {
		removeAtReboot(un)
	}
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		removeAtReboot(dir)
	}
}
