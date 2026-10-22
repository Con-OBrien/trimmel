//go:build windows

package startup

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func makeStoreTask(t *testing.T, pkg, task string, state uint32) {
	t.Helper()
	k, _, err := registry.CreateKey(registry.CURRENT_USER, storeRoot+`\`+pkg+`\`+task, registry.SET_VALUE)
	if err != nil {
		t.Skipf("cannot write the package registry area: %v", err)
	}
	defer k.Close()
	if err := k.SetDWordValue("State", state); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, storeRoot+`\`+pkg+`\`+task)
		_ = registry.DeleteKey(registry.CURRENT_USER, storeRoot+`\`+pkg)
	})
}

func storeItem(t *testing.T, id string) (Item, bool) {
	t.Helper()
	items, err := listStoreApps()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

func TestStoreTaskListedAndSwitched(t *testing.T) {
	pkg := uniqueName("Trimmel.Test") + "_abc123"
	makeStoreTask(t, pkg, "StartupTask", 2)
	id := "store|" + pkg + `\StartupTask`

	it, ok := storeItem(t, id)
	if !ok {
		t.Fatalf("%s not listed", id)
	}
	if !it.Enabled || it.Scope != "user" || it.RequiresAdmin || it.Name == "" {
		t.Errorf("unexpected item: %+v", it)
	}

	if err := SetEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if it, _ := storeItem(t, id); it.Enabled {
		t.Error("task should be disabled")
	}
	if s, _ := storeTaskState(pkg, "StartupTask"); s != storeDisabledByUser {
		t.Errorf("disabling should write state %d, got %d", storeDisabledByUser, s)
	}
	if err := SetEnabled(id, true); err != nil {
		t.Fatal(err)
	}
	if it, _ := storeItem(t, id); !it.Enabled {
		t.Error("task should be enabled again")
	}
}

func TestStoreTaskManagedByPolicyIsHiddenAndRefused(t *testing.T) {
	pkg := uniqueName("Trimmel.Policy") + "_abc123"
	makeStoreTask(t, pkg, "StartupTask", 3)
	id := "store|" + pkg + `\StartupTask`

	if _, ok := storeItem(t, id); ok {
		t.Error("a policy-managed task must not be listed")
	}
	if err := SetEnabled(id, true); err == nil {
		t.Error("a policy-managed task must not be changed")
	}
}

func TestStoreTasksOfOneAppAreToldApart(t *testing.T) {
	pkg := uniqueName("Trimmel.Multi") + "_abc123"
	makeStoreTask(t, pkg, "TaskA", 2)
	makeStoreTask(t, pkg, "TaskB", 0)
	a, okA := storeItem(t, "store|"+pkg+`\TaskA`)
	b, okB := storeItem(t, "store|"+pkg+`\TaskB`)
	if !okA || !okB || a.Name == b.Name {
		t.Errorf("two tasks of one app need different names: %q / %q", a.Name, b.Name)
	}
	if b.Enabled {
		t.Error("state 0 means disabled")
	}
}

func TestSetStoreEnabledErrors(t *testing.T) {
	if err := setStoreEnabled("no-separator", true); err == nil {
		t.Error("a path without a task should be rejected")
	}
	if err := setStoreEnabled(`Nope_abc\Missing`, true); err == nil {
		t.Error("a missing task should be rejected")
	}
}

func TestListTasksReadsTheScheduler(t *testing.T) {
	items, err := listTasks()
	if err != nil {
		t.Skipf("Task Scheduler not available: %v", err)
	}
	for _, it := range items {
		if !strings.HasPrefix(it.ID, "task|") || it.Source != taskSource || !it.RequiresAdmin {
			t.Errorf("unexpected task item: %+v", it)
		}
	}
}
