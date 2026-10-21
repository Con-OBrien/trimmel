//go:build !windows

package cleanup

import "os"

// Non-Windows builds only exist so the project compiles and tests run on any
// machine; the app itself targets Windows.
func targets() []target {
	return []target{{
		ID: "user-temp", Name: "Temp files",
		Description: "Files in the OS temp directory older than 24 hours.",
		MinAge:      24 * 60 * 60 * 1e9,
		Roots:       func() []string { return []string{os.TempDir()} },
	}}
}
