// Package appdir is the per-user folder where Trimmel keeps its settings
// (sign-in configuration, preferences, history). The app was called PC
// Optimizer before version 0.2.0 and used a "pcoptimizer" folder; the first
// time the new folder is wanted, the old one is copied across so nobody loses
// their settings. The old folder is left in place.
package appdir

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const (
	Name   = "trimmel"
	Legacy = "pcoptimizer"
)

var once sync.Once

func base() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return dir
}

// Dir returns the settings folder, copying the legacy one over on first use.
func Dir() string {
	b := base()
	once.Do(func() { migrate(b) })
	return filepath.Join(b, Name)
}

func LegacyDir() string { return filepath.Join(base(), Legacy) }

// migrate copies <base>\pcoptimizer to <base>\trimmel if the latter does not
// exist yet. It is best effort: a failure just means the user starts fresh.
func migrate(base string) {
	newDir, oldDir := filepath.Join(base, Name), filepath.Join(base, Legacy)
	if _, err := os.Stat(newDir); err == nil {
		return
	}
	if fi, err := os.Stat(oldDir); err != nil || !fi.IsDir() {
		return
	}
	if err := copyTree(oldDir, newDir); err != nil {
		_ = os.RemoveAll(newDir) // don't leave a half-copied folder that blocks a retry
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
