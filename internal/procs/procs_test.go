package procs

import (
	"os"
	"strings"
	"testing"
)

func TestKillRefusesSelfAndSystem(t *testing.T) {
	m := NewMonitor()
	if err := m.Kill(int32(os.Getpid()), "anything"); err == nil {
		t.Error("must refuse to end itself")
	}
	if err := m.Kill(4, "System"); err == nil {
		t.Error("must refuse PID 4")
	}
}

func TestKillRefusesRenamedPID(t *testing.T) {
	m := NewMonitor()
	// The test process exists but is not called this, as if its PID had been reused.
	err := m.Kill(int32(os.Getppid()), "definitely-not-this-name.exe")
	if err == nil || !strings.Contains(err.Error(), "nothing was ended") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "left alone") {
		t.Errorf("unexpected result: %v", err)
	}
}
