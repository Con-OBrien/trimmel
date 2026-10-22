package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"pcoptimizer/internal/auth"
	"pcoptimizer/internal/cleanup"
	"pcoptimizer/internal/drivers"
	"pcoptimizer/internal/elevation"
	"pcoptimizer/internal/mover"
	"pcoptimizer/internal/oldver"
	"pcoptimizer/internal/overview"
	"pcoptimizer/internal/procs"
	"pcoptimizer/internal/startup"
)

// App is the single object bound to the frontend. Every exported method
// becomes window.go.main.App.<Method> in JavaScript and returns a Promise.
type App struct {
	ctx   context.Context
	procs *procs.Monitor
	auth  *auth.Manager
	scan  driverScan
}

func NewApp() *App {
	a := &App{procs: procs.NewMonitor()}
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

// ---- Sign-in (OpenID Connect) ----

// AuthStatus restores a previous session from the credential store if there
// is one, then reports whether the app is unlocked.
func (a *App) AuthStatus() auth.Status {
	if a.auth.Status().User == nil {
		return a.auth.Restore(a.ctx)
	}
	return a.auth.Status()
}

// Login opens the system browser and blocks until sign-in finishes.
func (a *App) Login() (*auth.User, error) {
	return a.auth.Login(a.ctx)
}

func (a *App) CancelLogin() { a.auth.CancelLogin() }

func (a *App) Logout() { a.auth.Logout() }

// ---- Elevation ----

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

// ---- Overview ----

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
		hs = append(hs, Hint{"warn", fmt.Sprintf("PC Optimizer is running as %s, not as the account you are signed in to Windows with. Cleanup, Startup and the Recycle Bin affect that account's files, not yours. To tidy your own PC, sign in to Windows with an administrator account, or ask an administrator to make yours one.", name)})
	}
	if !elevation.IsAdmin() {
		hs = append(hs, Hint{"info", "Not running with admin rights: system-wide cleanup, machine startup items and driver installs are limited. Start the app with “Run as administrator”."})
	}
	return hs
}

// ---- Processes ----

func (a *App) ListProcesses() ([]procs.Process, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return a.procs.List()
}

func (a *App) KillProcess(pid int32, name string) error {
	if err := a.guard(); err != nil {
		return err
	}
	return a.procs.Kill(pid, name)
}

// ---- Cleanup ----

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
	return cleanup.Clean(targetIDs, elevation.IsAdmin()), nil
}

// ---- Startup ----

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

// ---- Drivers ----

func (a *App) ListDrivers() ([]drivers.Driver, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return drivers.ListInstalled()
}

// DriverScanState returns the cached result of the last driver check.
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

func (a *App) InstallDriverUpdates(updateIDs []string) (drivers.InstallResult, error) {
	if err := a.guard(); err != nil {
		return drivers.InstallResult{}, err
	}
	if !elevation.IsAdmin() {
		return drivers.InstallResult{}, fmt.Errorf("installing drivers needs admin rights; start the app with “Run as administrator”")
	}
	res, err := drivers.InstallUpdates(updateIDs)
	a.scan.remove(res.Installed)
	return res, err
}

// ---- Move installs ----

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
	return mover.Move(path, destDrive, a.emitMove)
}

func (a *App) MoveInstallBack(path string) (mover.Result, error) {
	if err := a.guard(); err != nil {
		return mover.Result{}, err
	}
	return mover.MoveBack(path, a.emitMove)
}

// ---- Old versions ----

func (a *App) ScanOldVersions() ([]oldver.Old, error) {
	if err := a.guard(); err != nil {
		return nil, err
	}
	return oldver.Scan()
}

// UninstallOldVersion launches the program's own uninstaller and waits for it.
func (a *App) UninstallOldVersion(id string) error {
	if err := a.guard(); err != nil {
		return err
	}
	return oldver.Uninstall(id)
}

// ScanLeftovers lists copies left on other drives by failed or undone moves.
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
