//go:build windows

package advisor

import (
	"os"
	"path/filepath"
	"testing"
)

func bigFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Skipf("cannot create a %d byte file here: %v", size, err)
	}
}

func fakeProfile(t *testing.T) (profile, local, drive string) {
	t.Helper()
	profile = t.TempDir()
	local = filepath.Join(profile, "AppData", "Local")
	t.Setenv("USERPROFILE", profile)
	t.Setenv("LOCALAPPDATA", local)
	return profile, local, filepath.VolumeName(profile)
}

func find(out []Suggestion, id string) (Suggestion, bool) {
	for _, s := range out {
		if s.ID == id {
			return s, true
		}
	}
	return Suggestion{}, false
}

func TestSystemDriveUsesEnvironment(t *testing.T) {
	t.Setenv("SystemDrive", "d:")
	if got := systemDrive(); got != "D:" {
		t.Errorf("got %q, want D:", got)
	}
	t.Setenv("SystemDrive", "")
	if got := systemDrive(); got != "C:" {
		t.Errorf("default should be C:, got %q", got)
	}
}

func TestFolderSuggestionsFindsBigPersonalFolder(t *testing.T) {
	profile, _, drive := fakeProfile(t)
	bigFile(t, filepath.Join(profile, "Downloads", "image.iso"), minFolder+1)

	out := folderSuggestions(Options{Drive: drive})
	s, ok := find(out, "downloads")
	if !ok {
		t.Fatalf("Downloads not suggested: %+v", out)
	}
	if s.Action != "review" || s.Safety != "review" || s.Size < minFolder || s.Path != filepath.Join(profile, "Downloads") {
		t.Errorf("unexpected suggestion: %+v", s)
	}
}

func TestFolderSuggestionsSkipsSmallFolders(t *testing.T) {
	profile, _, drive := fakeProfile(t)
	bigFile(t, filepath.Join(profile, "Downloads", "small.bin"), 1<<20)
	if out := folderSuggestions(Options{Drive: drive}); len(out) != 0 {
		t.Errorf("a 1 MB folder is not worth a suggestion, got %+v", out)
	}
}

func TestFolderSuggestionsIgnoresOtherDrives(t *testing.T) {
	profile, _, _ := fakeProfile(t)
	bigFile(t, filepath.Join(profile, "Downloads", "image.iso"), minFolder+1)
	if out := folderSuggestions(Options{Drive: "Q:"}); len(out) != 0 {
		t.Errorf("a folder on another drive must not be suggested for Q:, got %+v", out)
	}
}

func TestFolderSuggestionsNeedsAProfile(t *testing.T) {
	t.Setenv("USERPROFILE", "")
	if out := folderSuggestions(Options{Drive: "C:"}); out != nil {
		t.Errorf("no profile, no suggestions, got %+v", out)
	}
}

func TestDeveloperCacheIsMarkedSafe(t *testing.T) {
	_, local, drive := fakeProfile(t)
	bigFile(t, filepath.Join(local, "npm-cache", "pack.bin"), minCache+1)

	s, ok := find(folderSuggestions(Options{Drive: drive}), "npm")
	if !ok {
		t.Fatal("npm cache not suggested")
	}
	if s.Safety != "safe" {
		t.Errorf("a rebuildable cache should be safe, got %q", s.Safety)
	}
}

func TestBestDestRejectsFullAndUnknownDrives(t *testing.T) {
	if got := bestDest(nil, 1<<30); got != "" {
		t.Errorf("no drives, no destination, got %q", got)
	}
	if got := bestDest([]Drive{{Mount: "Q:", Free: 1 << 40}}, 1<<30); got != "" {
		t.Errorf("a drive that does not exist cannot hold a program, got %q", got)
	}
	if got := bestDest([]Drive{{Mount: filepath.VolumeName(os.TempDir()), Free: 10}}, 1<<30); got != "" {
		t.Errorf("a drive without room must be refused, got %q", got)
	}
}

func TestSystemSuggestionsOnlyForSystemDrive(t *testing.T) {
	t.Setenv("SystemDrive", "C:")
	if out := systemSuggestions(Options{Drive: "D:"}); out != nil {
		t.Errorf("system suggestions belong to the system drive, got %+v", out)
	}
}

func TestCleanupSuggestionsOnlyForSystemDrive(t *testing.T) {
	t.Setenv("SystemDrive", "C:")
	if out := cleanupSuggestions(Options{Drive: "D:"}); out != nil {
		t.Errorf("Cleanup targets live on the system drive, got %+v", out)
	}
}

func TestSuggestIsCappedAndSorted(t *testing.T) {
	out := Suggest(Options{Drive: "Q:"})
	if len(out) > maxResults {
		t.Errorf("got %d suggestions, cap is %d", len(out), maxResults)
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].Size < out[i].Size {
			t.Errorf("not sorted biggest first at %d", i)
		}
	}
}
