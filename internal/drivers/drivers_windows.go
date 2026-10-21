//go:build windows

package drivers

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/yusufpapurcu/wmi"
)

// ---- Installed drivers (WMI) ----

type win32PnPSignedDriver struct {
	DeviceName         *string
	DeviceClass        *string
	DriverProviderName *string
	DriverVersion      *string
	DriverDate         *string // CIM_DATETIME, e.g. 20230115000000.******+000
	IsSigned           *bool
	InfName            *string
}

func ListInstalled() ([]Driver, error) {
	var rows []win32PnPSignedDriver
	q := "SELECT DeviceName, DeviceClass, DriverProviderName, DriverVersion, DriverDate, IsSigned, InfName FROM Win32_PnPSignedDriver"
	if err := wmi.Query(q, &rows); err != nil {
		return nil, fmt.Errorf("querying drivers: %w", err)
	}

	out := make([]Driver, 0, len(rows))
	for _, r := range rows {
		if str(r.DeviceName) == "" || str(r.DriverVersion) == "" {
			continue
		}
		out = append(out, Driver{
			Device:   str(r.DeviceName),
			Class:    str(r.DeviceClass),
			Provider: str(r.DriverProviderName),
			Version:  str(r.DriverVersion),
			Date:     cimDate(str(r.DriverDate)),
			Signed:   r.IsSigned != nil && *r.IsSigned,
			InfName:  str(r.InfName),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Class != out[j].Class {
			return out[i].Class < out[j].Class
		}
		return out[i].Device < out[j].Device
	})
	return out, nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func cimDate(s string) string {
	if len(s) < 8 {
		return ""
	}
	t, err := time.Parse("20060102", s[:8])
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// ---- Windows Update Agent (COM) ----

const driverCriteria = "IsInstalled=0 and Type='Driver' and IsHidden=0"

// withCOM runs fn on a locked OS thread with COM initialised. WUA objects are
// apartment-bound, so a search and the install that follows must happen in
// the same call rather than passing COM objects between goroutines.
func withCOM(fn func(session *ole.IDispatch) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		var oe *ole.OleError
		if !errors.As(err, &oe) || oe.Code() != 1 { // S_FALSE: already initialised
			return fmt.Errorf("COM init: %w", err)
		}
	}
	defer ole.CoUninitialize()

	unk, err := oleutil.CreateObject("Microsoft.Update.Session")
	if err != nil {
		return fmt.Errorf("Windows Update unavailable: %w", err)
	}
	defer unk.Release()
	session, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer session.Release()
	return fn(session)
}

// search returns the Updates collection for driver updates not yet installed.
// The caller must Release it.
func search(session *ole.IDispatch) (*ole.IDispatch, error) {
	searcher, err := callDispatch(session, "CreateUpdateSearcher")
	if err != nil {
		return nil, err
	}
	defer searcher.Release()

	result, err := callDispatch(searcher, "Search", driverCriteria)
	if err != nil {
		return nil, fmt.Errorf("Windows Update search failed: %w", err)
	}
	defer result.Release()

	return getDispatch(result, "Updates")
}

func SearchUpdates() ([]Update, error) {
	var out []Update
	err := withCOM(func(session *ole.IDispatch) error {
		updates, err := search(session)
		if err != nil {
			return err
		}
		defer updates.Release()

		return eachUpdate(updates, func(item *ole.IDispatch) {
			u := Update{
				ID:           updateID(item),
				Title:        getString(item, "Title"),
				Manufacturer: getString(item, "DriverManufacturer"),
				Model:        getString(item, "DriverModel"),
				Class:        getString(item, "DriverClass"),
				NeedsEula:    !getBool(item, "EulaAccepted"),
			}
			if v, err := oleutil.GetProperty(item, "DriverVerDate"); err == nil {
				if t, ok := v.Value().(time.Time); ok {
					u.Date = t.Format("2006-01-02")
				}
				v.Clear()
			}
			out = append(out, u)
		})
	})
	return out, err
}

func InstallUpdates(ids []string) (InstallResult, error) {
	var res InstallResult
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}

	err := withCOM(func(session *ole.IDispatch) error {
		updates, err := search(session)
		if err != nil {
			return err
		}
		defer updates.Release()

		collUnk, err := oleutil.CreateObject("Microsoft.Update.UpdateColl")
		if err != nil {
			return err
		}
		defer collUnk.Release()
		coll, err := collUnk.QueryInterface(ole.IID_IDispatch)
		if err != nil {
			return err
		}
		defer coll.Release()

		var titles []string
		err = eachUpdate(updates, func(item *ole.IDispatch) {
			if !want[updateID(item)] {
				return
			}
			title := getString(item, "Title")
			// Accepting licence terms is the user's call; we don't do it
			// for them. They can install those via Windows Update.
			if !getBool(item, "EulaAccepted") {
				res.Skipped = append(res.Skipped, title)
				return
			}
			if v, err := oleutil.CallMethod(coll, "Add", item); err == nil {
				v.Clear()
				titles = append(titles, title)
			}
		})
		if err != nil {
			return err
		}
		if len(titles) == 0 {
			return nil
		}

		downloader, err := callDispatch(session, "CreateUpdateDownloader")
		if err != nil {
			return err
		}
		defer downloader.Release()
		if _, err := oleutil.PutProperty(downloader, "Updates", coll); err != nil {
			return err
		}
		if v, err := oleutil.CallMethod(downloader, "Download"); err != nil {
			return fmt.Errorf("download failed: %w", err)
		} else {
			v.Clear()
		}

		installer, err := callDispatch(session, "CreateUpdateInstaller")
		if err != nil {
			return err
		}
		defer installer.Release()
		if _, err := oleutil.PutProperty(installer, "Updates", coll); err != nil {
			return err
		}
		result, err := callDispatch(installer, "Install")
		if err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		defer result.Release()

		res.RebootRequired = getBool(result, "RebootRequired")
		for i, title := range titles {
			ur, err := callDispatch(result, "GetUpdateResult", i)
			if err != nil {
				res.Failed = append(res.Failed, title)
				continue
			}
			// OperationResultCode: 2 succeeded, 3 succeeded with errors.
			code := getInt(ur, "ResultCode")
			ur.Release()
			if code == 2 || code == 3 {
				res.Installed = append(res.Installed, title)
			} else {
				res.Failed = append(res.Failed, title)
			}
		}
		return nil
	})
	return res, err
}

// ---- small IDispatch helpers ----

func eachUpdate(updates *ole.IDispatch, fn func(item *ole.IDispatch)) error {
	n := getInt(updates, "Count")
	for i := 0; i < n; i++ {
		item, err := getDispatch(updates, "Item", i)
		if err != nil {
			return err
		}
		fn(item)
		item.Release()
	}
	return nil
}

func updateID(item *ole.IDispatch) string {
	ident, err := getDispatch(item, "Identity")
	if err != nil {
		return ""
	}
	defer ident.Release()
	return getString(ident, "UpdateID")
}

func callDispatch(d *ole.IDispatch, method string, args ...interface{}) (*ole.IDispatch, error) {
	v, err := oleutil.CallMethod(d, method, args...)
	if err != nil {
		return nil, err
	}
	return v.ToIDispatch(), nil // ownership passes to the caller
}

func getDispatch(d *ole.IDispatch, prop string, args ...interface{}) (*ole.IDispatch, error) {
	v, err := oleutil.GetProperty(d, prop, args...)
	if err != nil {
		return nil, err
	}
	return v.ToIDispatch(), nil
}

func getString(d *ole.IDispatch, prop string) string {
	v, err := oleutil.GetProperty(d, prop)
	if err != nil {
		return ""
	}
	defer v.Clear()
	if s, ok := v.Value().(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func getBool(d *ole.IDispatch, prop string) bool {
	v, err := oleutil.GetProperty(d, prop)
	if err != nil {
		return false
	}
	defer v.Clear()
	b, _ := v.Value().(bool)
	return b
}

func getInt(d *ole.IDispatch, prop string) int {
	v, err := oleutil.GetProperty(d, prop)
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
