package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"trimmel/internal/advisor"
	"trimmel/internal/alerts"
	"trimmel/internal/appinfo"
	"trimmel/internal/auth"
	"trimmel/internal/cleanup"
	"trimmel/internal/drivers"
	"trimmel/internal/elevation"
	"trimmel/internal/health"
	"trimmel/internal/license"
	"trimmel/internal/mover"
	"trimmel/internal/oldver"
	"trimmel/internal/overview"
	"trimmel/internal/procs"
	"trimmel/internal/schedule"
	"trimmel/internal/startup"
	"trimmel/internal/state"
	"trimmel/internal/storage"
)

// App is the single object bound to the frontend. Every exported method
// becomes window.go.main.App.<Method> in JavaScript and returns a Promise.
type App struct {
	ctx   context.Context
	procs *procs.Monitor
	auth  *auth.Manager
	scan  driverScan
	state *state.Store
	lic   *license.Manager
	sched *schedule.Store
	alert *alerts.Store
}

func NewApp() *App {
	a := &App{procs: procs.NewMonitor(), state: state.Open(""), lic: license.Open("", nil, nil), sched: schedule.Open(""), alert: alerts.Open("")}
	a.auth = auth.NewManager(func(url string) { runtime.BrowserOpenURL(a.ctx, url) })
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

var errSignedOut = errors.New("sign in required")

// guard blocks privileged calls until the user is signed in (when sign-in is
// configured), so the check holds even if the UI is bypassed.
func (a *App) guard() error {
	if !a.auth.Allowed() {
		return errSignedOut
	}
	return nil
}

// AuthStatus restores a previous session from the credential store if there
// is one, then reports whether the app is unlocked.
func (a *App) AuthStatus() auth.Status {
	if a.auth.Status().User == nil {
		return a.auth.Restore(a.ctx)
	}
	return a.auth.Status()
}

func (a *App) Login() (*auth.User, error) {
	return a.auth.Login(a.ctx)
}

func (a *App) CancelLogin() { a.auth.CancelLogin() }

func (a *App) Logout() { a.auth.Logout() }

func (a *App) IsAdmin() bool {
	return elevation.IsAdmin()
}

// OtherAccount returns the account the app runs as when that differs from the
// user signed in to Windows (a standard user who approved the admin prompt
// with an administrator's credentials), or "" when they match.
func (a *App) OtherAccount() string {
	if other, name := elevation.OtherAccount(); other {
		return name
	}
	return ""
}

type OverviewResult struct {
	System overview.Snapshot `json:"system"`
	Hints  []Hint            `json:"hints"`
}

type Hint struct {
	Level   string `json:"level"` // "info" | "warn"
	Message string `json:"message"`
}

func (a *App) GetOverview() (OverviewResult, error) {
	if err := a.guard(); err != nil {
		return OverviewResult{}, err
	}
	snap, err := overview.Collect()
	if err != nil {
		return OverviewResult{}, err
	}
	return OverviewResult{System: snap, Hints: a.hints(snap)}, nil
}

func (a *App) hints(s overview.Snapshot) []Hint {
	var hs []Hint
	for _, d := range s.Disks {
		if d.Total > 0 && float64(d.Free)/float64(d.Total) < 0.10 {
			hs = append(hs, Hint{"warn", fmt.Sprintf("%s has less than 10%% free space. Try the Cleanup tab.", d.Mount)})
		}
	}
	if s.Memory.UsedPercent > 85 {
		hs = append(hs, Hint{"warn", fmt.Sprintf("Memory is %.0f%% used. Check Processes for heavy apps.", s.Memory.UsedPercent)})
	}
	if items, err := startup.List(); err == nil {
		enabled := 0
		for _, it := range items {
			if it.Enabled {
				enabled++
			}
		}
		if enabled > 10 {
			hs = append(hs, Hint{"info", fmt.Sprintf("%d programs launch at startup. Disabling unneeded ones speeds up boot.", enabled)})
		}
	}
	if s.UptimeSeconds > 7*24*3600 {
		hs = append(hs, Hint{"info", "No restart in over a week. A reboot clears leaked memory and finishes pending updates."})
	}
	if name := a.OtherAccount(); name != "" {
		hs = append(hs, Hint{"warn", fmt.Sprintf("Trimmel is running as %s, not as the account you are signed in to Windows with. Cleanup, Startup and the Recycle Bin affect that account's files, not yours. To tidy your own PC, sign in to Windows with an administrator account, or ask an administrator to make yours one.", name)})
	}
	if !elevation.IsAdmin() {
		hs = append(hs, Hint{"info", "Not running with admin rights: system-wide cleanup, machine startup items and driver installs are limited. Start the app with “Run as administrator”."})
	}
	return hs
}

func (a *App) ListProcesses() ([]procs.Process, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return a.procs.List()
}

// KillProcess ends a process; name is what the list showed for pid and guards
// against the PID having been reused.
func (a *App) KillProcess(pid int32, name string) error {
	if err := a.guard(); err != nil {
		return err
	}
	return a.procs.Kill(pid, name)
}

func (a *App) ScanCleanup() ([]cleanup.TargetReport, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return cleanup.Scan(elevation.IsAdmin()), nil
}

func (a *App) RunCleanup(targetIDs []string) ([]cleanup.CleanResult, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	res := cleanup.Clean(targetIDs, elevation.IsAdmin())
	for _, r := range res {
		if r.Error == "" {
			_ = a.state.MarkTidy(time.Now())
			break
		}
	}
	return res, nil
}

func (a *App) ListStartup() ([]startup.Item, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return startup.List()
}

func (a *App) SetStartupEnabled(id string, enabled bool) error {
	if err := a.guard(); err != nil {
		return err
	}
	return startup.SetEnabled(id, enabled)
}

func (a *App) ListDrivers() ([]drivers.Driver, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return drivers.ListInstalled()
}

func (a *App) DriverScanState() (DriverScanState, error) {
	if err := a.guard(); err != nil {
		return DriverScanState{}, err
	}
	return a.scan.snapshot(), nil
}

// StartDriverScan checks Windows Update in the background (a minute or two),
// unless a recent result exists. force rechecks regardless. Poll DriverScanState.
func (a *App) StartDriverScan(force bool) (DriverScanState, error) {
	if err := a.guard(); err != nil {
		return DriverScanState{}, err
	}
	if a.scan.begin(force) {
		go func() { a.scan.finish(drivers.SearchUpdates()) }()
	}
	return a.scan.snapshot(), nil
}

// NetworkMetered reports whether Windows marks the current connection as
// metered, so the driver page can warn before a large download.
func (a *App) NetworkMetered() bool { return drivers.OnMeteredConnection() }

func (a *App) InstallDriverUpdates(updateIDs []string) (drivers.InstallResult, error) {
	if err := a.guard(); err != nil {
		return drivers.InstallResult{}, err
	}
	if !elevation.IsAdmin() {
		return drivers.InstallResult{}, fmt.Errorf("installing drivers needs admin rights; start the app with “Run as administrator”")
	}
	res, err := drivers.InstallUpdates(updateIDs, func(p drivers.Progress) {
		runtime.EventsEmit(a.ctx, "drivers:progress", p)
	})
	a.scan.remove(res.Installed)
	return res, err
}

func (a *App) ScanInstalls() ([]mover.Install, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return mover.Scan()
}

func (a *App) emitMove(p mover.Progress) { runtime.EventsEmit(a.ctx, "move:progress", p) }

// MoveInstall copies an install to destDrive, then links the old path to it.
// Progress arrives as "move:progress" events.
func (a *App) MoveInstall(path, destDrive string) (mover.Result, error) {
	if err := a.guard(); err != nil {
		return mover.Result{}, err
	}
	if err := a.checkMoveAllowance(path); err != nil {
		return mover.Result{}, err
	}
	return mover.Move(path, destDrive, a.emitMove)
}

// checkMoveAllowance applies the free-tier limit on moved apps. If the scan
// fails or does not list the path, it lets the move go on: the mover has its
// own safety checks, and a licence rule should never be the reason a user is
// stuck.
func (a *App) checkMoveAllowance(path string) error {
	tier := a.lic.Status(version).Tier
	if tier != license.Free {
		return nil
	}
	installs, err := mover.Scan()
	if err != nil {
		return nil
	}
	var count int
	var moved, size uint64
	var found bool
	for _, in := range installs {
		switch {
		case in.MovedTo != "" && !in.Broken:
			count++
			moved += in.Size
		case strings.EqualFold(in.Path, path):
			size, found = in.Size, true
		}
	}
	if !found {
		return nil
	}
	if ok, why := license.MoveAllowed(tier, count, moved, size); !ok {
		return errors.New(why)
	}
	return nil
}

func (a *App) MoveInstallBack(path string) (mover.Result, error) {
	if err := a.guard(); err != nil {
		return mover.Result{}, err
	}
	return mover.MoveBack(path, a.emitMove)
}

func (a *App) ScanOldVersions() ([]oldver.Old, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return oldver.Scan()
}

func (a *App) UninstallOldVersion(id string) error {
	if err := a.guard(); err != nil {
		return err
	}
	return oldver.Uninstall(id)
}

func (a *App) ScanLeftovers() ([]mover.Leftover, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return mover.Leftovers()
}

func (a *App) DeleteLeftover(path string) error {
	if err := a.guard(); err != nil {
		return err
	}
	return mover.DeleteLeftover(path)
}

// GetHealth combines a few quick measurements into a score and a short list
// of suggestions, each pointing at the tab that helps.
func (a *App) GetHealth() (health.Report, error) {
	if err := a.guard(); err != nil {
		return health.Report{}, err
	}
	snap, err := overview.Collect()
	if err != nil {
		return health.Report{}, err
	}
	in := health.Input{
		WorstDiskFreePct: 100,
		MemoryUsedPct:    snap.Memory.UsedPercent,
		UptimeDays:       float64(snap.UptimeSeconds) / 86400,
		RebootPending:    cleanup.RebootPending(),
	}
	for _, d := range snap.Disks {
		if d.Total == 0 {
			continue
		}
		if pct := float64(d.Free) / float64(d.Total) * 100; pct < in.WorstDiskFreePct {
			in.WorstDiskFreePct, in.WorstDiskMount = pct, d.Mount
		}
	}
	if items, err := startup.List(); err == nil {
		for _, it := range items {
			if it.Enabled {
				in.StartupEnabled++
				if it.Recommend == "disable" {
					in.StartupSuggested++
				}
			}
		}
	}
	st := a.state.Get()
	in.DaysSinceTidy = st.DaysSinceTidy(time.Now())
	in.TidyReminderOn = st.ReminderDays > 0
	return health.Evaluate(in), nil
}

type Prefs struct {
	Welcomed      bool `json:"welcomed"`
	ReminderDays  int  `json:"reminderDays"`
	DaysSinceTidy int  `json:"daysSinceTidy"` // -1 = never
}

func (a *App) GetPrefs() Prefs {
	d := a.state.Get()
	return Prefs{Welcomed: d.Welcomed, ReminderDays: d.ReminderDays, DaysSinceTidy: d.DaysSinceTidy(time.Now())}
}

func (a *App) SetReminderDays(n int) error { return a.state.SetReminderDays(n) }
func (a *App) MarkWelcomed() error         { return a.state.MarkWelcomed() }

func (a *App) AnalyzeFolder(path string) (storage.Result, error) {
	if err := a.guard(); err != nil {
		return storage.Result{}, err
	}
	return storage.Analyze(path)
}

type LargestFilesResult struct {
	Files   []storage.Entry `json:"files"`
	Partial bool            `json:"partial"`
}

func (a *App) LargestFiles(path string) (LargestFilesResult, error) {
	if err := a.guard(); err != nil {
		return LargestFilesResult{}, err
	}
	files, partial, err := storage.LargestFiles(path, 50)
	return LargestFilesResult{Files: files, Partial: partial}, err
}

// RevealInExplorer opens Explorer with path selected. It never opens or
// changes the file.
func (a *App) RevealInExplorer(path string) error {
	if err := a.guard(); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("that item no longer exists")
	}
	return exec.Command("explorer.exe", "/select,"+path).Start()
}

