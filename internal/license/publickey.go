package license

import (
	"crypto/ed25519"
	"encoding/base64"
)

// publicKeyB64 is the base64 (standard) Ed25519 public key that licence keys
// are checked against. It is empty until the seller generates a key pair with
// tools/licensegen; until then Plus keys cannot be activated, and the trial is
// the only way to try Plus. Set it in source or at build time:
//
//	go build -ldflags "-X trimmel/internal/license.publicKeyB64=<key>"
var publicKeyB64 = ""

// PublicKey returns the built-in key, or nil if this build has none.
func PublicKey() ed25519.PublicKey {
	raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil
	}
	return ed25519.PublicKey(raw)
}
