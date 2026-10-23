package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSizeOf(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), make([]byte, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b"), make([]byte, 50), 0o600); err != nil {
		t.Fatal(err)
	}
	size, ok := SizeOf(dir, time.Minute)
	if size != 150 || !ok {
		t.Errorf("got %d ok=%v, want 150 true", size, ok)
	}
	if size, _ := SizeOf(dir, -time.Second); size > 150 {
		t.Errorf("a spent budget gives a lower bound, got %d", size)
	}
}

func TestDescribeMoreFolders(t *testing.T) {
	for _, p := range []string{
		`C:\Users\x\AppData\Local`, `C:\Users\x\AppData\Roaming`, `C:\Users\x\AppData\LocalLow`,
		`C:\Windows`, `C:\Program Files`, `C:\ProgramData`, `C:\Users`, `C:\$Recycle.Bin`, `C:\Windows.old`,
		`C:\$WINDOWS.~BT`, `C:\System Volume Information`, `C:\pagefile.sys`, `C:\hiberfil.sys`, `C:\swapfile.sys`,
		`C:\Users\x\AppData`, `C:\Temp`, `C:\Users\x\Downloads`, `C:\Users\x\Videos`, `C:\Users\x\OneDrive`,
		`D:\Steam\steamapps`, `D:\p\node_modules`, `C:\Users\x\.nuget`, `C:\Users\x\.docker`,
		`C:\Users\x\AppData\Local\Packages`,
	} {
		if Describe(p) == "" {
			t.Errorf("%s should have a hint", p)
		}
	}
	if Describe(`C:\Users\x\Documents\Random`) != "" || Describe(`C:\other\packages`) != "" {
		t.Error("unknown folders have no hint")
	}
}
