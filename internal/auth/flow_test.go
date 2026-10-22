package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/zalando/go-keyring"
)

const (
	testClientID = "test-client"
	goodCode     = "good-code"
)

// idp is a small in-process OpenID Connect provider: discovery, keys and a
// token endpoint that checks the PKCE verifier, enough to run the real flows.
type idp struct {
	srv *httptest.Server
	key *rsa.PrivateKey

	mu         sync.Mutex
	nonce      string
	challenge  string
	forceNonce string
	claims     map[string]any
	refreshOK  string
	refreshOut string
	failStatus int
	failCode   string
}

func newIDP(t *testing.T) *idp {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &idp{key: key, refreshOK: "refresh-1", refreshOut: "refresh-1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                p.srv.URL,
			"authorization_endpoint":                p.srv.URL + "/auth",
			"token_endpoint":                        p.srv.URL + "/token",
			"jwks_uri":                              p.srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", p.token)
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *idp) token(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	p.mu.Lock()
	defer p.mu.Unlock()
	fail := func(status int, code string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": code})
	}
	if p.failStatus != 0 {
		fail(p.failStatus, p.failCode)
		return
	}
	nonce := p.nonce
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != goodCode || base64.RawURLEncoding.EncodeToString(sum[:]) != p.challenge {
			fail(http.StatusBadRequest, "invalid_grant")
			return
		}
	case "refresh_token":
		if r.Form.Get("refresh_token") != p.refreshOK {
			fail(http.StatusBadRequest, "invalid_grant")
			return
		}
		nonce = ""
	default:
		fail(http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	if p.forceNonce != "" {
		nonce = p.forceNonce
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "access",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": p.refreshOut,
		"id_token":      p.idToken(nonce),
	})
}

func (p *idp) idToken(nonce string) string {
	claims := map[string]any{
		"iss": p.srv.URL, "aud": testClientID, "sub": "user-1",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		"name": "Test User", "email": "test@example.com", "email_verified": true,
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	for k, v := range p.claims {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: p.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		panic(err)
	}
	payload, _ := json.Marshal(claims)
	obj, err := signer.Sign(payload)
	if err != nil {
		panic(err)
	}
	s, _ := obj.CompactSerialize()
	return s
}

func (p *idp) manager(open func(string)) *Manager {
	return &Manager{cfg: Config{Issuer: p.srv.URL, ClientID: testClientID}, open: open}
}

// browser stands in for the user's browser: it records the nonce and PKCE
// challenge from the sign-in URL, then follows the redirect with query.
func (p *idp) browser(t *testing.T, redirectQuery func(state string) string) func(string) {
	return func(authURL string) {
		u, err := url.Parse(authURL)
		if err != nil {
			t.Error(err)
			return
		}
		q := u.Query()
		p.mu.Lock()
		p.nonce, p.challenge = q.Get("nonce"), q.Get("code_challenge")
		p.mu.Unlock()
		if q.Get("code_challenge_method") != "S256" {
			t.Errorf("PKCE method must be S256, got %q", q.Get("code_challenge_method"))
		}
		resp, err := http.Get(q.Get("redirect_uri") + "?" + redirectQuery(q.Get("state")))
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
	}
}

func okQuery(state string) string { return "code=" + goodCode + "&state=" + url.QueryEscape(state) }

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestLoginSignsInAndStoresRefreshToken(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	m := p.manager(nil)
	m.open = p.browser(t, okQuery)

	u, err := m.Login(ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	if u.Subject != "user-1" || u.Name != "Test User" || u.Email != "test@example.com" {
		t.Errorf("unexpected user: %+v", u)
	}
	if st := m.Status(); st.User == nil || !st.Configured {
		t.Errorf("status should show the signed-in user: %+v", st)
	}
	if !m.Allowed() {
		t.Error("a signed-in user may run privileged calls")
	}
	if got, _ := keyring.Get(keyringService, keyringUser); got != "refresh-1" {
		t.Errorf("refresh token not stored, got %q", got)
	}
}

func TestLoginIgnoresRequestWithWrongState(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	var strayStatus int
	good := p.browser(t, okQuery)
	m := p.manager(func(authURL string) {
		u, _ := url.Parse(authURL)
		resp, err := http.Get(u.Query().Get("redirect_uri") + "?code=evil&state=wrong")
		if err != nil {
			t.Error(err)
			return
		}
		strayStatus = resp.StatusCode
		resp.Body.Close()
		good(authURL)
	})
	if _, err := m.Login(ctx(t)); err != nil {
		t.Fatalf("a stray request must not end the sign-in: %v", err)
	}
	if strayStatus != http.StatusBadRequest {
		t.Errorf("stray request got %d, want 400", strayStatus)
	}
}

func TestLoginReportsProviderError(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	m := p.manager(p.browser(t, func(state string) string {
		return "error=access_denied&error_description=User+said+no&state=" + url.QueryEscape(state)
	}))
	_, err := m.Login(ctx(t))
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("expected access_denied, got %v", err)
	}
	if m.Allowed() {
		t.Error("a failed sign-in must not unlock the app")
	}
}

func TestLoginWithoutCodeFails(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	m := p.manager(p.browser(t, func(state string) string { return "state=" + url.QueryEscape(state) }))
	if _, err := m.Login(ctx(t)); err == nil || !strings.Contains(err.Error(), "no authorization code") {
		t.Errorf("expected a missing code error, got %v", err)
	}
}

func TestLoginRejectsNonceMismatch(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	p.forceNonce = "replayed"
	m := p.manager(p.browser(t, okQuery))
	if _, err := m.Login(ctx(t)); err == nil || !strings.Contains(err.Error(), "nonce") {
		t.Errorf("expected a nonce error, got %v", err)
	}
	if m.Allowed() {
		t.Error("a replayed token must not unlock the app")
	}
}

func TestLoginRejectsUnverifiedEmail(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	p.claims = map[string]any{"email_verified": false}
	m := p.manager(p.browser(t, okQuery))
	if _, err := m.Login(ctx(t)); err == nil || !strings.Contains(err.Error(), "verified") {
		t.Errorf("expected an unverified email error, got %v", err)
	}
}

func TestLoginRejectsWrongAudience(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	p.claims = map[string]any{"aud": "someone-else"}
	m := p.manager(p.browser(t, okQuery))
	if _, err := m.Login(ctx(t)); err == nil || !strings.Contains(err.Error(), "invalid id_token") {
		t.Errorf("expected an invalid id_token error, got %v", err)
	}
}

func TestLoginRejectsExpiredToken(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	p.claims = map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}
	m := p.manager(p.browser(t, okQuery))
	if _, err := m.Login(ctx(t)); err == nil {
		t.Error("an expired id_token must be rejected")
	}
}

func TestLoginCanBeCancelled(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	var m *Manager
	m = p.manager(func(string) { go m.CancelLogin() })
	if _, err := m.Login(ctx(t)); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected a cancelled error, got %v", err)
	}
}

