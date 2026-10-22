//go:build windows

package oldver

import "testing"

func e(id, name, ver, pub string) entry {
	return entry{id: id, name: name, version: ver, publisher: pub, uninstall: "x"}
}

func TestFind(t *testing.T) {
	got := find([]entry{
		e("1", "IntelliJ IDEA 2025.2", "252.1", "JetBrains s.r.o."),
		e("2", "IntelliJ IDEA 2026.2.2", "262.9", "JetBrains s.r.o."),
		e("3", "WebStorm 2024.3", "243.5", "JetBrains s.r.o."), // alone
		e("4", "Python 3.11.9", "3.11.9", "PSF"),               // excluded
		e("5", "Python 3.12.1", "3.12.1", "PSF"),
		e("6", "7-Zip 23.01 (x64)", "23.01", "Igor Pavlov"),
		e("7", "7-Zip 24.08 (x86)", "24.08", "Igor Pavlov"), // different arch: not a pair
		e("8", "Foo 1.0", "1.0", "Acme"),
		e("9", "Foo 1.0", "1.0", "Acme"), // equal versions: keep both
		e("10", "Visual C++ 2015", "14.0", "Acme"),
	})
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("got %+v", got)
	}
}

func TestQuoteCommand(t *testing.T) {
	cases := map[string]string{
		`C:\Users\Example\AppData\Local\Programs\IntelliJ IDEA 2025.2\bin\uninstall.exe /S`: `"C:\Users\Example\AppData\Local\Programs\IntelliJ IDEA 2025.2\bin\uninstall.exe" /S`,
		`"C:\a b\u.exe" /x`:   `"C:\a b\u.exe" /x`,
		`MsiExec.exe /X{ABC}`: `"MsiExec.exe" /X{ABC}`,
	}
	for in, want := range cases {
		if got := quoteCommand(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestMsiUninstall(t *testing.T) {
	cases := map[string]string{
		`MsiExec.exe /I{ABC-1}`:                      `MsiExec.exe /X{ABC-1}`,
		`MsiExec.exe /X{ABC-1}`:                      `MsiExec.exe /X{ABC-1}`,
		`"C:\Windows\System32\msiexec.exe" /i {ABC}`: `"C:\Windows\System32\msiexec.exe" /X {ABC}`,
		`C:\Program Files\App\unins000.exe /SILENT`:  `C:\Program Files\App\unins000.exe /SILENT`,
	}
	for in, want := range cases {
		if got := msiUninstall(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestMeasurable(t *testing.T) {
	t.Setenv("ProgramFiles", `C:\Program Files`)
	for dir, want := range map[string]bool{`C:\`: false, `C:\Program Files`: false, `C:\Program Files\App`: true} {
		if got := measurable(dir); got != want {
			t.Errorf("%s: got %v want %v", dir, got, want)
		}
	}
}
