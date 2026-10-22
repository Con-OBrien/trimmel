//go:build windows

package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func isolatedConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	for _, k := range []string{
		"TRIMMEL_OIDC_ISSUER", "TRIMMEL_OIDC_CLIENT_ID", "TRIMMEL_OIDC_CLIENT_SECRET",
		"PCOPT_OIDC_ISSUER", "PCOPT_OIDC_CLIENT_ID", "PCOPT_OIDC_CLIENT_SECRET",
	} {
		t.Setenv(k, "")
	}
	return dir
}

func writeConfig(t *testing.T, appdata, body string) {
	t.Helper()
	dir := filepath.Join(appdata, "trimmel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigDefaultsToBuiltInGoogle(t *testing.T) {
	isolatedConfig(t)
	oldID, oldSecret := GoogleClientID, GoogleClientSecret
	GoogleClientID, GoogleClientSecret = "built-in-id", "built-in-secret"
	t.Cleanup(func() { GoogleClientID, GoogleClientSecret = oldID, oldSecret })

	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Issuer != googleIssuer || c.ClientID != "built-in-id" || c.ClientSecret != "built-in-secret" || !c.Enabled() {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestLoadConfigWithoutClientIDIsDisabled(t *testing.T) {
	isolatedConfig(t)
	oldID := GoogleClientID
	GoogleClientID = ""
	t.Cleanup(func() { GoogleClientID = oldID })

	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled() {
		t.Errorf("a build without a client ID must open without sign-in: %+v", c)
	}
}

func TestLoadConfigFileOverridesDefaults(t *testing.T) {
	appdata := isolatedConfig(t)
	writeConfig(t, appdata, `{"issuer":"https://idp.example","clientId":"from-file","scopes":["groups"]}`)

	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Issuer != "https://idp.example" || c.ClientID != "from-file" || len(c.Scopes) != 1 {
		t.Errorf("unexpected config: %+v", c)
	}
}

func TestLoadConfigEnvironmentBeatsFile(t *testing.T) {
	appdata := isolatedConfig(t)
	writeConfig(t, appdata, `{"issuer":"https://file.example","clientId":"from-file"}`)
	t.Setenv("TRIMMEL_OIDC_ISSUER", "  https://env.example  ")
	t.Setenv("TRIMMEL_OIDC_CLIENT_ID", " from-env ")
	t.Setenv("TRIMMEL_OIDC_CLIENT_SECRET", "s3cret")

	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Issuer != "https://env.example" || c.ClientID != "from-env" || c.ClientSecret != "s3cret" {
		t.Errorf("environment should win and be trimmed: %+v", c)
	}
}

func TestLoadConfigAcceptsOldVariableNames(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("PCOPT_OIDC_ISSUER", "https://old.example")
	t.Setenv("PCOPT_OIDC_CLIENT_ID", "old-id")

	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Issuer != "https://old.example" || c.ClientID != "old-id" {
		t.Errorf("the pre-0.2.0 names should still work: %+v", c)
	}
	t.Setenv("TRIMMEL_OIDC_CLIENT_ID", "new-id")
	if c, _ := LoadConfig(); c.ClientID != "new-id" {
		t.Errorf("the new name should win over the old one: %+v", c)
	}
}

func TestLoadConfigReportsBrokenFile(t *testing.T) {
	appdata := isolatedConfig(t)
	writeConfig(t, appdata, `{not json`)
	if _, err := LoadConfig(); err == nil {
		t.Error("a broken auth.json must be reported, not ignored")
	}
}

func TestNewManagerKeepsConfigError(t *testing.T) {
	appdata := isolatedConfig(t)
	writeConfig(t, appdata, `{not json`)
	m := NewManager(func(string) {})
	if st := m.Status(); st.Error == "" || st.ConfigPath == "" {
		t.Errorf("status should show the config problem: %+v", st)
	}
}
