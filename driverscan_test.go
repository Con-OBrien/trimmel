package main

import (
	"errors"
	"testing"
	"time"

	"trimmel/internal/drivers"
)

func updates(titles ...string) []drivers.Update {
	var out []drivers.Update
	for _, t := range titles {
		out = append(out, drivers.Update{ID: t, Title: t})
	}
	return out
}

func TestScanBeginsOnlyOnce(t *testing.T) {
	var s driverScan
	if !s.begin(false) {
		t.Fatal("the first scan should start")
	}
	if s.begin(false) || s.begin(true) {
		t.Error("a second scan must not start while one is running, even when forced")
	}
	if !s.snapshot().Checking {
		t.Error("state should show a scan in progress")
	}
}

func TestFreshResultIsNotRescannedUnlessForced(t *testing.T) {
	var s driverScan
	s.begin(false)
	s.finish(updates("a"), nil)
	if s.begin(false) {
		t.Error("a result under six hours old should be reused")
	}
	if !s.begin(true) {
		t.Error("force should rescan regardless")
	}
}

func TestStaleResultIsRescanned(t *testing.T) {
	var s driverScan
	s.state.CheckedAt = time.Now().Add(-scanStaleAfter - time.Minute).UnixMilli()
	if !s.begin(false) {
		t.Error("a result older than the stale limit should trigger a scan")
	}
}

func TestFinishKeepsPreviousResultOnError(t *testing.T) {
	var s driverScan
	s.begin(false)
	s.finish(updates("a", "b"), nil)
	before := s.snapshot()

	s.begin(true)
	s.finish(nil, errors.New("offline"))
	after := s.snapshot()
	if after.Checking || after.Error != "offline" {
		t.Errorf("error not recorded: %+v", after)
	}
	if len(after.Updates) != 2 || after.CheckedAt != before.CheckedAt {
		t.Errorf("a failed scan must keep the previous list and time: %+v", after)
	}

	s.begin(true)
	s.finish(updates("c"), nil)
	if st := s.snapshot(); st.Error != "" || len(st.Updates) != 1 {
		t.Errorf("a good scan should clear the error: %+v", st)
	}
}

func TestRemoveDropsInstalledUpdates(t *testing.T) {
	var s driverScan
	s.begin(false)
	s.finish(updates("a", "b", "c"), nil)
	s.remove([]string{"b", "missing"})
	got := s.snapshot().Updates
	if len(got) != 2 || got[0].Title != "a" || got[1].Title != "c" {
		t.Errorf("unexpected updates after remove: %+v", got)
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	var s driverScan
	s.begin(false)
	s.finish(updates("a"), nil)
	snap := s.snapshot()
	snap.Updates[0].Title = "changed"
	if s.snapshot().Updates[0].Title != "a" {
		t.Error("changing a snapshot must not change the stored state")
	}
}
