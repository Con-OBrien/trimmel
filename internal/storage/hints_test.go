package storage

import "testing"

func TestDescribe(t *testing.T) {
	cases := map[string]bool{ // path -> expects a hint
		`C:\Windows`:                    true,
		`C:\Program Files (x86)`:        true,
		`C:\$Recycle.Bin`:               true,
		`C:\hiberfil.sys`:               true,
		`C:\Users\x\AppData`:            true,
		`C:\Users\x\AppData\Local`:      true,
		`C:\Users\x\Downloads`:          true,
		`C:\Users\x\proj\node_modules`:  true,
		`C:\Users\x\Local`:              false, // "Local" only means something inside AppData
		`C:\Users\x\Some Random Folder`: false,
	}
	for path, want := range cases {
		if got := Describe(path) != ""; got != want {
			t.Errorf("%s: hint=%v want %v", path, got, want)
		}
	}
}
