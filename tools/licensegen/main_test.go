package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trimmel/internal/license"
)

var now = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func do(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var b bytes.Buffer
	err := run(args, &b, now)
	return b.String(), err
}

func TestKeygenThenSignThenVerify(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "priv.key")
	out, err := do(t, "keygen", "-out", keyFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	pub, err := base64.StdEncoding.DecodeString(lines[len(lines)-1])
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatalf("bad public key line %q", lines[len(lines)-1])
	}

	if _, err := do(t, "keygen", "-out", keyFile); err == nil {
		t.Fatal("keygen must not overwrite an existing key")
	}

	for _, tc := range []struct {
		args    []string
		expires bool
	}{
		{[]string{"-years", "1"}, true},
		{nil, false},
	} {
		args := append([]string{"sign", "-key", keyFile, "-email", "a@b.c", "-id", "O1"}, tc.args...)
		out, err = do(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		c, err := license.Verify(pub, out, now, 0)
		if err != nil {
			t.Fatalf("key from sign does not verify: %v", err)
		}
		if c.Email != "a@b.c" || c.ID != "O1" || c.Devices != license.MaxDevices || c.Expires.IsZero() == tc.expires {
			t.Fatalf("claims: %+v", c)
		}
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.key")
	if _, err := do(t, "keygen", "-out", bad); err != nil {
		t.Fatal(err)
	}
	notKey := filepath.Join(dir, "notkey")
	if err := writeText(notKey, "hello"); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"no command":      nil,
		"unknown command": {"frobnicate"},
		"keygen no out":   {"keygen"},
		"keygen bad flag": {"keygen", "-nope"},
		"keygen bad path": {"keygen", "-out", filepath.Join(dir, "no", "such", "dir", "k")},
		"sign no args":    {"sign"},
		"sign bad flag":   {"sign", "-nope"},
		"sign negative":   {"sign", "-key", bad, "-email", "a", "-id", "b", "-years", "-1"},
		"sign no file":    {"sign", "-key", filepath.Join(dir, "missing"), "-email", "a", "-id", "b"},
		"sign not a key":  {"sign", "-key", notKey, "-email", "a", "-id", "b"},
	} {
		if _, err := do(t, args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
