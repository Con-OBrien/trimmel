package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the setup UI.
type App struct {
	ctx  context.Context
	mode string // "install", "modify" (change location) or "uninstall"
	busy bool
}

func NewApp(mode string) *App { return &App{mode: mode} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// Info describes what the UI should show.
type Info struct {
	Version    string `json:"version"`
	Mode       string `json:"mode"` // "install", "modify" or "uninstall"
	DefaultDir string `json:"defaultDir"`
	Existing   string `json:"existing"` // install dir of a current install, or ""
	SizeMB     int    `json:"sizeMB"`
}

func (a *App) Info() Info {
	existing := existingInstallDir()
	dir := defaultInstallDir()
	if existing != "" {
		dir = existing
	}
	return Info{Version: version, Mode: a.mode, DefaultDir: dir, Existing: existing, SizeMB: payloadSizeMB()}
}

// Options chosen on the options screen.
type Options struct {
	Dir          string `json:"dir"`
	DesktopLink  bool   `json:"desktopLink"`
	RemoveConfig bool   `json:"removeConfig"`
}

func (a *App) emit(stage string, percent float64) {
	runtime.EventsEmit(a.ctx, "setup:progress", map[string]any{"stage": stage, "percent": percent})
}

func (a *App) Install(o Options) error {
	if a.busy {
		return fmt.Errorf("setup is already running")
	}
	a.busy = true
	defer func() { a.busy = false }()
	return install(o, a.emit)
}

func (a *App) Uninstall(o Options) error {
	if a.busy {
		return fmt.Errorf("setup is already running")
	}
	a.busy = true
	defer func() { a.busy = false }()
	return uninstall(o, a.emit)
}

// PickDir opens a folder chooser and returns the chosen folder ("" if cancelled).
func (a *App) PickDir(start string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose where to install Trimmel",
		DefaultDirectory: start,
	})
}

// Launch starts the installed app (it asks for admin itself) and closes setup.
func (a *App) Launch(dir string) error {
	exe := filepath.Join(dir, appExe)
	cmd := exec.Command("cmd", "/c", "start", "", exe)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func (a *App) Minimise() { runtime.WindowMinimise(a.ctx) }

func (a *App) Close() {
	if a.busy {
		return
	}
	runtime.Quit(a.ctx)
}