func (a *App) ListApps() ([]oldver.App, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return oldver.ListApps(), nil
}

func (a *App) UninstallApp(id string) error {
	if err := a.guard(); err != nil {
		return err
	}
	return oldver.UninstallApp(id)
}

type AppInfo struct {
	Version string          `json:"version"`
	Admin   bool            `json:"admin"`
	Install appinfo.Install `json:"install"`
}

func (a *App) AppInfo() (AppInfo, error) {
	in, err := appinfo.Current()
	if err != nil {
		return AppInfo{}, err
	}
	return AppInfo{Version: version, Admin: elevation.IsAdmin(), Install: in}, nil
}

func (a *App) CheckUpdate() (appinfo.Update, error) {
	return appinfo.CheckUpdate(a.ctx, version)
}

// LicenseStatus reports whether Plus is active, in trial or off. The free app
// does not depend on it; nothing is locked by this call.
func (a *App) LicenseStatus() license.Status { return a.lic.Status(version) }

func (a *App) StartTrial() (license.Status, error) {
	if err := a.lic.StartTrial(); err != nil {
		return a.lic.Status(version), err
	}
	return a.lic.Status(version), nil
}

// ActivateLicense saves a pasted key. When the app has Google sign-in, the key must belong to the
// signed-in account; a build without sign-in has no account to match.
func (a *App) ActivateLicense(key string) (license.Status, error) {
	st := a.auth.Status()
	if !st.Configured {
		return a.lic.Activate(key, version)
	}
	if st.User == nil {
		return a.lic.Status(version), errSignedOut
	}
	return a.lic.ActivateFor(key, version, &license.Holder{Subject: st.User.Subject, Email: st.User.Email})
}

