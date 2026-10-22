//go:build windows

package cleanup

import "testing"

func TestUnsafeRoot(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	t.Setenv("USERPROFILE", `C:\Users\x`)
	for _, p := range []string{`C:\`, `C:\Windows`, `C:\Users\x`} {
		if !unsafeRoot(p) {
			t.Errorf("%s should be unsafe", p)
		}
	}
	for _, p := range []string{`C:\Windows\Temp`, `C:\Users\x\AppData\Local\Temp`} {
		if unsafeRoot(p) {
			t.Errorf("%s should be allowed", p)
		}
	}
}

func TestKeepFile(t *testing.T) {
	for _, p := range []string{`a\~$doc.docx`, `a\x.asd`, `a\Backup of x.wbk`} {
		if !keepFile(p) {
			t.Errorf("%s should be kept", p)
		}
	}
	if keepFile(`a\old.tmp`) {
		t.Error("plain temp file should not be kept")
	}
}
