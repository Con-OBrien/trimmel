// Package alerts sends the two quiet Windows notifications that come with
// Trimmel Plus: a low-disk-space alert and a weekly summary. A daily scheduled
// task runs "trimmel.exe --check"; that run looks at the drives, sends at most
// the notifications that are due, and exits. Nothing runs in between.
//
// Alerts never repeat more than once a week per drive, and an alert never
// changes or deletes anything: its button only opens Trimmel.
package alerts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trimmel/internal/appdir"
	"trimmel/internal/license"
)

const (
	DefaultThreshold = 10 // percent free, below which a drive raises an alert
	Repeat           = 7 * 24 * time.Hour
)

// Settings is what is kept in alerts.json.
type Settings struct {
	LowDisk    bool                 `json:"lowDisk"`
	Threshold  int                  `json:"threshold"` // percent free; 0 means DefaultThreshold
	Report     bool                 `json:"report"`
	LastReport time.Time            `json:"lastReport,omitzero"`
	LastAlert  map[string]time.Time `json:"lastAlert,omitempty"` // by drive, e.g. "C:"
}

// View is Settings shaped for the UI.
type View struct {
	LowDisk   bool `json:"lowDisk"`
	Threshold int  `json:"threshold"`
	Report    bool `json:"report"`
}

// Disk is the part of a drive reading the checks need.
type Disk struct {
	Mount string
	Total uint64
	Free  uint64
}

// Notifier shows one notification. The button label and action are the
// caller's business; the text is ours.
type Notifier func(title, body string, withCleanButton bool) error

// Store reads and writes alerts.json.
type Store struct {
	mu   sync.Mutex
	path string
}

func Path() string { return filepath.Join(appdir.Dir(), "alerts.json") }

// Open uses Path() if path is empty.
func Open(path string) *Store {
	if path == "" {
		path = Path()
	}
	return &Store{path: path}
}

func (s *Store) load() Settings {
	var st Settings
	if raw, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	if st.Threshold < 1 || st.Threshold > 50 {
		st.Threshold = DefaultThreshold
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

// Load returns the saved settings, or defaults.
func (s *Store) Load() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Configure saves the switches and threshold. Turning the report on starts its
// week from now, so it does not fire the moment the box is ticked.
func (s *Store) Configure(lowDisk bool, threshold int, report bool, now time.Time) (Settings, error) {
	if threshold < 1 || threshold > 50 {
		return Settings{}, fmt.Errorf("choose a percentage between 1 and 50")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.load()
	if report && !st.Report {
		st.LastReport = now
	}
	st.LowDisk, st.Threshold, st.Report = lowDisk, threshold, report
	return st, s.save(st)
}

// ViewOf converts settings for the UI.
func ViewOf(st Settings) View {
	return View{LowDisk: st.LowDisk, Threshold: st.Threshold, Report: st.Report}
}

// Wanted says whether the daily check task should exist.
func (st Settings) Wanted() bool { return st.LowDisk || st.Report }

func gb(n uint64) string { return fmt.Sprintf("%.0f GB", float64(n)/(1<<30)) }

// Check sends whatever notifications are due and records them. It does nothing
// for the Free plan. lastFreed is what the last scheduled cleanup freed (0 if
// none), used in the weekly summary. It returns how many notifications went out.
func (s *Store) Check(tier license.Tier, now time.Time, disks []Disk, lastFreed uint64, notify Notifier) int {
	if tier != license.Plus && tier != license.Trial {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.load()
	sent := 0

	if st.LowDisk {
		if st.LastAlert == nil {
			st.LastAlert = map[string]time.Time{}
		}
		for _, d := range disks {
			if d.Total == 0 {
				continue
			}
			pct := int(d.Free * 100 / d.Total)
			if pct >= st.Threshold {
				delete(st.LastAlert, d.Mount) // recovered: a later dip may alert again
				continue
			}
			if last, ok := st.LastAlert[d.Mount]; ok && now.Sub(last) < Repeat {
				continue
			}
			body := fmt.Sprintf("%s has %d%% free (%s of %s). Open Trimmel to see what can be cleared.", d.Mount, pct, gb(d.Free), gb(d.Total))
			if notify("Low disk space", body, true) == nil {
				st.LastAlert[d.Mount] = now
				sent++
			}
		}
	}

	if st.Report && now.Sub(st.LastReport) >= Repeat {
		body := ""
		for _, d := range disks {
			if d.Total > 0 {
				body += fmt.Sprintf("%s has %s free of %s. ", d.Mount, gb(d.Free), gb(d.Total))
			}
		}
		if lastFreed > 0 {
			body += fmt.Sprintf("Scheduled cleanup last freed %s.", gb(lastFreed))
		}
		if notify("Your weekly Trimmel summary", body, false) == nil {
			st.LastReport = now
			sent++
		}
	}

	_ = s.save(st)
	return sent
}
