// Package schedule runs Trimmel's quiet cleanup (a Trimmel Plus feature). A
// Windows scheduled task starts "trimmel.exe --scheduled" weekly or monthly;
// that run cleans a short list of safe targets, records what it freed and
// exits. There is no service and nothing runs between those starts.
package schedule

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trimmel/internal/appdir"
	"trimmel/internal/cleanup"
	"trimmel/internal/license"
)

const (
	Weekly  = "weekly"
	Monthly = "monthly"
)

// SafeTargets are the cleanup targets a quiet run may clear. The Recycle Bin
// and Windows Update downloads are left out on purpose: a person should choose
// to empty those, not a timer.
var SafeTargets = []string{
	"user-temp", "windows-temp", "chrome-cache", "edge-cache", "firefox-cache",
	"shader-cache", "error-reports", "crash-dumps", "delivery-optimization",
}

// Settings is what is kept in schedule.json.
type Settings struct {
	Enabled   bool      `json:"enabled"`
	Frequency string    `json:"frequency"` // Weekly or Monthly
	LastRun   time.Time `json:"lastRun,omitzero"`
	LastFreed uint64    `json:"lastFreed"`
	LastNote  string    `json:"lastNote,omitempty"`
}

// View is Settings shaped for the UI (dates as text so Wails binds them cleanly).
type View struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"`
	LastRun   string `json:"lastRun,omitempty"` // yyyy-mm-dd hh:mm
	LastFreed uint64 `json:"lastFreed"`
	LastNote  string `json:"lastNote,omitempty"`
}

// Store reads and writes schedule.json.
type Store struct {
	mu   sync.Mutex
	path string
}

func Path() string { return filepath.Join(appdir.Dir(), "schedule.json") }

// Open uses Path() if path is empty.
func Open(path string) *Store {
	if path == "" {
		path = Path()
	}
	return &Store{path: path}
}

// Load returns the saved settings; a missing or unreadable file means defaults.
func (s *Store) Load() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) load() Settings {
	st := Settings{Frequency: Weekly}
	if raw, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	if st.Frequency != Weekly && st.Frequency != Monthly {
		st.Frequency = Weekly
	}
	return st
}

func (s *Store) save(st Settings) error {
	raw, err := json.MarshalIndent(st, "", "  ")
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

// Configure saves the on/off switch and frequency, keeping the last-run record.
func (s *Store) Configure(enabled bool, frequency string) (Settings, error) {
	if frequency != Weekly && frequency != Monthly {
		return Settings{}, errors.New("choose weekly or monthly")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.load()
	st.Enabled, st.Frequency = enabled, frequency
	return st, s.save(st)
}

// ViewOf converts settings for the UI.
func ViewOf(st Settings) View {
	v := View{Enabled: st.Enabled, Frequency: st.Frequency, LastFreed: st.LastFreed, LastNote: st.LastNote}
	if !st.LastRun.IsZero() {
		v.LastRun = st.LastRun.Local().Format("2006-01-02 15:04")
	}
	return v
}

// Cleaner clears the given targets; cleanup.Clean fits it.
type Cleaner func(ids []string, isAdmin bool) []cleanup.CleanResult

// Run is the body of "trimmel.exe --scheduled". It does nothing unless the
// schedule is on and the plan is Plus or an active trial, and always records
// why in LastNote. A quiet run that fails should leave a note, not a crash
// dialog.
func (s *Store) Run(tier license.Tier, isAdmin bool, now time.Time, clean Cleaner) Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.load()
	st.LastRun = now
	st.LastFreed = 0
	switch {
	case !st.Enabled:
		st.LastNote = "Scheduled cleanup is switched off, so nothing was cleaned."
	case tier != license.Plus && tier != license.Trial:
		st.LastNote = "Scheduled cleanup is part of Trimmel Plus, so nothing was cleaned. Your files are untouched."
	default:
		var failed int
		for _, r := range clean(SafeTargets, isAdmin) {
			st.LastFreed += r.FreedBytes
			if r.Error != "" {
				failed++
			}
		}
		st.LastNote = "Cleaned safe temp files and caches."
		if failed > 0 {
			st.LastNote += " Some items could not be cleaned and were skipped."
		}
	}
	_ = s.save(st)
	return st
}
