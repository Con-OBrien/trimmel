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

	"pcoptimizer/internal/appdir"
)

const (
	keyringService       = "trimmel"
	legacyKeyringService = "pcoptimizer" // PC Optimizer 0.1.x; read once, then moved
	keyringUser          = "oidc-refresh-token"
	loginTimeout         = 5 * time.Minute
)

// Config is read from %APPDATA%\trimmel\auth.json, with PCOPT_OIDC_*
// environment variables taking precedence.
type Config struct {
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret,omitempty"` // only for providers that insist on one; PKCE protects public clients
	Scopes       []string `json:"scopes,omitempty"`       // "openid" is always added
}

func (c Config) Enabled() bool { return c.Issuer != "" && c.ClientID != "" }

func ConfigPath() string {
	return filepath.Join(appdir.Dir(), "auth.json")
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
	mu         sync.Mutex
	cfg        Config
	cfgErr     error
	user       *User
	restoreErr string             // set when a silent sign-in failed for a non-credential reason
	cancel     context.CancelFunc // in-flight login
	open       func(url string)   // opens the system browser
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
	} else if m.restoreErr != "" {
		s.Error = m.restoreErr
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
		u, err := m.restore(ctx)
		m.mu.Lock()
		m.restoreErr = ""
		switch {
		case err == nil:
			m.user = u
		case errors.Is(err, keyring.ErrNotFound):
		case rejectedByProvider(err):
			_ = keyring.Delete(keyringService, keyringUser) // revoked or expired
		default:
			// Offline or provider unreachable: keep the token so the next
			// launch with a connection signs in silently.
			m.restoreErr = "Couldn't reach the sign-in service. Check your internet connection and try again."
		}
		m.mu.Unlock()
	}
	return m.Status()
}

func (m *Manager) restore(ctx context.Context) (*User, error) {
	rt, fromLegacy, err := getToken()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
	} else if fromLegacy { // signed in under the old name: move the token to the new entry
		_ = keyring.Set(keyringService, keyringUser, rt)
	}
	if fromLegacy {
		_ = keyring.Delete(legacyKeyringService, keyringUser)
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
		if q.Get("state") != state {
			// Not our redirect (a stray or hostile request). Ignore it so it
			// can't end the sign-in the user is completing in the browser.
			http.Error(w, "Unexpected request", http.StatusBadRequest)
			return
		}
		var res result
		switch {
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
			fmt.Fprint(w, page("Signed in", "You can close this tab and return to Trimmel."))
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
	_ = keyring.Delete(legacyKeyringService, keyringUser)
	m.mu.Lock()
	m.user = nil
	m.mu.Unlock()
}

// rejectedByProvider reports whether err means the provider refused the stored
// credentials (as opposed to a network or server problem).
func rejectedByProvider(err error) bool {
	var re *oauth2.RetrieveError
	if !errors.As(err, &re) {
		return false
	}
	return re.ErrorCode == "invalid_grant" || re.ErrorCode == "invalid_client" || re.ErrorCode == "unauthorized_client"
}

// getToken reads the stored refresh token, falling back to the entry written
// by PC Optimizer 0.1.x so existing users stay signed in after the rename.
func getToken() (token string, fromLegacy bool, err error) {
	token, err = keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		if lt, lerr := keyring.Get(legacyKeyringService, keyringUser); lerr == nil {
			return lt, true, nil
		}
	}
	return token, false, err
}

func userFrom(idt *oidc.IDToken) (*User, error) {
	var c struct {
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		EmailVerified     *bool  `json:"email_verified"`
		Picture           string `json:"picture"`
	}
	if err := idt.Claims(&c); err != nil {
		return nil, err
	}
	if c.Email != "" && c.EmailVerified != nil && !*c.EmailVerified {
		return nil, errors.New("that account's email address isn't verified")
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
	return `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + title + `</title>
<body style="font-family:'Segoe UI',system-ui,sans-serif;background:radial-gradient(ellipse at 50% 0%,#ffe9c9 0%,#fff8f1 60%);color:#3a2b3d;display:grid;place-items:center;min-height:100vh;margin:0">
<div style="text-align:center;background:#fff;border:1px solid #f0e0cf;border-radius:28px;padding:40px 44px;max-width:380px;box-shadow:0 6px 18px rgba(92,56,40,.07)">
<svg width="64" height="64" viewBox="0 0 32 32"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ffc25a"/><stop offset="1" stop-color="#ff9d6c"/></linearGradient></defs><rect width="32" height="32" rx="8" fill="#3b2a3f"/><path d="M6 17a10 10 0 0 1 20 0" fill="none" stroke="#5a4560" stroke-width="3" stroke-linecap="round"/><path d="M6 17a10 10 0 0 1 15 -8.66" fill="none" stroke="url(#g)" stroke-width="3" stroke-linecap="round"/><path d="M17.5 8.5 12.5 18h4l-1.5 6 6-9.5h-4z" fill="url(#g)" transform="translate(16 20) scale(.62) translate(-16.75 -16.25)"/></svg>
<h2 style="margin:16px 0 8px">` + title + `</h2><p style="margin:0;color:#86768a">` + body + `</p></div>`
}
