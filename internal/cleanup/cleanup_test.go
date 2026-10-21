package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path string, size int, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(-age)
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
}

func TestWalkRespectsAgeAndLinks(t *testing.T) {
	root := t.TempDir()
	precious := t.TempDir() // stands in for user data outside the target

	write(t, filepath.Join(root, "old.tmp"), 100, 48*time.Hour)
	write(t, filepath.Join(root, "sub", "old2.tmp"), 50, 48*time.Hour)
	write(t, filepath.Join(root, "fresh.tmp"), 999, time.Minute)
	write(t, filepath.Join(precious, "keep.doc"), 10, 48*time.Hour)

	// A link inside the temp folder pointing at user data must not be followed.
	if err := os.Symlink(precious, filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(root, "sub"), old, old)

	tg := target{MinAge: 24 * time.Hour, Roots: func() []string { return []string{root} }}

	bytes, files := walk(tg, false, nil)
	if bytes != 150 || files != 2 {
		t.Fatalf("scan: got %d bytes / %d files, want 150 / 2", bytes, files)
	}

	var skipped int64
	walk(tg, true, &skipped)

	for _, p := range []string{"old.tmp", filepath.Join("sub", "old2.tmp"), "sub"} {
		if _, err := os.Stat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", p)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "fresh.tmp")); err != nil {
		t.Error("fresh file must be kept")
	}
	if _, err := os.Stat(filepath.Join(precious, "keep.doc")); err != nil {
		t.Error("file behind symlink must be kept")
	}
	if _, err := os.Stat(root); err != nil {
		t.Error("root folder itself must be kept")
	}
}
