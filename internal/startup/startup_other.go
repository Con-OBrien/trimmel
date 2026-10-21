//go:build !windows

package startup

import "errors"

func List() ([]Item, error) { return nil, nil }

func SetEnabled(id string, enabled bool) error {
	return errors.New("startup management is only supported on Windows")
}
