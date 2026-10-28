package license

import (
	"crypto/ed25519"
	"encoding/base64"
)

// publicKeyB64 is the base64 (standard) Ed25519 public key that licence keys
// are checked against. The matching private key signs keys on the website
// (docs/api) and in tools/licensegen; it is never in this repository. To use a
// different key pair, edit this value or override it at build time:
//
//	go build -ldflags "-X trimmel/internal/license.publicKeyB64=<key>"
var publicKeyB64 = "AOUHhG0sg2v7Yglvd+C3lOdWdoB6ZQ4b3utu5ejSgZA="

// PublicKey returns the built-in key, or nil if this build has none.
func PublicKey() ed25519.PublicKey {
	raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil
	}
	return ed25519.PublicKey(raw)
}
