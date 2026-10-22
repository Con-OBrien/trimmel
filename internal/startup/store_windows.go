//go:build windows

package startup

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	storeSource = "Microsoft Store app"
	storeRoot   = `Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\SystemAppData`
)

// Packaged (Store) apps declare startup tasks. Windows keeps each task's state
// in a DWORD named State under the package's SystemAppData key, which is also
// what Task Manager's Startup tab reads.
const (
	storeDisabled       = 0
	storeDisabledByUser = 1
	storeEnabled        = 2
	// 3 and 4 mean a policy decides; those are left out of the list.
)

func listStoreApps() ([]Item, error) {
	root, err := registry.OpenKey(registry.CURRENT_USER, storeRoot, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	pkgs, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	var items []Item
	for _, pkg := range pkgs {
		pk, err := registry.OpenKey(registry.CURRENT_USER, storeRoot+`\`+pkg, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		tasks, _ := pk.ReadSubKeyNames(-1)
		pk.Close()

		var found []Item
		for _, task := range tasks {
			state, ok := storeTaskState(pkg, task)
			if !ok || state > storeEnabled {
				continue
			}
			found = append(found, Item{
				ID:      "store|" + pkg + `\` + task,
				Name:    storeAppName(pkg),
				Command: pkg,
				Source:  storeSource,
				Scope:   "user",
				Enabled: state == storeEnabled,
			})
		}
		if len(found) > 1 { // tell the tasks of one app apart
			for i := range found {
				_, task, _ := strings.Cut(strings.TrimPrefix(found[i].ID, "store|"), `\`)
				found[i].Name += " (" + task + ")"
			}
		}
		items = append(items, found...)
	}
	return items, nil
}

// storeTaskState reads State from a package task key. Keys without State
// (HAM, PSR, Schemas and so on) are not startup tasks.
func storeTaskState(pkg, task string) (uint32, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, storeRoot+`\`+pkg+`\`+task, registry.QUERY_VALUE)
	if err != nil {
		return 0, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("State")
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

// setStoreEnabled switches a packaged app's startup task on or off. Only the
// State value changes, so it is reversible and shows in Task Manager.
func setStoreEnabled(path string, enabled bool) error {
	pkg, task, ok := strings.Cut(path, `\`)
	if !ok {
		return fmt.Errorf("bad Store app startup id %q", path)
	}
	state, ok := storeTaskState(pkg, task)
	if !ok {
		return fmt.Errorf("that Store app no longer has a startup task")
	}
	if state > storeEnabled {
		return fmt.Errorf("your organisation manages this startup task, so it can't be changed here")
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, storeRoot+`\`+pkg+`\`+task, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	want := uint32(storeDisabledByUser)
	if enabled {
		want = storeEnabled
	}
	return k.SetDWordValue("State", want)
}