func TestLoginNeedsConfiguration(t *testing.T) {
	m := &Manager{}
	if _, err := m.Login(ctx(t)); err == nil {
		t.Error("login without configuration must fail")
	}
}

func TestLoginProviderUnreachable(t *testing.T) {
	m := &Manager{cfg: Config{Issuer: "http://127.0.0.1:1", ClientID: testClientID}, open: func(string) {}}
	if _, err := m.Login(ctx(t)); err == nil || !strings.Contains(err.Error(), "contacting") {
		t.Errorf("expected a contacting error, got %v", err)
	}
}

func TestRestoreWithoutTokenIsQuiet(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	m := p.manager(nil)
	st := m.Restore(ctx(t))
	if st.User != nil || st.Error != "" {
		t.Errorf("no stored token is not an error: %+v", st)
	}
}

func TestRestoreSignsInSilently(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	keyring.Set(keyringService, keyringUser, "refresh-1")
	m := p.manager(nil)
	st := m.Restore(ctx(t))
	if st.User == nil || st.User.Email != "test@example.com" || st.Error != "" {
		t.Errorf("expected a silent sign-in, got %+v", st)
	}
}

func TestRestoreStoresRotatedToken(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	p.refreshOut = "refresh-2"
	keyring.Set(keyringService, keyringUser, "refresh-1")
	m := p.manager(nil)
	m.Restore(ctx(t))
	if got, _ := keyring.Get(keyringService, keyringUser); got != "refresh-2" {
		t.Errorf("rotated token not kept, got %q", got)
	}
}

func TestRestoreDeletesTokenTheProviderRejects(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	keyring.Set(keyringService, keyringUser, "revoked")
	m := p.manager(nil)
	st := m.Restore(ctx(t))
	if st.User != nil || st.Error != "" {
		t.Errorf("a revoked token just means sign in again: %+v", st)
	}
	if _, err := keyring.Get(keyringService, keyringUser); err == nil {
		t.Error("a rejected token must be deleted")
	}
}

