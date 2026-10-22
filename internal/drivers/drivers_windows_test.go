//go:build windows

package drivers

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFriendlyWUError(t *testing.T) {
	cases := map[string]string{
		"exception 0x80072EE7":  "internet connection",
		"exception 0x8024402C":  "internet connection",
		"exception 0x80240FFF":  "organisation",
		"exception 0x80240016":  "restart",
		"exception 0x80070005x": "boom",
	}
	for in, want := range cases {
		msg := friendlyWUError("Search failed", errors.New(in)).Error()
		if want == "boom" {
			want = in // unknown codes keep the original text
		}
		if !strings.Contains(strings.ToLower(msg), strings.ToLower(want)) {
			t.Errorf("%q: got %q, want it to mention %q", in, msg, want)
		}
	}
}

func TestCimDate(t *testing.T) {
	for in, want := range map[string]string{"20230115000000.******+000": "2023-01-15", "": "", "bad": ""} {
		if got := cimDate(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func TestToDriversFiltersTrimsAndSorts(t *testing.T) {
	rows := []win32PnPSignedDriver{
		{DeviceName: ptr("  Wi-Fi Adapter "), DeviceClass: ptr("NET"), DriverProviderName: ptr(" Intel "), DriverVersion: ptr("22.1"), DriverDate: ptr("20230115000000.******+000"), IsSigned: ptr(true), InfName: ptr("oem1.inf")},
		{DeviceName: ptr("Audio"), DeviceClass: ptr("MEDIA"), DriverVersion: ptr("6.0"), IsSigned: nil},
		{DeviceName: ptr("Ethernet"), DeviceClass: ptr("NET"), DriverVersion: ptr("1.0"), IsSigned: ptr(false)},
		{DeviceName: ptr("No version"), DeviceClass: ptr("NET")},
		{DeviceName: nil, DriverVersion: ptr("1.0")},
		{DeviceName: ptr("   "), DriverVersion: ptr("1.0")},
	}
	got := toDrivers(rows)
	if len(got) != 3 {
		t.Fatalf("rows without a name or version must be dropped, got %d: %+v", len(got), got)
	}
	order := []string{"Audio", "Ethernet", "Wi-Fi Adapter"}
	for i, want := range order {
		if got[i].Device != want {
			t.Errorf("position %d: got %q, want %q (sorted by class, then device)", i, got[i].Device, want)
		}
	}
	wifi := got[2]
	if wifi.Provider != "Intel" || wifi.Date != "2023-01-15" || !wifi.Signed || wifi.InfName != "oem1.inf" {
		t.Errorf("fields not mapped or trimmed: %+v", wifi)
	}
	if got[0].Signed || got[1].Signed {
		t.Error("a missing or false IsSigned means unsigned")
	}
}

func TestToDriversEmpty(t *testing.T) {
	if got := toDrivers(nil); got == nil || len(got) != 0 {
		t.Errorf("want an empty, non-nil list for the UI, got %#v", got)
	}
}

func TestStr(t *testing.T) {
	if str(nil) != "" || str(ptr("  x ")) != "x" {
		t.Error("str should treat nil as empty and trim")
	}
}

func TestCimDateEdgeCases(t *testing.T) {
	for in, want := range map[string]string{
		"20231301000000.******+000": "",
		"2023011":                   "",
		"20240229":                  "2024-02-29",
	} {
		if got := cimDate(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestFriendlyWUErrorAllBranches(t *testing.T) {
	cases := map[string]string{
		"x 80240438":  "internet connection",
		"x 8024401c":  "internet connection",
		"x 80072ef1":  "internet connection",
		"x 8024500c":  "organisation",
		"x 80240037":  "organisation",
		"x 8024001e":  "restart",
		"plain error": "plain error",
	}
	for in, want := range cases {
		err := friendlyWUError("Install failed", errors.New(in))
		if !strings.HasPrefix(err.Error(), "Install failed") || !strings.Contains(strings.ToLower(err.Error()), want) {
			t.Errorf("%q: got %q, want it to start with the action and mention %q", in, err, want)
		}
	}
	base := errors.New("raw")
	if !errors.Is(friendlyWUError("x", base), base) {
		t.Error("an unknown error should stay unwrappable")
	}
}

func TestListInstalledReadsWMI(t *testing.T) {
	got, err := ListInstalled()
	if err != nil {
		t.Skipf("WMI not available: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("a Windows machine has installed drivers")
	}
	for _, d := range got {
		if d.Device == "" || d.Version == "" {
			t.Fatalf("rows without a name or version must be filtered: %+v", d)
		}
	}
}

func TestOnMeteredConnectionAnswers(t *testing.T) {
	done := make(chan bool, 1)
	go func() { done <- OnMeteredConnection() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the metered check must give up on its own")
	}
}
