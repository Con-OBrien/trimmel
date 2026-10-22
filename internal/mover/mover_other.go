//go:build !windows

package mover

import "errors"

var errUnsupported = errors.New("moving installs is only supported on Windows")

func Scan() ([]Install, error) { return nil, errUnsupported }

func Move(src, destRoot string, emit func(Progress)) (Result, error) {
	return Result{}, errUnsupported
}

func MoveBack(src string, emit func(Progress)) (Result, error) {
	return Result{}, errUnsupported
}

func Leftovers() ([]Leftover, error)   { return nil, errUnsupported }
func DeleteLeftover(path string) error { return errUnsupported }

func CheckDest(root string) error { return errUnsupported }