func (a *App) RemoveLicense() (license.Status, error) { return a.lic.Deactivate(version) }

// GetSchedule reports the scheduled-cleanup switch and the last quiet run.
func (a *App) GetSchedule() schedule.View { return schedule.ViewOf(a.sched.Load()) }

// SetSchedule switches scheduled cleanup on or off. Turning it on needs Plus or
// the trial; turning it off is always allowed. The Windows task is created
// first, so a failure leaves the saved setting unchanged.
func (a *App) SetSchedule(enabled bool, frequency string) (schedule.View, error) {
	cur := a.sched.Load()
	if enabled && a.lic.Status(version).Tier == license.Free {
		return schedule.ViewOf(cur), errors.New("scheduled cleanup is part of Trimmel Plus. See Plus in Settings, or start the free trial")
	}
	var err error
	if enabled {
		var exe string
		if exe, err = os.Executable(); err == nil {
			err = schedule.Install(exe, frequency)
		}
	} else {
		err = schedule.Remove()
	}
	if err != nil {
		return schedule.ViewOf(cur), err
	}
	st, err := a.sched.Configure(enabled, frequency)
	return schedule.ViewOf(st), err
}

// GetAlerts reports the low-disk and weekly-summary switches.
func (a *App) GetAlerts() alerts.View { return alerts.ViewOf(a.alert.Load()) }

