//go:build windows

package appinfo

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func serve(t *testing.T, status int, body string) {
	t.Helper()
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != versionURL {
			t.Errorf("unexpected URL %s", r.URL)
		}
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}, nil
	})
	t.Cleanup(func() { http.DefaultClient.Transport = old })
}

func TestCheckUpdateReportsNewerRelease(t *testing.T) {
	serve(t, 200, `{"version":"0.3.0","date":"2026-01-02"}`)
	u, err := CheckUpdate(context.Background(), "0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if !u.Available || u.Latest != "0.3.0" || u.Current != "0.2.1" || u.Date != "2026-01-02" || !strings.HasPrefix(u.URL, SiteURL) {
		t.Errorf("unexpected update: %+v", u)
	}
}

func TestCheckUpdateCurrentIsLatest(t *testing.T) {
	serve(t, 200, `{"version":"0.2.1"}`)
	u, err := CheckUpdate(context.Background(), "0.2.1")
	if err != nil || u.Available {
		t.Errorf("same version is not an update: %+v %v", u, err)
	}
}

func TestCheckUpdateDevBuildNeverOffersUpdate(t *testing.T) {
	serve(t, 200, `{"version":"0.3.0"}`)
	u, err := CheckUpdate(context.Background(), "dev")
	if err != nil || u.Available {
		t.Errorf("an unparsable current version must not claim an update: %+v %v", u, err)
	}
}

func TestCheckUpdateErrors(t *testing.T) {
	serve(t, 500, `{}`)
	if _, err := CheckUpdate(context.Background(), "0.2.1"); err == nil || !strings.Contains(err.Error(), "answered") {
		t.Errorf("a server error should be reported, got %v", err)
	}
	serve(t, 200, `not json`)
	if _, err := CheckUpdate(context.Background(), "0.2.1"); err == nil || !strings.Contains(err.Error(), "readable") {
		t.Errorf("bad JSON should be reported, got %v", err)
	}
	serve(t, 200, `{"date":"x"}`)
	if _, err := CheckUpdate(context.Background(), "0.2.1"); err == nil {
		t.Error("a missing version should be reported")
	}
}

func TestCheckUpdateUnreachable(t *testing.T) {
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	if _, err := CheckUpdate(context.Background(), "0.2.1"); err == nil || !strings.Contains(err.Error(), "reach") {
		t.Errorf("expected a connection message, got %v", err)
	}
}

func TestCurrentDescribesRunningExe(t *testing.T) {
	in, err := Current()
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	if in.Exe == "" || in.Dir != filepath.Dir(in.Exe) || in.SettingsDir == "" {
		t.Errorf("unexpected install info: %+v (exe %s)", in, exe)
	}
	if in.Installed {
		t.Error("a test binary is not the registered install")
	}
}

func TestSettingsDirsAndUninstallerPath(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if filepath.Base(SettingsDir()) != "trimmel" || filepath.Base(LegacySettingsDir()) != "pcoptimizer" {
		t.Errorf("unexpected settings folders: %s, %s", SettingsDir(), LegacySettingsDir())
	}
	if got := UninstallerPath(`C:\x`); got != `C:\x\uninstall.exe` {
		t.Errorf("got %s", got)
	}
}
