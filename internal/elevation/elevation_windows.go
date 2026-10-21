//go:build windows

package elevation

import "golang.org/x/sys/windows"

// IsAdmin reports whether the process token is elevated.
func IsAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
