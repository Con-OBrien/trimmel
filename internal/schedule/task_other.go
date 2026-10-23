//go:build !windows

package schedule

import "errors"

const TaskName = "Trimmel/Scheduled cleanup"

func Install(exe, frequency string) error {
	return errors.New("scheduled cleanup is only available on Windows")
}

func Remove() error { return nil }

func InstallCheck(exe string) error {
	return errors.New("alerts are only available on Windows")
}

func RemoveCheck() error { return nil }