// SetAlerts saves the alert switches and creates or removes the daily check
// task to match. Turning anything on needs Plus or the trial; turning
// everything off is always allowed. The task is changed first, so a failure
// leaves the saved settings unchanged.
func (a *App) SetAlerts(lowDisk bool, threshold int, report bool) (alerts.View, error) {
	cur := a.alert.Load()
	if (lowDisk || report) && a.lic.Status(version).Tier == license.Free {
		return alerts.ViewOf(cur), errors.New("alerts are part of Trimmel Plus. See Plus in Settings, or start the free trial")
	}
	var err error
	if lowDisk || report {
		var exe string
		if exe, err = os.Executable(); err == nil {
			err = schedule.InstallCheck(exe)
		}
	} else {
		err = schedule.RemoveCheck()
	}
	if err != nil {
		return alerts.ViewOf(cur), err
	}
	st, err := a.alert.Configure(lowDisk, threshold, report, time.Now())
	return alerts.ViewOf(st), err
}

func (a *App) OpenPlusPage() {
	runtime.BrowserOpenURL(a.ctx, appinfo.SiteURL+"/plus.html")
}

// OpenAccountPage opens the website account page, where Plus is bought and the key is shown.
func (a *App) OpenAccountPage() {
	runtime.BrowserOpenURL(a.ctx, appinfo.SiteURL+"/account.html")
}

