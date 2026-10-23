package schedule

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trimmel/internal/cleanup"
	"trimmel/internal/license"
)

func fakeClean(calls *[]string) Cleaner {
	return func(ids []string, _ bool) []cleanup.CleanResult {
		*calls = append(*calls, ids...)
		return []cleanup.CleanResult{{ID: ids[0], FreedBytes: 100}, {ID: ids[1], FreedBytes: 50, Error: "locked"}}
	}
}

func TestDefaultsAndConfigure(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "s.json"))
	if st := s.Load(); st.Enabled || st.Frequency != Weekly {
		t.Fatalf("defaults wrong: %+v", st)
	}
	if _, err := s.Configure(true, "daily"); err == nil {
		t.Fatal("bad frequency accepted")
	}
	st, err := s.Configure(true, Monthly)
	if err != nil || !st.Enabled || s.Load().Frequency != Monthly {
		t.Fatalf("configure failed: %+v %v", st, err)
	}
}

func TestRunCleansForPlusAndTrial(t *testing.T) {
	for _, tier := range []license.Tier{license.Plus, license.Trial} {
		s := Open(filepath.Join(t.TempDir(), "s.json"))
		if _, err := s.Configure(true, Weekly); err != nil {
			t.Fatal(err)
		}
		var calls []string
		now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
		st := s.Run(tier, true, now, fakeClean(&calls))
		if len(calls) != len(SafeTargets) || st.LastFreed != 150 || !st.LastRun.Equal(now) {
			t.Errorf("%s: %+v calls=%v", tier, st, calls)
		}
		if !strings.Contains(st.LastNote, "skipped") {
			t.Errorf("%s: failure not noted: %q", tier, st.LastNote)
		}
		if s.Load().LastFreed != 150 {
			t.Errorf("%s: not saved", tier)
		}
	}
}

func TestRunDoesNothingWhenOffOrFree(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "s.json"))
	var calls []string
	s.Run(license.Plus, true, time.Now(), fakeClean(&calls)) // switched off
	if _, err := s.Configure(true, Weekly); err != nil {
		t.Fatal(err)
	}
	st := s.Run(license.Free, true, time.Now(), fakeClean(&calls))
	if len(calls) != 0 {
		t.Fatalf("cleaned when it should not: %v", calls)
	}
	if !strings.Contains(st.LastNote, "Plus") {
		t.Errorf("free note: %q", st.LastNote)
	}
}

func TestSafeTargetsExcludeRecycleBin(t *testing.T) {
	for _, id := range SafeTargets {
		if id == "recycle-bin" || id == "wu-cache" {
			t.Errorf("%s must not run unattended", id)
		}
	}
}

func TestViewOf(t *testing.T) {
	v := ViewOf(Settings{Enabled: true, Frequency: Weekly})
	if v.LastRun != "" || !v.Enabled {
		t.Errorf("%+v", v)
	}
	v = ViewOf(Settings{LastRun: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)})
	if v.LastRun == "" {
		t.Error("last run missing")
	}
}
