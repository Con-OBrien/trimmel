//go:build windows

package cleanup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanListsEveryTargetWithoutAdmin(t *testing.T) {
	reports := Scan(false)
	if len(reports) != len(targets()) {
		t.Fatalf("got %d reports for %d targets", len(reports), len(targets()))
	}
	for i, r := range reports {
		if i > 0 && r.Bytes > reports[i-1].Bytes {
			t.Error("reports must be sorted largest first")
		}
		if r.RequiresAdmin && (r.Available || r.Error == "") {
			t.Errorf("%s needs admin and must say so: %+v", r.ID, r)
		}
	}
}

func TestCleanWithoutAdminRefusesAdminTargetsAndDeletesNothing(t *testing.T) {
	res := Clean([]string{"windows-temp", "wu-cache", "error-reports", "no-such-target"}, false)
	if len(res) != 3 {
		t.Fatalf("unknown ids are ignored, got %d results", len(res))
	}
	for _, r := range res {
		if r.Error == "" || r.Deleted != 0 || r.FreedBytes != 0 {
			t.Errorf("%s must be refused: %+v", r.ID, r)
		}
	}
}

func TestCleanNoIDsDoesNothing(t *testing.T) {
	if res := Clean(nil, true); len(res) != 0 {
		t.Errorf("nothing selected, nothing cleaned: %+v", res)
	}
}

func TestTargetsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tg := range targets() {
		if tg.ID == "" || tg.Name == "" || tg.Description == "" || seen[tg.ID] {
			t.Errorf("bad or duplicate target %q", tg.ID)
		}
		seen[tg.ID] = true
		if tg.ScanFn == nil && tg.Roots == nil {
			t.Errorf("%s has nothing to scan", tg.ID)
		}
		if tg.Roots != nil {
			for _, r := range tg.Roots() {
				if unsafeRoot(r) {
					t.Errorf("%s: root %s is unsafe", tg.ID, r)
				}
			}
		}
	}
}

func TestGlobAllIgnoresBadPatterns(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a1", "a2", "b1"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := globAll(filepath.Join(dir, "a*"), "[", filepath.Join(dir, "none*"))
	if len(got) != 2 {
		t.Errorf("want the two a-files, got %v", got)
	}
}

func TestRecycleBinQueryAndRebootFlag(t *testing.T) {
	if _, _, err := queryRecycleBin(); err != nil {
		t.Logf("recycle bin query unavailable here: %v", err)
	}
	_ = RebootPending()
}
