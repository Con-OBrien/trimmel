package schedule

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// TaskName is the Task Scheduler entry Trimmel owns.
const TaskName = `Trimmel\Scheduled cleanup`

// Install creates (or replaces) the task that runs exe --scheduled. It uses
// the built-in schtasks tool; the task runs as the current user.
func Install(exe, frequency string) error {
	args := []string{"/Create", "/F", "/TN", TaskName, "/TR", `"` + exe + `" --scheduled`, "/RL", "HIGHEST", "/ST", "12:00"}
	if frequency == Monthly {
		args = append(args, "/SC", "MONTHLY", "/D", "1")
	} else {
		args = append(args, "/SC", "WEEKLY", "/D", "SUN")
	}
	return schtasks(args...)
}

// CheckTaskName is the daily task behind the low-disk alert and weekly summary.
const CheckTaskName = `Trimmel\Alerts check`

// InstallCheck creates (or replaces) the daily task that runs exe --check.
func InstallCheck(exe string) error {
	return schtasks("/Create", "/F", "/TN", CheckTaskName, "/TR", `"`+exe+`" --check`, "/RL", "HIGHEST", "/SC", "DAILY", "/ST", "12:00")
}

// RemoveCheck deletes the daily task. A task that is already gone is not an error.
func RemoveCheck() error { return deleteTask(CheckTaskName) }

// Remove deletes the task. A task that is already gone is not an error.
func Remove() error { return deleteTask(TaskName) }

func deleteTask(name string) error {
	err := schtasks("/Delete", "/F", "/TN", name)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "cannot find") {
		return nil
	}
	return err
}

func schtasks(args ...string) error {
	cmd := exec.Command("schtasks", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("couldn't change the scheduled task: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
