// Package license decides whether this PC has Trimmel Plus: a signed licence
// key checked offline, or a 14-day trial the user starts by hand. Nothing here
// contacts a server, and nothing in the free app depends on it.
//
// A key looks like  TRIM1-<payload>.<signature>  where both parts are
// base64url (no padding): the payload is JSON, the signature is Ed25519 over
// the payload bytes. The private key lives only with the seller; the app
// carries the public key (publickey.go).
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	keyPrefix = "TRIM1-"
	TrialDays = 14
	// MaxDevices is how many PCs one licence covers. It is recorded in the key
	// for the optional online check; the offline check does not count PCs.
	MaxDevices = 3
)

// Tier names a level of access.
type Tier string

const (
	Free  Tier = "free"
	Trial Tier = "trial"
	Plus  Tier = "plus"
)

// Claims is the signed payload of a licence key.
type Claims struct {
	ID      string    `json:"id"`                // order reference, for support and refunds
	Email   string    `json:"email"`             // who it was issued to
	Issued  time.Time `json:"issued"`            //
	Expires time.Time `json:"expires,omitzero"`  // zero = never (lifetime)
	Major   int       `json:"major"`             // covers app versions up to this major version
	Devices int       `json:"devices,omitempty"` // PCs covered; 0 = MaxDevices
	Account string    `json:"account,omitempty"` // Google account id (sub) of the buyer; empty on older keys
}

var (
	ErrFormat     = errors.New("that does not look like a Trimmel Plus key")
	ErrSignature  = errors.New("that key is not valid")
	ErrExpired    = errors.New("that key has expired")
	ErrVersion    = errors.New("that key covers an older version of Trimmel")
	ErrNoKeyInApp = errors.New("this build of Trimmel cannot check licence keys")
)

// Holder is the signed-in Google account that is activating a key.
type Holder struct {
	Subject string
	Email   string
}

// ErrOtherAccount says a key was bought with a different Google account.
type ErrOtherAccount struct{ Email string }

func (e ErrOtherAccount) Error() string {
	who := "another Google account"
	if e.Email != "" {
		who = maskEmail(e.Email)
	}
	return "that key belongs to " + who + ". Sign in to Trimmel with that Google account to use it"
}

// maskEmail keeps enough of an address to be recognised without publishing it: a***@example.com.
func maskEmail(email string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" {
		return "another Google account"
	}
	return local[:1] + "***@" + domain
}

// belongsTo reports whether the claims were issued to this account. A key with an account id must
// match it; an older key without one is matched by email, ignoring case.
func (c Claims) belongsTo(h Holder) bool {
	if c.Account != "" && h.Subject != "" {
		return c.Account == h.Subject
	}
	return c.Email != "" && strings.EqualFold(c.Email, h.Email)
}

// Verify checks a key against the public key and the app's major version, and
// returns its claims. Leading and trailing whitespace (from pasting) is ignored.
func Verify(pub ed25519.PublicKey, key string, now time.Time, appMajor int) (Claims, error) {
	if len(pub) != ed25519.PublicKeySize {
		return Claims{}, ErrNoKeyInApp
	}
	key = strings.Join(strings.Fields(key), "")
	body, ok := strings.CutPrefix(key, keyPrefix)
	if !ok {
		return Claims{}, ErrFormat
	}
	p, s, ok := strings.Cut(body, ".")
	if !ok {
		return Claims{}, ErrFormat
	}
	payload, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return Claims{}, ErrFormat
	}
	sig, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Claims{}, ErrFormat
	}
	if !ed25519.Verify(pub, payload, sig) {
		return Claims{}, ErrSignature
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrFormat
	}
	if !c.Expires.IsZero() && !now.Before(c.Expires) {
		return c, ErrExpired
	}
	if appMajor > c.Major {
		return c, ErrVersion
	}
	return c, nil
}

// Sign builds a key from claims. The app never calls it; it is here so the
// issuing tool and the tests share one definition of the format.
func Sign(priv ed25519.PrivateKey, c Claims) string {
	payload, _ := json.Marshal(c)
	sig := ed25519.Sign(priv, payload)
	return keyPrefix + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)
}
