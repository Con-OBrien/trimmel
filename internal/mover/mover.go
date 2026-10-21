// Package mover relocates large installed programs to another drive.
//
// A move copies the install folder to the other drive, verifies the copy,
// then replaces the original folder with an NTFS junction pointing at the
// copy. Registry entries, shortcuts and services keep working because the
// original path still resolves. The move is undone by "Move back".
//
// Order matters for safety: nothing is deleted until the copy is verified
// and the junction is in place, and every failure before that point rolls
// back to the untouched original.
package mover

import "errors"

type Install struct {
	Name      string `json:"name"`
	Publisher string `json:"publisher"`
	Path      string `json:"path"`
	Drive     string `json:"drive"` // e.g. "C:"
	Size      uint64 `json:"size"`
	Movable   bool   `json:"movable"`
	Reason    string `json:"reason"`  // why it can't be moved
	MovedTo   string `json:"movedTo"` // set when Path is already a junction
}

type Progress struct {
	Name    string  `json:"name"`
	Stage   string  `json:"stage"` // checking | copying | verifying | switching | cleaning | done
	Percent float64 `json:"percent"`
}

type Result struct {
	Dest    string `json:"dest"`
	Bytes   uint64 `json:"bytes"`
	Warning string `json:"warning,omitempty"`
}

var errBusy = errors.New("another move is already in progress")

// Leftover is a folder under "<drive>\Moved Programs" that no installed
// program links to: the remains of a failed or undone move.
type Leftover struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Size  uint64 `json:"size"`
}
