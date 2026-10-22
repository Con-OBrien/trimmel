package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateCopiesLegacySettings(t *testing.T) {
	base := t.TempDir()
	write(t, filepath.Join(base, Legacy, "state.json"), `{"welcomed":true}`)
	write(t, filepath.Join(base, Legacy, "sub", "auth.json"), `{"issuer":"x"}`)

	migrate(base)

	got, err := os.ReadFile(filepath.Join(base, Name, "state.json"))
	if err != nil || string(got) != `{"welcomed":true}` {
		t.Fatalf("state.json not copied: %v %q", err, got)
	}
	if _, err := os.Stat(filepath.Join(base, Name, "sub", "auth.json")); err != nil {
		t.Errorf("nested file not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, Legacy, "state.json")); err != nil {
		t.Errorf("legacy folder must be left in place: %v", err)
	}
}

func TestMigrateNeverOverwritesNewSettings(t *testing.T) {
	base := t.TempDir()
	write(t, filepath.Join(base, Legacy, "state.json"), "old")
	write(t, filepath.Join(base, Name, "state.json"), "new")

	migrate(base)

	got, _ := os.ReadFile(filepath.Join(base, Name, "state.json"))
	if string(got) != "new" {
		t.Fatalf("existing settings were overwritten: %q", got)
	}
}

func TestMigrateWithoutLegacyDoesNothing(t *testing.T) {
	base := t.TempDir()
	migrate(base)
	if _, err := os.Stat(filepath.Join(base, Name)); err == nil {
		t.Fatal("no folder should be created when there is nothing to migrate")
	}
}
