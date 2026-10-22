package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultsAndRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "state.json")
	s := Open(p)
	if d := s.Get(); d.ReminderDays != DefaultReminderDays || d.Welcomed || d.DaysSinceTidy(time.Now()) != -1 {
		t.Fatalf("defaults: %+v", d)
	}
	when := time.Now().Add(-72 * time.Hour)
	if err := s.MarkTidy(when); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkWelcomed(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetReminderDays(0); err != nil {
		t.Fatal(err)
	}

	d := Open(p).Get() // fresh load from disk
	if !d.Welcomed || d.ReminderDays != 0 || d.DaysSinceTidy(time.Now()) != 3 {
		t.Fatalf("reloaded: %+v", d)
	}
}

func TestBadFileFallsBackToDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	os.WriteFile(p, []byte("{not json"), 0o600)
	if d := Open(p).Get(); d.ReminderDays != DefaultReminderDays {
		t.Fatalf("got %+v", d)
	}
}

func TestReminderRange(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "s.json"))
	if s.SetReminderDays(-1) == nil || s.SetReminderDays(366) == nil {
		t.Error("out-of-range reminder must be rejected")
	}
}
