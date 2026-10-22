package startup

import "testing"

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
