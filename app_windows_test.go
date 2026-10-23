//go:build windows

package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trimmel/internal/auth"
	"trimmel/internal/overview"
)

// newTestApp builds an App whose settings live in a temp folder. With
// signedIn false the build is treated as one with Google sign-in configured,
// so every guarded call is refused.
func newTestApp(t *testing.T, signInConfigured bool) *App {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	for _, k := range []string{"TRIMMEL_OIDC_ISSUER", "TRIMMEL_OIDC_CLIENT_ID", "TRIMMEL_OIDC_CLIENT_SECRET", "PCOPT_OIDC_ISSUER", "PCOPT_OIDC_CLIENT_ID", "PCOPT_OIDC_CLIENT_SECRET"} {
		t.Setenv(k, "")
	}
	old := auth.GoogleClientID
	auth.GoogleClientID = ""
	if signInConfigured {
		auth.GoogleClientID = "test-client-id"
	}
	t.Cleanup(func() { auth.GoogleClientID = old })
	a := NewApp()
	a.startup(context.Background())
	return a
}

func TestEveryGuardedCallIsRefusedWhenSignedOut(t *testing.T) {
	a := newTestApp(t, true)
	calls := map[string]func() error{
		"GetOverview":           func() error { _, err := a.GetOverview(); return err },
		"ListProcesses":         func() error { _, err := a.ListProcesses(); return err },
		"KillProcess":           func() error { return a.KillProcess(1234, "x.exe") },
		"ScanCleanup":           func() error { _, err := a.ScanCleanup(); return err },
		"RunCleanup":            func() error { _, err := a.RunCleanup([]string{"user-temp"}); return err },
		"ListStartup":           func() error { _, err := a.ListStartup(); return err },
		"SetStartupEnabled":     func() error { return a.SetStartupEnabled("x", false) },
		"ListDrivers":           func() error { _, err := a.ListDrivers(); return err },
		"DriverScanState":       func() error { _, err := a.DriverScanState(); return err },
		"StartDriverScan":       func() error { _, err := a.StartDriverScan(false); return err },
		"InstallDriverUpdates":  func() error { _, err := a.InstallDriverUpdates([]string{"x"}); return err },
		"ScanInstalls":          func() error { _, err := a.ScanInstalls(); return err },
		"MoveInstall":           func() error { _, err := a.MoveInstall(`C:\x`, `D:\`); return err },
		"MoveInstallBack":       func() error { _, err := a.MoveInstallBack(`C:\x`); return err },
		"ScanOldVersions":       func() error { _, err := a.ScanOldVersions(); return err },
		"UninstallOldVersion":   func() error { return a.UninstallOldVersion("x") },
		"ScanLeftovers":         func() error { _, err := a.ScanLeftovers(); return err },
		"DeleteLeftover":        func() error { return a.DeleteLeftover(`C:\x`) },
		"GetHealth":             func() error { _, err := a.GetHealth(); return err },
		"AnalyzeFolder":         func() error { _, err := a.AnalyzeFolder(`C:\`); return err },
		"LargestFiles":          func() error { _, err := a.LargestFiles(`C:\`); return err },
		"RevealInExplorer":      func() error { return a.RevealInExplorer(`C:\`) },
		"ListApps":              func() error { _, err := a.ListApps(); return err },
		"UninstallApp":          func() error { return a.UninstallApp("x") },
		"RunSetup":              func() error { return a.RunSetup("uninstall") },
		"DeleteSettings":        func() error { return a.DeleteSettings() },
		"GetStorageSuggestions": func() error { _, err := a.GetStorageSuggestions("C:"); return err },
	}
	for name, call := range calls {
		if err := call(); err != errSignedOut {
			t.Errorf("%s: want errSignedOut, got %v", name, err)
		}
	}
}

func TestAuthStatusWithoutSignInConfigured(t *testing.T) {
	a := newTestApp(t, false)
	if st := a.AuthStatus(); st.Configured {
		t.Errorf("a build without a client ID has no sign-in: %+v", st)
	}
	a.CancelLogin()
	a.Logout()
	if err := a.guard(); err != nil {
		t.Errorf("without sign-in the app is open, got %v", err)
	}
}

func TestAuthStatusSignedOutWhenConfigured(t *testing.T) {
	a := newTestApp(t, true)
	st := a.AuthStatus()
	if !st.Configured || st.User != nil {
		t.Errorf("configured sign-in with nobody signed in: %+v", st)
	}
}

func TestReadOnlyCallsWork(t *testing.T) {
	a := newTestApp(t, false)

	if o, err := a.GetOverview(); err != nil || o.System.Hostname == "" {
		t.Errorf("overview: %v %+v", err, o.System)
	}
	if list, err := a.ListProcesses(); err != nil || len(list) == 0 {
		t.Errorf("processes: %v (%d)", err, len(list))
	}
	if reports, err := a.ScanCleanup(); err != nil || len(reports) == 0 {
		t.Errorf("cleanup scan: %v (%d)", err, len(reports))
	}
	if res, err := a.RunCleanup(nil); err != nil || len(res) != 0 {
		t.Errorf("cleaning nothing: %v %+v", err, res)
	}
	if a.state.Get().LastTidy.After(time.Time{}) {
		t.Error("cleaning nothing must not count as a tidy")
	}
	if _, err := a.ListStartup(); err != nil {
		t.Errorf("startup: %v", err)
	}
	if _, err := a.ListDrivers(); err != nil {
		t.Logf("drivers unavailable here: %v", err)
	}
	if st, err := a.DriverScanState(); err != nil || st.Checking || st.CheckedAt != 0 {
		t.Errorf("no scan has run: %+v %v", st, err)
	}
	_ = a.NetworkMetered()
	if _, err := a.ScanInstalls(); err != nil {
		t.Errorf("installs: %v", err)
	}
	if _, err := a.ScanOldVersions(); err != nil {
		t.Errorf("old versions: %v", err)
	}
	if _, err := a.ScanLeftovers(); err != nil {
		t.Errorf("leftovers: %v", err)
	}
	if _, err := a.ListApps(); err != nil {
		t.Errorf("apps: %v", err)
	}
	if r, err := a.GetHealth(); err != nil || r.Score <= 0 {
		t.Errorf("health: %v %+v", err, r)
	}
	_ = a.IsAdmin()
	_ = a.OtherAccount()
}

func TestRefusedRequestsReturnErrors(t *testing.T) {
	a := newTestApp(t, false)

	if err := a.KillProcess(0, "System"); err == nil {
		t.Error("PID 0 must be refused")
	}
	if err := a.SetStartupEnabled("no-such-item", false); err == nil {
		t.Error("an unknown startup item must be an error")
	}
	if err := a.UninstallOldVersion("no-such-id"); err == nil {
		t.Error("an unknown old version must be an error")
	}
	if err := a.UninstallApp("no-such-id"); err == nil {
		t.Error("an unknown program must be an error")
	}
	if err := a.DeleteLeftover(filepath.Join(t.TempDir(), "x")); err == nil {
		t.Error("a folder outside the hold area must be refused")
	}
	if err := a.RevealInExplorer(filepath.Join(t.TempDir(), "gone")); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("a missing path should be reported, got %v", err)
	}
	if err := a.OpenFolder("elsewhere"); err == nil || !strings.Contains(err.Error(), "unknown folder") {
		t.Errorf("only install and settings are allowed, got %v", err)
	}
	if err := a.RunSetup("explode"); err == nil || !strings.Contains(err.Error(), "unknown setup action") {
		t.Errorf("unknown setup action, got %v", err)
	}
	if err := a.RunSetup("modify"); err == nil || !strings.Contains(err.Error(), "not installed with setup") {
		t.Errorf("a copy not installed by setup has nothing to modify, got %v", err)
	}
	if a.IsAdmin() {
		return
	}
	if _, err := a.InstallDriverUpdates([]string{"x"}); err == nil || !strings.Contains(err.Error(), "admin") {
		t.Errorf("driver installs need admin, got %v", err)
	}
}

func TestPrefsRoundTrip(t *testing.T) {
	a := newTestApp(t, false)
	if p := a.GetPrefs(); p.Welcomed || p.ReminderDays != 14 || p.DaysSinceTidy != -1 {
		t.Errorf("defaults: %+v", p)
	}
	if err := a.SetReminderDays(30); err != nil {
		t.Fatal(err)
	}
	if err := a.MarkWelcomed(); err != nil {
		t.Fatal(err)
	}
	if p := a.GetPrefs(); !p.Welcomed || p.ReminderDays != 30 {
		t.Errorf("saved prefs: %+v", p)
	}
	if err := a.SetReminderDays(-1); err == nil {
		t.Error("a negative reminder must be refused")
	}
}

func TestFolderAnalysis(t *testing.T) {
	a := newTestApp(t, false)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.bin"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := a.AnalyzeFolder(dir)
	if err != nil || res.Total != 4096 {
		t.Errorf("analyze: %v %+v", err, res)
	}
	lf, err := a.LargestFiles(dir)
	if err != nil || len(lf.Files) != 1 || lf.Partial {
		t.Errorf("largest: %v %+v", err, lf)
	}
	if _, err := a.AnalyzeFolder("relative"); err == nil {
		t.Error("a relative path must be refused")
	}
}

func TestStorageSuggestionsForTempDrive(t *testing.T) {
	a := newTestApp(t, false)
	drive := filepath.VolumeName(os.TempDir())
	if _, err := a.GetStorageSuggestions(drive); err != nil {
		t.Errorf("suggestions: %v", err)
	}
}

func TestAppInfoAndUpdateCheck(t *testing.T) {
	a := newTestApp(t, false)
	info, err := a.AppInfo()
	if err != nil || info.Version != version || info.Install.Exe == "" {
		t.Errorf("app info: %v %+v", err, info)
	}

	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"version":"99.0.0"}`)), Header: http.Header{}}, nil
	})
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	u, err := a.CheckUpdate()
	if err != nil || u.Latest != "99.0.0" {
		t.Errorf("update check: %v %+v", err, u)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDeleteSettingsRemovesStoredFiles(t *testing.T) {
	a := newTestApp(t, false)
	if err := a.MarkWelcomed(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(filepathOfState(a))
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("settings folder should exist: %v", err)
	}
	if err := a.DeleteSettings(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("settings folder should be gone: %v", err)
	}
}

func filepathOfState(a *App) string {
	return filepath.Join(os.Getenv("APPDATA"), "trimmel", "state.json")
}

func TestHintsReflectTheSnapshot(t *testing.T) {
	a := newTestApp(t, false)
	snap := overview.Snapshot{
		Disks:         []overview.Disk{{Mount: "C:", Total: 100, Free: 5}, {Mount: "D:", Total: 100, Free: 90}, {Mount: "E:"}},
		Memory:        overview.MemInfo{UsedPercent: 92},
		UptimeSeconds: 8 * 24 * 3600,
	}
	var joined string
	for _, h := range a.hints(snap) {
		if h.Level != "info" && h.Level != "warn" {
			t.Errorf("bad level %q", h.Level)
		}
		joined += h.Message + "\n"
	}
	for _, want := range []string{"C: has less than 10% free", "Memory is 92% used", "No restart in over a week"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing hint %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "D: has") || strings.Contains(joined, "E: has") {
		t.Errorf("healthy and empty drives must not be flagged:\n%s", joined)
	}

	calm := a.hints(overview.Snapshot{Disks: []overview.Disk{{Mount: "C:", Total: 100, Free: 50}}})
	for _, h := range calm {
		if strings.Contains(h.Message, "free space") || strings.Contains(h.Message, "Memory is") || strings.Contains(h.Message, "restart") {
			t.Errorf("nothing to warn about, got %q", h.Message)
		}
	}
}
