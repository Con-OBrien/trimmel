package main

import (
	"sync"
	"time"

	"trimmel/internal/drivers"
)

// scanStaleAfter is how long a result is trusted before an automatic rescan.
const scanStaleAfter = 6 * time.Hour

// DriverScanState is the latest Windows Update driver check, kept in memory
// for the session so every tab can show it without rescanning.
type DriverScanState struct {
	Updates   []drivers.Update `json:"updates"`
	CheckedAt int64            `json:"checkedAt"` // unix ms, 0 = never checked
	Checking  bool             `json:"checking"`
	Error     string           `json:"error,omitempty"`
}

type driverScan struct {
	mu    sync.Mutex
	state DriverScanState
}

func (s *driverScan) snapshot() DriverScanState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	st.Updates = append([]drivers.Update(nil), st.Updates...)
	return st
}

// begin marks a scan as running unless one already is, or (when !force) the
// last result is still fresh.
func (s *driverScan) begin(force bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Checking {
		return false
	}
	fresh := s.state.CheckedAt != 0 && time.Since(time.UnixMilli(s.state.CheckedAt)) < scanStaleAfter
	if fresh && !force {
		return false
	}
	s.state.Checking = true
	return true
}

func (s *driverScan) finish(updates []drivers.Update, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Checking = false
	if err != nil {
		s.state.Error = err.Error() // keep the previous list and time
		return
	}
	s.state.Error = ""
	s.state.Updates = updates
	s.state.CheckedAt = time.Now().UnixMilli()
}

func (s *driverScan) remove(titles []string) {
	gone := map[string]bool{}
	for _, t := range titles {
		gone[t] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.state.Updates[:0]
	for _, u := range s.state.Updates {
		if !gone[u.Title] {
			kept = append(kept, u)
		}
	}
	s.state.Updates = kept
}
