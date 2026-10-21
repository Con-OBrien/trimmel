//go:build windows

package mover

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path string, n int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChooseDest(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src", "App")
	write(t, filepath.Join(src, "a.bin"), 10)
	write(t, filepath.Join(src, "sub", "b.bin"), 20)
	dest := filepath.Join(base, "dst", "App")

	// Free: used as is.
	if got, _ := chooseDest(dest, src); got != dest {
		t.Fatalf("free: %s", got)
	}

	// Leftover from an interrupted move of the same install: reused.
	write(t, filepath.Join(dest, "a.bin"), 10)
	if got, _ := chooseDest(dest, src); got != dest {
		t.Fatalf("partial copy should be reused, got %s", got)
	}

	// A different program with the same folder name: not touched, new name.
	write(t, filepath.Join(dest, "other.dll"), 5)
	got, err := chooseDest(dest, src)
	if err != nil || got != dest+" (2)" {
		t.Fatalf("collision: %s %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "other.dll")); err != nil {
		t.Fatal("collision folder was modified")
	}
}