func (a *App) OpenDownloadPage() {
	runtime.BrowserOpenURL(a.ctx, appinfo.SiteURL+"/#download")
}

// OpenFolder shows the install or settings folder in Explorer. Only those two
// are allowed, never a path from the UI.
func (a *App) OpenFolder(kind string) error {
	in, err := appinfo.Current()
	if err != nil {
		return err
	}
	dir := in.Dir
	if kind == "settings" {
		dir = in.SettingsDir
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	} else if kind != "install" {
		return fmt.Errorf("unknown folder %q", kind)
	}
	return exec.Command("explorer.exe", dir).Start()
}

// RunSetup hands over to setup's own window to move or remove the install,
// then closes the app so its files are free.
func (a *App) RunSetup(mode string) error {
	if err := a.guard(); err != nil {
		return err
	}
	flag := map[string]string{"modify": "--modify", "uninstall": "--uninstall"}[mode]
	if flag == "" {
		return fmt.Errorf("unknown setup action %q", mode)
	}
	in, err := appinfo.Current()
	if err != nil {
		return err
	}
	if !in.Installed || !in.HasUninstall {
		return errors.New("this copy was not installed with setup, so there is nothing to change here")
	}
	cmd := exec.Command(appinfo.UninstallerPath(in.Dir), flag)
	cmd.Dir = in.Dir
	if err := cmd.Start(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

// DeleteSettings signs out and removes everything the app stored for this user.
func (a *App) DeleteSettings() error {
	if err := a.guard(); err != nil {
		return err
	}
	a.auth.Logout()
	err := os.RemoveAll(appinfo.SettingsDir())
	_ = os.RemoveAll(appinfo.LegacySettingsDir())
	return err
}

// GetStorageSuggestions looks at one drive (for example "C:") and returns
// concrete things the user could do to free space on it, biggest first.
func (a *App) GetStorageSuggestions(drive string) ([]advisor.Suggestion, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	snap, err := overview.Collect()
	if err != nil {
		return nil, err
	}
	var others []advisor.Drive
	for _, d := range snap.Disks {
		if !strings.EqualFold(d.Mount, drive) {
			others = append(others, advisor.Drive{Mount: d.Mount, Free: d.Free})
		}
	}
	return advisor.Suggest(advisor.Options{Drive: drive, Others: others, IsAdmin: elevation.IsAdmin()}), nil
}

// runScheduled is "trimmel.exe --scheduled": clean the safe targets quietly,
// record the result and exit. It opens no window.
func runScheduled() {
	tier := license.Open("", nil, nil).Status(version).Tier
	schedule.Open("").Run(tier, elevation.IsAdmin(), time.Now(), cleanup.Clean)
}

// runCheck is "trimmel.exe --check": send any low-disk or weekly notification
// that is due, then exit. It opens no window.
func runCheck() {
	snap, _ := overview.Collect()
	disks := make([]alerts.Disk, 0, len(snap.Disks))
	for _, d := range snap.Disks {
		disks = append(disks, alerts.Disk{Mount: d.Mount, Total: d.Total, Free: d.Free})
	}
	tier := license.Open("", nil, nil).Status(version).Tier
	alerts.Open("").Check(tier, time.Now(), disks, schedule.Open("").Load().LastFreed, showToast)
}
