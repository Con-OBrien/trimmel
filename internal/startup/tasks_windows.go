//go:build windows

package startup

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Task Scheduler trigger types that run something at sign-in or boot.
const (
	triggerBoot  = 8
	triggerLogon = 9
)

const taskSource = "Scheduled task"

// Per-user tasks such as OneDrive's are named "<task>-S-1-5-21-...".
var reSID = regexp.MustCompile(`-S-1-5-21(-\d+)+$`)

// withScheduler runs fn with a connected Task Scheduler service on a locked
// OS thread (COM objects are apartment-bound).
func withScheduler(fn func(svc *ole.IDispatch) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		var oe *ole.OleError
		if !errors.As(err, &oe) || oe.Code() != 1 {
			return err
		}
	}
	defer ole.CoUninitialize()

	unk, err := oleutil.CreateObject("Schedule.Service")
	if err != nil {
		return err
	}
	defer unk.Release()
	svc, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer svc.Release()
	if _, err := oleutil.CallMethod(svc, "Connect"); err != nil {
		return err
	}
	return fn(svc)
}

// listTasks returns enabled and disabled third-party tasks that start at
// sign-in or boot. Windows' own tasks (the \Microsoft folder) are left out:
// there are hundreds and none should be switched off from here.
func listTasks() ([]Item, error) {
	var items []Item
	err := withScheduler(func(svc *ole.IDispatch) error {
		root, err := dispatchResult(oleutil.CallMethod(svc, "GetFolder", `\`))
		if err != nil {
			return err
		}
		defer root.Release()
		walkFolder(root, &items)
		return nil
	})
	return items, err
}

func walkFolder(folder *ole.IDispatch, out *[]Item) {
	path := propString(folder, "Path")
	if strings.HasPrefix(strings.ToLower(path), `\microsoft`) {
		return
	}
	if tasks, err := dispatchResult(oleutil.CallMethod(folder, "GetTasks", 1)); err == nil {
		eachItem(tasks, func(t *ole.IDispatch) {
			if it, ok := taskItem(t); ok {
				*out = append(*out, it)
			}
		})
		tasks.Release()
	}
	if subs, err := dispatchResult(oleutil.CallMethod(folder, "GetFolders", 0)); err == nil {
		eachItem(subs, func(f *ole.IDispatch) { walkFolder(f, out) })
		subs.Release()
	}
}

func taskItem(t *ole.IDispatch) (Item, bool) {
	def, err := dispatchResult(oleutil.GetProperty(t, "Definition"))
	if err != nil {
		return Item{}, false
	}
	defer def.Release()

	starts := false
	if trig, err := dispatchResult(oleutil.GetProperty(def, "Triggers")); err == nil {
		eachItem(trig, func(tr *ole.IDispatch) {
			if ty := propInt(tr, "Type"); ty == triggerBoot || ty == triggerLogon {
				starts = true
			}
		})
		trig.Release()
	}
	if !starts {
		return Item{}, false
	}

	cmd := ""
	if acts, err := dispatchResult(oleutil.GetProperty(def, "Actions")); err == nil {
		eachItem(acts, func(a *ole.IDispatch) {
			if cmd == "" {
				cmd = strings.TrimSpace(propString(a, "Path") + " " + propString(a, "Arguments"))
			}
		})
		acts.Release()
	}

	// Windows' own helpers (System32 and friends) are not the user's to manage.
	if root := os.Getenv("SystemRoot"); root != "" && strings.HasPrefix(strings.ToLower(strings.Trim(cmd, `"`)), strings.ToLower(root)+`\`) {
		return Item{}, false
	}

	name := reSID.ReplaceAllString(propString(t, "Name"), "")
	enabled, _ := oleutil.GetProperty(t, "Enabled")
	on := enabled != nil && enabled.Value() == true
	return Item{
		ID:            "task|" + propString(t, "Path"),
		Name:          name,
		Command:       cmd,
		Source:        taskSource,
		Scope:         "machine",
		Enabled:       on,
		RequiresAdmin: true,
	}, true
}

// setTaskEnabled switches a scheduled task on or off. The task is only
// changed, never deleted, so this is reversible.
func setTaskEnabled(path string, enabled bool) error {
	return withScheduler(func(svc *ole.IDispatch) error {
		root, err := dispatchResult(oleutil.CallMethod(svc, "GetFolder", `\`))
		if err != nil {
			return err
		}
		defer root.Release()
		task, err := dispatchResult(oleutil.CallMethod(root, "GetTask", path))
		if err != nil {
			return fmt.Errorf("that scheduled task no longer exists: %w", err)
		}
		defer task.Release()
		if _, err := oleutil.PutProperty(task, "Enabled", enabled); err != nil {
			return fmt.Errorf("couldn't change the scheduled task (it may need admin rights): %w", err)
		}
		return nil
	})
}

// ---- small IDispatch helpers ----

func dispatchResult(v *ole.VARIANT, err error) (*ole.IDispatch, error) {
	if err != nil {
		return nil, err
	}
	return v.ToIDispatch(), nil
}

// eachItem walks a Task Scheduler collection. Its Item() index is 1-based.
func eachItem(coll *ole.IDispatch, fn func(*ole.IDispatch)) {
	n := propInt(coll, "Count")
	for i := 1; i <= n; i++ {
		item, err := dispatchResult(oleutil.GetProperty(coll, "Item", i))
		if err != nil {
			continue
		}
		fn(item)
		item.Release()
	}
}

func propString(d *ole.IDispatch, name string) string {
	v, err := oleutil.GetProperty(d, name)
	if err != nil {
		return ""
	}
	defer v.Clear()
	s, _ := v.Value().(string)
	return strings.TrimSpace(s)
}

func propInt(d *ole.IDispatch, name string) int {
	v, err := oleutil.GetProperty(d, name)
	if err != nil {
		return 0
	}
	defer v.Clear()
	switch n := v.Value().(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int16:
		return int(n)
	case uint32:
		return int(n)
	case int:
		return n
	}
	return 0
}
