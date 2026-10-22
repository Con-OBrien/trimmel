//go:build windows

package mover

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Uses a SUBST drive as a stand-in for a second disk.
func TestMoveAndBack(t *testing.T) {
	base := t.TempDir()
	vdir := filepath.Join(base, "vdrive")
	os.MkdirAll(vdir, 0o755)
	letter := ""
	for _, l := range "XYZWV" {
		if out, err := exec.Command("subst", string(l)+":", vdir).CombinedOutput(); err == nil {
			letter = string(l) + ":"
			break
		} else {
			t.Log(string(out))
		}
	}
	if letter == "" {
		t.Skip("no free drive letter for subst")
	}
	defer exec.Command("subst", letter, "/D").Run()

	// The temp folder isn't a registered install; keep only the path rules.
	orig := verifyMovable
	verifyMovable = func(src string) error {
		if r := blockReason(src, "", false); r != "" {
			return errors.New(r)
		}
		return nil
	}
	defer func() { verifyMovable = orig }()

	src := filepath.Join(base, "App")
	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "b.bin"), make([]byte, 5000), 0o644)

	var stages []string
	emit := func(p Progress) { stages = append(stages, p.Stage) }

	res, err := Move(src, letter+`\`, emit)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(res, stages)
	fi, _ := os.Lstat(src)
	if !isLink(fi) {
		t.Fatal("source is not a junction")
	}
	if b, err := os.ReadFile(filepath.Join(src, "a.txt")); err != nil || string(b) != "hello" {
		t.Fatalf("read through junction: %v %q", err, b)
	}
	if _, err := os.Stat(src + oldSuffix); err == nil {
		t.Fatal("old copy left behind")
	}
	if got := linkTarget(src); got == "" {
		t.Fatal("linkTarget empty")
	} else {
		t.Log("target:", got)
	}

	if _, err := MoveBack(src, emit); err != nil {
		t.Fatal(err)
	}
	fi, _ = os.Lstat(src)
	if isLink(fi) || !fi.IsDir() {
		t.Fatal("source not restored to a real dir")
	}
	if _, err := os.Stat(filepath.Join(src, "sub", "b.bin")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(vdir, holdFolder, "App")); err == nil {
		t.Fatal("moved copy not cleaned up")
	}
}