func TestRestoreKeepsTokenWhenOffline(t *testing.T) {
	keyring.MockInit()
	keyring.Set(keyringService, keyringUser, "refresh-1")
	m := &Manager{cfg: Config{Issuer: "http://127.0.0.1:1", ClientID: testClientID}}
	st := m.Restore(ctx(t))
	if st.User != nil || !strings.Contains(st.Error, "internet") {
		t.Errorf("offline should explain itself, got %+v", st)
	}
	if got, _ := keyring.Get(keyringService, keyringUser); got != "refresh-1" {
		t.Errorf("an offline start must keep the token, got %q", got)
	}
}

func TestRestoreKeepsTokenOnServerError(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	p.failStatus, p.failCode = http.StatusServiceUnavailable, "temporarily_unavailable"
	keyring.Set(keyringService, keyringUser, "refresh-1")
	m := p.manager(nil)
	st := m.Restore(ctx(t))
	if st.User != nil || st.Error == "" {
		t.Errorf("a server error should be reported, got %+v", st)
	}
	if got, _ := keyring.Get(keyringService, keyringUser); got != "refresh-1" {
		t.Error("a server error must not delete the token")
	}
}

func TestRestoreMovesLegacyToken(t *testing.T) {
	keyring.MockInit()
	p := newIDP(t)
	keyring.Set(legacyKeyringService, keyringUser, "refresh-1")
	m := p.manager(nil)
	st := m.Restore(ctx(t))
	if st.User == nil {
		t.Fatalf("legacy token should sign in: %+v", st)
	}
	if got, _ := keyring.Get(keyringService, keyringUser); got != "refresh-1" {
		t.Errorf("token not copied to the new entry, got %q", got)
	}
	if _, err := keyring.Get(legacyKeyringService, keyringUser); err == nil {
		t.Error("legacy entry should be removed")
	}
}

func TestRestoreSkippedWhenNotConfigured(t *testing.T) {
	keyring.MockInit()
	keyring.Set(keyringService, keyringUser, "refresh-1")
	m := &Manager{}
	if st := m.Restore(ctx(t)); st.Configured || st.User != nil {
		t.Errorf("unconfigured restore must do nothing: %+v", st)
	}
}

func TestLogoutClearsEverything(t *testing.T) {
	keyring.MockInit()
	keyring.Set(keyringService, keyringUser, "a")
	keyring.Set(legacyKeyringService, keyringUser, "b")
	m := &Manager{cfg: Config{Issuer: "x", ClientID: "y"}, user: &User{Name: "n"}}
	m.Logout()
	if m.Allowed() || m.Status().User != nil {
		t.Error("logout must lock the app again")
	}
	if _, err := keyring.Get(keyringService, keyringUser); err == nil {
		t.Error("token should be deleted")
	}
	if _, err := keyring.Get(legacyKeyringService, keyringUser); err == nil {
		t.Error("legacy token should be deleted")
	}
}

func TestAllowedWhenSignInIsNotConfigured(t *testing.T) {
	if !(&Manager{}).Allowed() {
		t.Error("with no sign-in configured the app opens straight away")
	}
	if (&Manager{cfg: Config{Issuer: "x", ClientID: "y"}}).Allowed() {
		t.Error("with sign-in configured and nobody signed in, privileged calls are blocked")
	}
}

func TestStatusReportsConfigError(t *testing.T) {
	m := &Manager{cfgErr: context.DeadlineExceeded}
	if st := m.Status(); st.Error == "" {
		t.Error("a config error must reach the UI")
	}
}

func TestScopesDifferForGoogle(t *testing.T) {
	p := newIDP(t)
	provider, err := oidc.NewProvider(ctx(t), p.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	google := &Manager{cfg: Config{Issuer: googleIssuer + "/", ClientID: "x"}}
	other := &Manager{cfg: Config{Issuer: "https://idp.example", ClientID: "x", Scopes: []string{"extra"}}}
	if !google.isGoogle() || other.isGoogle() {
		t.Fatal("isGoogle gave a wrong answer")
	}
	if has(google.oauthConfig(provider, "").Scopes, "offline_access") {
		t.Error("Google has no offline_access scope")
	}
	sc := other.oauthConfig(provider, "").Scopes
	if !has(sc, "offline_access") || !has(sc, "extra") || !has(sc, "openid") {
		t.Errorf("unexpected scopes %v", sc)
	}
}

func has(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestRandStringIsUnique(t *testing.T) {
	a, b := randString(), randString()
	if a == b || len(a) < 40 {
		t.Errorf("state values must be long and unique: %q %q", a, b)
	}
}
