package license

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"trimmel/internal/appdir"
)

// saved is what lives in licence.json next to the other settings.
type saved struct {
	Key        string    `json:"key,omitempty"`
	TrialStart time.Time `json:"trialStart,omitzero"`
}

// Status is what the UI shows. Tier is what the app may use right now;
// Note explains it in plain English.
type Status struct {
	Tier          Tier      `json:"tier"`
	Note          string    `json:"note"`
	Email         string    `json:"email,omitempty"`
	Expires       time.Time `json:"expires,omitzero"`
	TrialDaysLeft int       `json:"trialDaysLeft"` // only while Tier is Trial
	CanTrial      bool      `json:"canTrial"`      // the trial has not been used yet
	CanActivate   bool      `json:"canActivate"`   // this build can check keys
}

// Manager holds licence state for one user. It never blocks the app: any
// problem reading or checking a key just means the Free tier.
type Manager struct {
	mu   sync.Mutex
	path string
	pub  ed25519.PublicKey
	now  func() time.Time
	s    saved
}

// Path is the licence file for the current user.
func Path() string { return filepath.Join(appdir.Dir(), "licence.json") }

// Open loads the file at path (Path() if empty). pub is the key to check keys
// against (PublicKey() if nil); now is time.Now if nil.
func Open(path string, pub ed25519.PublicKey, now func() time.Time) *Manager {
	if path == "" {
		path = Path()
	}
	if pub == nil {
		pub = PublicKey()
	}
	if now == nil {
		now = time.Now
	}
	m := &Manager{path: path, pub: pub, now: now}
	if raw, err := os.ReadFile(path); err == nil {
		var s saved
		if json.Unmarshal(raw, &s) == nil {
			m.s = s
		}
	}
	return m
}

func (m *Manager) save() error {
	raw, err := json.MarshalIndent(m.s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// major reads the leading number of a version like "0.3.0". "dev" and anything
// unreadable count as 0, so development builds accept any key.
func major(version string) int {
	n, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), ".")
	v, _ := strconv.Atoi(n)
	return v
}

// Status reports the tier for the given app version.
func (m *Manager) Status(appVersion string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status(appVersion)
}

func (m *Manager) status(appVersion string) Status {
	now := m.now()
	st := Status{Tier: Free, CanTrial: m.s.TrialStart.IsZero(), CanActivate: len(m.pub) == ed25519.PublicKeySize}

	if m.s.Key != "" {
		c, err := Verify(m.pub, m.s.Key, now, major(appVersion))
		switch {
		case err == nil:
			st.Tier, st.Email, st.Expires = Plus, c.Email, c.Expires
			st.Note = "Trimmel Plus is active."
			return st
		case errors.Is(err, ErrExpired):
			st.Note = "Your Plus key has expired. Everything you already changed still works and can be undone."
		case errors.Is(err, ErrVersion):
			st.Note = "Your Plus key covers an older version of Trimmel. The free features are unchanged."
		default:
			st.Note = "The saved Plus key could not be checked, so Trimmel is running in Free mode."
		}
	}

	if !m.s.TrialStart.IsZero() {
		left := TrialDays - int(now.Sub(m.s.TrialStart).Hours()/24)
		if left > 0 {
			st.Tier, st.TrialDaysLeft = Trial, left
			st.Note = "Free trial of Plus: " + plural(left, "day") + " left."
			return st
		}
		if st.Note == "" {
			st.Note = "Your Plus trial has ended. Everything you already changed still works and can be undone."
		}
	}
	if st.Note == "" {
		st.Note = "You are using the free version. It has no time limit."
	}
	return st
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// StartTrial begins the trial once per PC. Calling it again is an error, so
// the date cannot be reset from the UI.
func (m *Manager) StartTrial() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.s.TrialStart.IsZero() {
		return errors.New("the free trial has already been used on this PC")
	}
	m.s.TrialStart = m.now()
	return m.save()
}

// Activate checks a pasted key and, if it is good, saves it.
func (m *Manager) Activate(key, appVersion string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key = strings.Join(strings.Fields(key), "")
	if _, err := Verify(m.pub, key, m.now(), major(appVersion)); err != nil {
		return m.status(appVersion), err
	}
	m.s.Key = key
	if err := m.save(); err != nil {
		return m.status(appVersion), err
	}
	return m.status(appVersion), nil
}

// Deactivate forgets the saved key (the trial date is kept).
func (m *Manager) Deactivate(appVersion string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.s.Key = ""
	err := m.save()
	return m.status(appVersion), err
}
