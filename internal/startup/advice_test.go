package startup

import (
	"strings"
	"testing"
)

func TestAdvise(t *testing.T) {
	cases := []struct {
		name, cmd, impact, rec string
	}{
		{"SecurityHealth", `%windir%\system32\SecurityHealthSystray.exe`, "low", "keep"},
		{"Discord", `C:\Users\x\AppData\Local\Discord\Update.exe --processStart Discord.exe`, "high", "disable"},
		{"AdobeAAMUpdater-1.0", "x", "medium", "disable"},
		{"SomethingUnknown", `C:\foo\bar.exe`, "low", ""},
	}
	for _, c := range cases {
		a := Advise(c.name, c.cmd)
		if a.Impact != c.impact || a.Recommend != c.rec {
			t.Errorf("%s: got %+v, want impact=%s recommend=%s", c.name, a, c.impact, c.rec)
		}
	}
}

func TestRulesAreWellFormed(t *testing.T) {
	impacts := map[string]bool{"low": true, "medium": true, "high": true}
	recs := map[string]bool{"": true, "keep": true, "disable": true}
	for i, r := range rules {
		if len(r.match) == 0 {
			t.Errorf("rule %d has no patterns", i)
		}
		for _, m := range r.match {
			if m == "" || m != strings.ToLower(m) {
				t.Errorf("rule %d: pattern %q must be non-empty and lower-case", i, m)
			}
		}
		if !impacts[r.Impact] || !recs[r.Recommend] || r.Note == "" {
			t.Errorf("rule %d (%v): bad advice %+v", i, r.match, r.Advice)
		}
	}
}

func TestAdviseEveryPatternFindsAdvice(t *testing.T) {
	for i, r := range rules {
		for _, m := range r.match {
			if got := Advise(m, ""); got.Note == "" {
				t.Errorf("rule %d: pattern %q found no advice", i, m)
			}
		}
	}
}

func TestAdviseUsesExeNameNotFolders(t *testing.T) {
	if a := Advise("Helper", `C:\Program Files\Steam Tools\helper.exe`); a.Note != "" {
		t.Errorf("a folder name must not match a rule, got %+v", a)
	}
	if a := Advise("Helper", `"C:\Games\steam.exe" -silent`); a.Recommend != "disable" {
		t.Errorf("the exe name should match, got %+v", a)
	}
}

func TestExeName(t *testing.T) {
	for in, want := range map[string]string{
		`"C:\Program Files\App\App.exe" --flag`: "app.exe",
		`C:\x\y\tool.EXE /s`:                    "tool.exe",
		`tool.exe`:                              "tool.exe",
		`rundll32 shell32.dll,Run`:              "",
		``:                                      "",
	} {
		if got := exeName(in); got != want {
			t.Errorf("exeName(%q) = %q, want %q", in, got, want)
		}
	}
}
