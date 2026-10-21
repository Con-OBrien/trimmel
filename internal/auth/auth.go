// Package auth signs the user in with OpenID Connect (OAuth 2.0 authorization
// code flow with PKCE, RFC 7636) using the system browser and a loopback
// redirect (RFC 8252). It works with any standards-compliant provider
// (Entra ID, Google, Okta, Auth0, Keycloak, ...) via OIDC discovery.
//
// Only the refresh token is persisted, in the OS credential store. ID tokens
// are verified (signature, issuer, audience, expiry, nonce) before the user
// is considered signed in.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	keyringService = "pcoptimizer"
	keyringUser    = "oidc-refresh-token"
	loginTimeout   = 5 * time.Minute
)

// Config is read from %APPDATA%\pcoptimizer\auth.json, with PCOPT_OIDC_*
// environment variables taking precedence.
type Config struct {
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret,omitempty"` // only for providers that insist on one; PKCE protects public clients
	Scopes       []string `json:"scopes,omitempty"`       // "openid" is always added
}

func (c Config) Enabled() bool { return c.Issuer != "" && c.ClientID != "" }

func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "pcoptimizer", "auth.json")
}

func LoadConfig() (Config, error) {
	c := Config{Issuer: googleIssuer, ClientID: GoogleClientID, ClientSecret: GoogleClientSecret}
	data, err := os.ReadFile(ConfigPath()) // optional override for developers
	if err == nil {
		if err := json.Unmarshal(data, &c); err != nil {
			return c, fmt.Errorf("%s: %w", ConfigPath(), err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	if v := os.Getenv("PCOPT_OIDC_ISSUER"); v != "" {
		c.Issuer = v
	}
	if v := os.Getenv("PCOPT_OIDC_CLIENT_ID"); v != "" {
		c.ClientID = v
	}
	if v := os.Getenv("PCOPT_OIDC_CLIENT_SECRET"); v != "" {
		c.ClientSecret = v
	}
	c.Issuer = strings.TrimSpace(c.Issuer)
	c.ClientID = strings.TrimSpace(c.ClientID)
	return c, nil
}

type User struct {
	Subject string `json:"subject"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Picture string `json:"picture"`
}

// Status is what the frontend needs to decide between the app and the login screen.
type Status struct {
	Configured bool   `json:"configured"`
	User       *User  `json:"user"`
	ConfigPath string `json:"configPath"`
	Error      string `json:"error,omitempty"` // config problem, if any
}

type Manager struct {
	mu     sync.Mutex
	cfg    Config
	cfgErr error
	user   *User
	cancel context.CancelFunc // in-flight login
	open   func(url string)   // opens the system browser
}

func NewManager(openBrowser func(url string)) *Manager {
	cfg, err := LoadConfig()
	return &Manager{cfg: cfg, cfgErr: err, open: openBrowser}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Status{Configured: m.cfg.Enabled(), User: m.user, ConfigPath: ConfigPath()}
	if m.cfgErr != nil {
		s.Error = m.cfgErr.Error()
	}
	return s
}

// Allowed reports whether privileged operations may run: always when sign-in
// is not configured, otherwise only once a user is signed in.
func (m *Manager) Allowed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.cfg.Enabled() || m.user != nil
}

func (m *Manager) oauthConfig(provider *oidc.Provider, redirect string) *oauth2.Config {
	scopes := []string{oidc.ScopeOpenID, "profile", "email"}
	if !m.isGoogle() { // Google has no offline_access scope; see Login
		scopes = append(scopes, oidc.ScopeOfflineAccess)
	}
	return &oauth2.Config{
		ClientID:     m.cfg.ClientID,
		ClientSecret: m.cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirect,
		Scopes:       append(scopes, m.cfg.Scopes...),
	}
}

func (m *Manager) isGoogle() bool {
	return strings.TrimRight(m.cfg.Issuer, "/") == googleIssuer
}

// Restore signs in silently from the stored refresh token. A missing or
// rejected token is not an error: the user just has to sign in again.
func (m *Manager) Restore(ctx context.Context) Status {
	if m.cfg.Enabled() {
		if u, err := m.restore(ctx); err == nil {
			m.mu.Lock()
			m.user = u
			m.mu.Unlock()
		} else if !errors.Is(err, keyring.ErrNotFound) {
			_ = keyring.Delete(keyringService, keyringUser) // stale or revoked
		}
	}
	return m.Status()
}

func (m *Manager) restore(ctx context.Context) (*User, error) {
	rt, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, m.cfg.Issuer)
	if err != nil {
		return nil, err
	}
	conf := m.oauthConfig(provider, "")
	tok, err := conf.TokenSource(ctx, &oauth2.Token{RefreshToken: rt}).Token()
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken != "" && tok.RefreshToken != rt { // rotated
		_ = keyring.Set(keyringService, keyringUser, tok.RefreshToken)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return nil, errors.New("no id_token on refresh")
	}
	idt, err := provider.Verifier(&oidc.Config{ClientID: m.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	return userFrom(idt)
}

// Login runs the interactive flow and blocks until the user finishes in the
// browser, cancels, or the timeout passes.
func (m *Manager) Login(ctx context.Context) (*User, error) {
	if !m.cfg.Enabled() {
		return nil, errors.New("sign-in is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.cancel = cancel
	m.mu.Unlock()
	defer cancel()

	provider, err := oidc.NewProvider(ctx, m.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("contacting %s: %w", m.cfg.Issuer, err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer ln.Close()
	redirect := fmt.Sprintf("http://%s/callback", ln.Addr().String())
	conf := m.oauthConfig(provider, redirect)

	state, nonce, verifier := randString(), randString(), oauth2.GenerateVerifier()
	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce)}
	if m.isGoogle() {
		// Google only returns a refresh token with access_type=offline, and
		// only when the consent screen is shown.
		opts = append(opts,
			oauth2.SetAuthURLParam("access_type", "offline"),
			oauth2.SetAuthURLParam("prompt", "select_account consent"))
	} else {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "select_account"))
	}
	authURL := conf.AuthCodeURL(state, opts...)

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var res result
		switch {
		case q.Get("state") != state:
			res.err = errors.New("state mismatch")
		case q.Get("error") != "":
			res.err = fmt.Errorf("%s: %s", q.Get("error"), q.Get("error_description"))
		case q.Get("code") == "":
			res.err = errors.New("no authorization code returned")
		default:
			res.code = q.Get("code")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if res.err != nil {
			fmt.Fprint(w, page("Sign-in failed", html.EscapeString(res.err.Error())))
		} else {
			fmt.Fprint(w, page("Signed in", "You can close this tab and return to PC Optimizer."))
		}
		select {
		case done <- res:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	defer srv.Close()

	m.open(authURL)

	var res result
	select {
	case res = <-done:
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("sign-in timed out")
		}
		return nil, errors.New("sign-in cancelled")
	}
	if res.err != nil {
		return nil, res.err
	}

	tok, err := conf.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return nil, errors.New("provider returned no id_token")
	}
	idt, err := provider.Verifier(&oidc.Config{ClientID: m.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid id_token: %w", err)
	}
	if idt.Nonce != nonce {
		return nil, errors.New("nonce mismatch")
	}
	user, err := userFrom(idt)
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken != "" {
		_ = keyring.Set(keyringService, keyringUser, tok.RefreshToken)
	}
	m.mu.Lock()
	m.user = user
	m.mu.Unlock()
	return user, nil
}

func (m *Manager) CancelLogin() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) Logout() {
	_ = keyring.Delete(keyringService, keyringUser)
	m.mu.Lock()
	m.user = nil
	m.mu.Unlock()
}

func userFrom(idt *oidc.IDToken) (*User, error) {
	var c struct {
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Picture           string `json:"picture"`
	}
	if err := idt.Claims(&c); err != nil {
		return nil, err
	}
	name := c.Name
	if name == "" {
		name = c.PreferredUsername
	}
	if name == "" {
		name = c.Email
	}
	return &User{Subject: idt.Subject, Name: name, Email: c.Email, Picture: c.Picture}, nil
}

func randString() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func page(title, body string) string {
	return `<!doctype html><meta charset="utf-8"><title>` + title + `</title>
<body style="font-family:Segoe UI,sans-serif;background:#111318;color:#e6e8ee;display:grid;place-items:center;height:100vh;margin:0">
<div style="text-align:center"><h2>` + title + `</h2><p>` + body + `</p></div>`
}
