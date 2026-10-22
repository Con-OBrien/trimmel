//go:build windows

package drivers

import (
	"errors"
	"strings"
	"testing"
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
