//go:build windows

package procs

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// sleeper starts a harmless child (ping to localhost) that lives for a while
// and is ended when the test finishes.
func sleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd
}

func exited(cmd *exec.Cmd) bool {
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

func alive(pid int) bool {
	ok, err := process.PidExists(int32(pid))
	return err == nil && ok
}

func TestKillEndsAProcessWhenTheNameMatches(t *testing.T) {
	cmd := sleeper(t)
	m := NewMonitor()
	if err := m.Kill(int32(cmd.Process.Pid), "PING.EXE"); err != nil {
		t.Fatalf("the name check is case-insensitive and should pass: %v", err)
	}
	if !exited(cmd) {
		t.Error("the helper should have been ended")
	}
}

func TestKillLeavesAProcessWhoseNameChanged(t *testing.T) {
	cmd := sleeper(t)
	m := NewMonitor()
	err := m.Kill(int32(cmd.Process.Pid), "notepad.exe")
	if err == nil || !strings.Contains(err.Error(), "nothing was ended") {
		t.Fatalf("a reused PID must be refused, got %v", err)
	}
	if !alive(cmd.Process.Pid) {
		t.Error("the process must still be running")
	}
}

func TestKillUnknownPID(t *testing.T) {
	m := NewMonitor()
	err := m.Kill(0x7FFFFFF0, "x.exe")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestKillRefusesLowPIDsWithoutLookingThemUp(t *testing.T) {
	m := NewMonitor()
	for _, pid := range []int32{0, 1, 2, 3, 4} {
		if err := m.Kill(pid, "System"); err == nil || !strings.Contains(err.Error(), "protected") {
			t.Errorf("pid %d: expected a protected error, got %v", pid, err)
		}
	}
}

func TestCriticalNamesAreLowerCase(t *testing.T) {
	for name := range critical {
		if name != strings.ToLower(name) {
			t.Errorf("%q must be lower-case, lookups lower-case the name first", name)
		}
	}
	for _, must := range []string{"explorer.exe", "csrss.exe", "lsass.exe", "winlogon.exe", "svchost.exe", "dwm.exe"} {
		if !critical[must] {
			t.Errorf("%s must be protected", must)
		}
	}
}

func TestKnownExplanationsAreWellFormed(t *testing.T) {
	for name, text := range known {
		if name != strings.ToLower(name) || strings.TrimSpace(text) == "" {
			t.Errorf("bad entry %q: %q", name, text)
		}
	}
}

func TestListMarksItselfProtected(t *testing.T) {
	m := NewMonitor()
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int32]bool{}
	var self *Process
	for i, p := range list {
		if seen[p.PID] || p.PID == 0 || p.Name == "" {
			t.Fatalf("bad entry: %+v", p)
		}
		seen[p.PID] = true
		if p.PID == int32(os.Getpid()) {
			self = &list[i]
		}
	}
	if self == nil {
		t.Fatal("the test process is not in the list")
	}
	if !self.Protected || self.Path == "" {
		t.Errorf("own process must be protected and have a path: %+v", *self)
	}
}

func TestListSecondCallMeasuresCPU(t *testing.T) {
	m := NewMonitor()
	if _, err := m.List(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p.CPUPercent < 0 || p.CPUPercent > 100.5 {
			t.Errorf("%s: CPU %.1f%% is outside 0-100", p.Name, p.CPUPercent)
		}
	}
	if len(m.handles) == 0 || len(m.meta) == 0 {
		t.Error("handles and metadata should be kept between calls")
	}
}

func TestListDropsHandlesOfExitedProcesses(t *testing.T) {
	cmd := sleeper(t)
	m := NewMonitor()
	if _, err := m.List(); err != nil {
		t.Fatal(err)
	}
	pid := int32(cmd.Process.Pid)
	if _, ok := m.handles[pid]; !ok {
		t.Skip("helper not listed (access rights)")
	}
	_ = cmd.Process.Kill()
	if !exited(cmd) {
		t.Fatal("helper did not exit")
	}
	if _, err := m.List(); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.handles[pid]; ok {
		t.Error("the handle of an exited process should be dropped")
	}
}

func TestOwnChildOnlyMatchesOwnWebViewHelpers(t *testing.T) {
	cmd := sleeper(t)
	h, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	self := int32(os.Getpid())
	if !ownChild(h, "msedgewebview2.exe", self) {
		t.Error("a WebView2 helper started by this app is its own child")
	}
	if ownChild(h, "msedgewebview2.exe", self+100000) {
		t.Error("a helper of another app is not ours")
	}
	if ownChild(h, "chrome.exe", self) {
		t.Error("only WebView2 helpers count")
	}
}

func TestLookupPrefersCuratedText(t *testing.T) {
	m := NewMonitor()
	h, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	got := m.lookup(h, "Explorer.EXE")
	if got.relatedTo != known["explorer.exe"] {
		t.Errorf("curated text should win, got %q", got.relatedTo)
	}
	if got.path == "" {
		t.Error("the path should be filled in")
	}
	if _, cached := m.vinfo[got.path]; !cached {
		t.Error("version info should be cached by path")
	}
}

func TestReadVersionInfo(t *testing.T) {
	windir := os.Getenv("SystemRoot")
	desc, vendor := readVersionInfo(windir + `\System32\cmd.exe`)
	if desc == "" || !strings.Contains(vendor, "Microsoft") {
		t.Errorf("cmd.exe should describe itself, got %q / %q", desc, vendor)
	}
	if d, v := readVersionInfo(windir + `\no-such-file.exe`); d != "" || v != "" {
		t.Errorf("a missing file has no version info, got %q / %q", d, v)
	}
	if d, v := readVersionInfo("bad\x00path"); d != "" || v != "" {
		t.Errorf("an invalid path has no version info, got %q / %q", d, v)
	}
}

func TestNewMonitorHasAtLeastOneThread(t *testing.T) {
	if NewMonitor().threads < 1 {
		t.Error("CPU percentages divide by the thread count, it must be at least 1")
	}
}
