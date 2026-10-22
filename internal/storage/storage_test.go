package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, p string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeAndLargest(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "big", "a.bin"), 5000)
	put(t, filepath.Join(root, "big", "deep", "b.bin"), 3000)
	put(t, filepath.Join(root, "small", "c.bin"), 100)
	put(t, filepath.Join(root, "loose.txt"), 50)

	res, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 8150 {
		t.Errorf("total: got %d want 8150", res.Total)
	}
	if len(res.Entries) != 3 || res.Entries[0].Name != "big" || res.Entries[0].Size != 8000 || res.Entries[0].Files != 2 {
		t.Errorf("entries: %+v", res.Entries)
	}

	files, partial, err := LargestFiles(root, 2)
	if err != nil || partial {
		t.Fatalf("largest: %v partial=%v", err, partial)
	}
	if len(files) != 2 || files[0].Size != 5000 || files[1].Size != 3000 {
		t.Errorf("largest files: %+v", files)
	}
}

func TestAnalyzeRejectsBadPaths(t *testing.T) {
	if _, err := Analyze("relative/path"); err == nil {
		t.Error("relative path must be rejected")
	}
	if _, err := Analyze(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("missing folder must be rejected")
	}
}
