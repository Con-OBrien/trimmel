//go:build windows

package startup

import "testing"

func TestIsEnabled(t *testing.T) {
	cases := []struct {
		name string
		v    []byte
		want bool
	}{
		{"missing value means enabled", nil, true},
		{"0x02 enabled", []byte{0x02, 0, 0, 0}, true},
		{"0x06 enabled", []byte{0x06, 0, 0, 0}, true},
		{"0x03 disabled", []byte{0x03, 0, 0, 0}, false},
		{"0x07 disabled", []byte{0x07, 0, 0, 0}, false},
	}
	for _, c := range cases {
		if got := isEnabled(c.v); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestApprovedValueRoundTrip(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		v := approvedValue(enabled)
		if len(v) != 12 {
			t.Fatalf("value must be 12 bytes, got %d", len(v))
		}
		if isEnabled(v) != enabled {
			t.Errorf("enabled=%v did not round-trip", enabled)
		}
	}
}

func TestDisplayName(t *testing.T) {
	for in, want := range map[string]string{"Steam.lnk": "Steam", "STEAM.LNK": "STEAM", "My.App": "My.App", "x": "x"} {
		if got := displayName(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestSetEnabledRejectsBadIDs(t *testing.T) {
	for _, id := range []string{"nobar", "unknown-src|x"} {
		if err := SetEnabled(id, true); err == nil {
			t.Errorf("%q should be rejected", id)
		}
	}
}
