package alerts

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trimmel/internal/license"
)

const g = 1 << 30

var t0 = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

type sink struct {
	titles []string
	bodies []string
	fail   bool
}

func (k *sink) notify(title, body string, _ bool) error {
	if k.fail {
		return errors.New("no toast")
	}
	k.titles = append(k.titles, title)
	k.bodies = append(k.bodies, body)
	return nil
}

func store(t *testing.T) *Store { return Open(filepath.Join(t.TempDir(), "a.json")) }

func TestDefaultsAndValidation(t *testing.T) {
	s := store(t)
	if st := s.Load(); st.Threshold != DefaultThreshold || st.LowDisk || st.Report {
		t.Fatalf("defaults: %+v", st)
	}
	if _, err := s.Configure(true, 0, false, t0); err == nil {
		t.Fatal("0% accepted")
	}
	if _, err := s.Configure(true, 99, false, t0); err == nil {
		t.Fatal("99% accepted")
	}
	st, err := s.Configure(true, 15, true, t0)
	if err != nil || !st.Wanted() || st.LastReport != t0 {
		t.Fatalf("%+v %v", st, err)
	}
	if (Settings{}).Wanted() {
		t.Fatal("empty settings want the task")
	}
}

func TestLowDiskAlertsOncePerWeek(t *testing.T) {
	s := store(t)
	s.Configure(true, 10, false, t0)
	k := &sink{}
	low := []Disk{{"C:", 100 * g, 5 * g}, {"D:", 100 * g, 60 * g}}
	if n := s.Check(license.Plus, t0, low, 0, k.notify); n != 1 || !strings.Contains(k.bodies[0], "C: has 5%") {
		t.Fatalf("first check: %d %v", n, k.bodies)
	}
	if n := s.Check(license.Plus, t0.Add(24*time.Hour), low, 0, k.notify); n != 0 {
		t.Fatalf("repeated within a week: %d", n)
	}
	if n := s.Check(license.Plus, t0.Add(8*24*time.Hour), low, 0, k.notify); n != 1 {
		t.Fatalf("not repeated after a week: %d", n)
	}
}

func TestRecoveredDriveAlertsAgainOnNextDip(t *testing.T) {
	s := store(t)
	s.Configure(true, 10, false, t0)
	k := &sink{}
	s.Check(license.Plus, t0, []Disk{{"C:", 100 * g, 5 * g}}, 0, k.notify)
	s.Check(license.Plus, t0.Add(time.Hour), []Disk{{"C:", 100 * g, 50 * g}}, 0, k.notify)
	if n := s.Check(license.Plus, t0.Add(2*time.Hour), []Disk{{"C:", 100 * g, 4 * g}}, 0, k.notify); n != 1 {
		t.Fatalf("dip after recovery did not alert: %d", n)
	}
}

func TestFailedToastIsRetriedNextTime(t *testing.T) {
	s := store(t)
	s.Configure(true, 10, false, t0)
	k := &sink{fail: true}
	low := []Disk{{"C:", 100 * g, 5 * g}}
	if n := s.Check(license.Plus, t0, low, 0, k.notify); n != 0 {
		t.Fatal("counted a failed toast")
	}
	k.fail = false
	if n := s.Check(license.Plus, t0.Add(time.Hour), low, 0, k.notify); n != 1 {
		t.Fatalf("not retried: %d", n)
	}
}

func TestWeeklyReport(t *testing.T) {
	s := store(t)
	s.Configure(false, 10, true, t0)
	k := &sink{}
	disks := []Disk{{"C:", 200 * g, 80 * g}}
	if n := s.Check(license.Plus, t0.Add(24*time.Hour), disks, 0, k.notify); n != 0 {
		t.Fatal("report sent before a week")
	}
	if n := s.Check(license.Trial, t0.Add(7*24*time.Hour), disks, 3*g, k.notify); n != 1 {
		t.Fatalf("report not sent: %d", n)
	}
	if !strings.Contains(k.bodies[0], "C: has 80 GB free of 200 GB") || !strings.Contains(k.bodies[0], "freed 3 GB") {
		t.Errorf("body: %q", k.bodies[0])
	}
	if n := s.Check(license.Plus, t0.Add(8*24*time.Hour), disks, 0, k.notify); n != 0 {
		t.Fatal("report repeated within a week")
	}
}

func TestFreePlanSendsNothing(t *testing.T) {
	s := store(t)
	s.Configure(true, 10, true, t0.Add(-30*24*time.Hour))
	k := &sink{}
	if n := s.Check(license.Free, t0, []Disk{{"C:", 100 * g, 1 * g}}, 0, k.notify); n != 0 || len(k.titles) != 0 {
		t.Fatalf("free plan notified: %v", k.titles)
	}
}

func TestViewOf(t *testing.T) {
	if v := ViewOf(Settings{LowDisk: true, Threshold: 12, Report: true}); !v.LowDisk || v.Threshold != 12 || !v.Report {
		t.Errorf("%+v", v)
	}
}
