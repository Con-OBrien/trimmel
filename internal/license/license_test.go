package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func keys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestVerify(t *testing.T) {
	pub, priv := keys(t)
	otherPub, _ := keys(t)
	good := Claims{ID: "o1", Email: "a@b.c", Issued: t0, Major: 0}
	lifetime := Sign(priv, good)

	yearly := good
	yearly.Expires = t0.AddDate(1, 0, 0)

	tamper := lifetime[:len(lifetime)-2] + "AA"
	if tamper == lifetime {
		tamper = lifetime[:len(lifetime)-2] + "BB"
	}

	tests := []struct {
		name  string
		pub   ed25519.PublicKey
		key   string
		now   time.Time
		major int
		want  error
	}{
		{"valid lifetime", pub, lifetime, t0, 0, nil},
		{"pasted with spaces and newline", pub, "  " + lifetime[:20] + "\n" + lifetime[20:] + " ", t0, 0, nil},
		{"valid yearly before expiry", pub, Sign(priv, yearly), t0.AddDate(0, 11, 0), 0, nil},
		{"yearly on expiry day", pub, Sign(priv, yearly), yearly.Expires, 0, ErrExpired},
		{"older app major is covered", pub, lifetime, t0, 0, nil},
		{"newer app major is not", pub, lifetime, t0, 1, ErrVersion},
		{"wrong public key", otherPub, lifetime, t0, 0, ErrSignature},
		{"tampered signature", pub, tamper, t0, 0, ErrSignature},
		{"no prefix", pub, strings.TrimPrefix(lifetime, keyPrefix), t0, 0, ErrFormat},
		{"no dot", pub, keyPrefix + "abc", t0, 0, ErrFormat},
		{"bad base64 payload", pub, keyPrefix + "!!!.abc", t0, 0, ErrFormat},
		{"bad base64 signature", pub, keyPrefix + "abc.!!!", t0, 0, ErrFormat},
		{"empty", pub, "", t0, 0, ErrFormat},
		{"build has no public key", nil, lifetime, t0, 0, ErrNoKeyInApp},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Verify(tc.pub, tc.key, tc.now, tc.major)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestVerifyGarbagePayloadWithValidSignature(t *testing.T) {
	pub, priv := keys(t)
	// Signed, but not JSON: must be rejected, not panic.
	key := keyPrefix + b64("not json") + "." + b64(string(ed25519.Sign(priv, []byte("not json"))))
	if _, err := Verify(pub, key, t0, 0); !errors.Is(err, ErrFormat) {
		t.Fatalf("got %v, want ErrFormat", err)
	}
}

func TestPublicKeyEmptyInThisBuild(t *testing.T) {
	if k := PublicKey(); k != nil {
		t.Skip("a public key is built in")
	}
	// Without a key, activation is impossible but the app still works.
	m := Open(filepath.Join(t.TempDir(), "l.json"), nil, func() time.Time { return t0 })
	if st := m.Status("0.3.0"); st.Tier != Free || st.CanActivate {
		t.Fatalf("got %+v", st)
	}
}

func TestPublicKeyParsing(t *testing.T) {
	old := publicKeyB64
	defer func() { publicKeyB64 = old }()
	for _, bad := range []string{"", "!!!", "AAAA"} {
		publicKeyB64 = bad
		if PublicKey() != nil {
			t.Fatalf("%q should give no key", bad)
		}
	}
	pub, _ := keys(t)
	publicKeyB64 = stdB64(pub)
	if got := PublicKey(); string(got) != string(pub) {
		t.Fatal("round trip failed")
	}
}

func TestMajor(t *testing.T) {
	for v, want := range map[string]int{"0.3.0": 0, "1.2.3": 1, "v2.0.0": 2, "dev": 0, "": 0, "10.1": 10} {
		if got := major(v); got != want {
			t.Errorf("major(%q) = %d, want %d", v, got, want)
		}
	}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newManager(t *testing.T) (*Manager, *clock, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv := keys(t)
	c := &clock{t0}
	path := filepath.Join(t.TempDir(), "sub", "licence.json")
	return Open(path, pub, c.now), c, priv, path
}

func TestFreshInstallIsFree(t *testing.T) {
	m, _, _, _ := newManager(t)
	st := m.Status("0.3.0")
	if st.Tier != Free || !st.CanTrial || !st.CanActivate || st.Note == "" {
		t.Fatalf("got %+v", st)
	}
}

func TestTrialLifecycle(t *testing.T) {
	m, c, _, path := newManager(t)
	if err := m.StartTrial(); err != nil {
		t.Fatal(err)
	}
	if err := m.StartTrial(); err == nil {
		t.Fatal("second trial should be refused")
	}
	st := m.Status("0.3.0")
	if st.Tier != Trial || st.TrialDaysLeft != TrialDays || st.CanTrial {
		t.Fatalf("day 0: %+v", st)
	}

	c.t = t0.AddDate(0, 0, TrialDays-1)
	if st := m.Status("0.3.0"); st.Tier != Trial || st.TrialDaysLeft != 1 || !strings.Contains(st.Note, "1 day left") {
		t.Fatalf("last day: %+v", st)
	}

	c.t = t0.AddDate(0, 0, TrialDays)
	st = m.Status("0.3.0")
	if st.Tier != Free || st.CanTrial || !strings.Contains(st.Note, "trial has ended") {
		t.Fatalf("after trial: %+v", st)
	}

	// The trial date survives a restart, and the trial cannot be restarted.
	m2 := Open(path, m.pub, c.now)
	if m2.Status("0.3.0").Tier != Free || m2.StartTrial() == nil {
		t.Fatal("trial state not persisted")
	}
}

func TestActivateAndDeactivate(t *testing.T) {
	m, c, priv, path := newManager(t)
	key := Sign(priv, Claims{ID: "o1", Email: "a@b.c", Issued: t0, Major: 0})

	if _, err := m.Activate("garbage", "0.3.0"); !errors.Is(err, ErrFormat) {
		t.Fatalf("garbage: %v", err)
	}
	if m.Status("0.3.0").Tier != Free {
		t.Fatal("bad key must not unlock Plus")
	}

	st, err := m.Activate(key, "0.3.0")
	if err != nil || st.Tier != Plus || st.Email != "a@b.c" {
		t.Fatalf("activate: %+v, %v", st, err)
	}
	if Open(path, m.pub, c.now).Status("0.3.0").Tier != Plus {
		t.Fatal("key not persisted")
	}

	st, err = m.Deactivate("0.3.0")
	if err != nil || st.Tier != Free {
		t.Fatalf("deactivate: %+v, %v", st, err)
	}
	if Open(path, m.pub, c.now).Status("0.3.0").Tier != Free {
		t.Fatal("deactivation not persisted")
	}
}

func TestExpiredKeyFallsBackToFreeWithExplanation(t *testing.T) {
	m, c, priv, _ := newManager(t)
	key := Sign(priv, Claims{Email: "a@b.c", Issued: t0, Expires: t0.AddDate(1, 0, 0), Major: 0})
	if _, err := m.Activate(key, "0.3.0"); err != nil {
		t.Fatal(err)
	}
	c.t = t0.AddDate(1, 0, 1)
	st := m.Status("0.3.0")
	if st.Tier != Free || !strings.Contains(st.Note, "expired") || !strings.Contains(st.Note, "undone") {
		t.Fatalf("got %+v", st)
	}
}

func TestExpiredKeyDoesNotHideARunningTrial(t *testing.T) {
	m, c, priv, _ := newManager(t)
	key := Sign(priv, Claims{Issued: t0, Expires: t0.AddDate(0, 0, 1), Major: 0})
	if _, err := m.Activate(key, "0.3.0"); err != nil {
		t.Fatal(err)
	}
	if err := m.StartTrial(); err != nil {
		t.Fatal(err)
	}
	c.t = t0.AddDate(0, 0, 2)
	if st := m.Status("0.3.0"); st.Tier != Trial {
		t.Fatalf("got %+v", st)
	}
}

func TestKeyForOlderMajorExplainsItself(t *testing.T) {
	m, c, priv, _ := newManager(t)
	if _, err := m.Activate(Sign(priv, Claims{Issued: t0, Major: 0}), "0.3.0"); err != nil {
		t.Fatal(err)
	}
	_ = c
	st := m.Status("1.0.0")
	if st.Tier != Free || !strings.Contains(st.Note, "older version") {
		t.Fatalf("got %+v", st)
	}
}

func TestUnreadableFilesGiveFree(t *testing.T) {
	pub, _ := keys(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "l.json")
	if err := writeFile(path, "{not json"); err != nil {
		t.Fatal(err)
	}
	if st := Open(path, pub, nil).Status("0.3.0"); st.Tier != Free || !st.CanTrial {
		t.Fatalf("got %+v", st)
	}
	// A key that no longer verifies (say, the file was edited) is explained, not fatal.
	if err := writeFile(path, `{"key":"TRIM1-x.y"}`); err != nil {
		t.Fatal(err)
	}
	if st := Open(path, pub, nil).Status("0.3.0"); st.Tier != Free || !strings.Contains(st.Note, "could not be checked") {
		t.Fatalf("got %+v", st)
	}
}

func TestSaveFailureIsReported(t *testing.T) {
	pub, _ := keys(t)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := writeFile(blocker, "x"); err != nil {
		t.Fatal(err)
	}
	// The parent "folder" is a file, so the licence file cannot be created.
	m := Open(filepath.Join(blocker, "licence.json"), pub, nil)
	if err := m.StartTrial(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDefaultPathIsInAppDir(t *testing.T) {
	if filepath.Base(Path()) != "licence.json" {
		t.Fatal(Path())
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "day") != "1 day" || plural(3, "day") != "3 days" {
		t.Fatal("plural")
	}
}
