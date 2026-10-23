//go:build windows

package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirMigratesLegacyOnFirstUse(t *testing.T) {
	appdata := t.TempDir()
	t.Setenv("APPDATA", appdata)
	legacy := filepath.Join(appdata, Legacy)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "state.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got, want := Dir(), filepath.Join(appdata, Name); got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(appdata, Name, "state.json")); err != nil {
		t.Errorf("legacy settings should be copied across: %v", err)
	}
	if got, want := LegacyDir(), legacy; got != want {
		t.Errorf("LegacyDir() = %q, want %q", got, want)
	}
}

func TestBaseFallsBackWhenNoConfigDir(t *testing.T) {
	t.Setenv("APPDATA", "")
	if got := base(); got != "." {
		t.Errorf("without APPDATA the base should be the current folder, got %q", got)
	}
}
