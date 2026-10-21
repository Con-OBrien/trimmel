//go:build !windows

package oldver

import "errors"

func Scan() ([]Old, error)   { return nil, errors.New("only supported on Windows") }
func Uninstall(string) error { return errors.New("only supported on Windows") }
