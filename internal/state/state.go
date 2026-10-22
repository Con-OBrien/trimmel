// Package state keeps small per-user preferences and history on disk: when the
// user last ran Cleanup, how often to nudge them, and whether the welcome
// guide has been shown. Everything stays on this PC.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"pcoptimizer/internal/appdir"
)

const DefaultReminderDays = 14

type Data struct {
	LastTidy     time.Time `json:"lastTidy"`     // zero = never
	ReminderDays int       `json:"reminderDays"` // 0 = no reminders
	Welcomed     bool      `json:"welcomed"`
}

type Store struct {
	mu   sync.Mutex
	path string
	d    Data
}

func Path() string {
	return filepath.Join(appdir.Dir(), "state.json")
}

// Open loads the file at path (Path() if empty). A missing or unreadable file
// gives defaults, so a bad file never blocks the app.
func Open(path string) *Store {
	if path == "" {
		path = Path()
	}
	s := &Store{path: path, d: Data{ReminderDays: DefaultReminderDays}}
	if raw, err := os.ReadFile(path); err == nil {
		var d Data
		if json.Unmarshal(raw, &d) == nil {
			s.d = d
		}
	}
	return s
}

func (s *Store) Get() Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.d
}

func (s *Store) update(fn func(*Data)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.d)
	raw, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) MarkTidy(t time.Time) error { return s.update(func(d *Data) { d.LastTidy = t }) }
func (s *Store) MarkWelcomed() error        { return s.update(func(d *Data) { d.Welcomed = true }) }

func (s *Store) SetReminderDays(n int) error {
	if n < 0 || n > 365 {
		return errors.New("reminder must be between 0 and 365 days")
	}
	return s.update(func(d *Data) { d.ReminderDays = n })
}

// DaysSinceTidy is -1 if Cleanup has never been run.
func (d Data) DaysSinceTidy(now time.Time) int {
	if d.LastTidy.IsZero() {
		return -1
	}
	return int(now.Sub(d.LastTidy).Hours() / 24)
}
