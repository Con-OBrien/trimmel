package state

import (
	"path/filepath"
	"testing"
)

func TestPathAndOpenDefault(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if filepath.Base(Path()) != "state.json" {
		t.Errorf("unexpected path %s", Path())
	}
	s := Open("")
	if s.Get().ReminderDays != DefaultReminderDays {
		t.Errorf("an empty store uses defaults: %+v", s.Get())
	}
}
