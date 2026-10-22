//go:build !windows

package drivers

import "errors"

var errUnsupported = errors.New("driver management is only supported on Windows")

func ListInstalled() ([]Driver, error)                   { return nil, errUnsupported }
func SearchUpdates() ([]Update, error)                   { return nil, errUnsupported }
func InstallUpdates(ids []string, emit func(Progress)) (InstallResult, error) {
	return InstallResult{}, errUnsupported
}

func OnMeteredConnection() bool { return false }
