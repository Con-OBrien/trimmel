// Command licensegen is the seller's tool for Trimmel Plus keys. It is not
// part of the app.
//
//	licensegen keygen -out plus-private.key
//	licensegen sign   -key plus-private.key -email a@b.c -id ORDER123 [-years 1] [-major 0]
//
// keygen writes a new private key (keep it secret and backed up; anyone who
// has it can mint keys) and prints the public key to paste into
// internal/license/publickey.go. sign prints one licence key. Leave -years
// off for a lifetime key.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"trimmel/internal/license"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "licensegen:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer, now time.Time) error {
	if len(args) == 0 {
		return errors.New("usage: licensegen keygen|sign [flags]")
	}
	switch args[0] {
	case "keygen":
		return keygen(args[1:], out)
	case "sign":
		return sign(args[1:], out, now)
	}
	return fmt.Errorf("unknown command %q (want keygen or sign)", args[0])
}

func keygen(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	path := fs.String("out", "", "file to write the private key to (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return errors.New("keygen: -out is required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	// O_EXCL: never overwrite an existing key, which would orphan every key already sold.
	f, err := os.OpenFile(*path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, base64.StdEncoding.EncodeToString(priv)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(out, "Private key written to %s. Keep it secret and back it up.\n", *path)
	fmt.Fprintf(out, "Public key (set publicKeyB64 in internal/license/publickey.go):\n%s\n", base64.StdEncoding.EncodeToString(pub))
	return nil
}

func sign(args []string, out io.Writer, now time.Time) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	keyPath := fs.String("key", "", "private key file from keygen (required)")
	email := fs.String("email", "", "buyer's email (required)")
	id := fs.String("id", "", "order reference (required)")
	years := fs.Int("years", 0, "validity in years; 0 = lifetime")
	major := fs.Int("major", 0, "highest app major version the key covers")
	devices := fs.Int("devices", license.MaxDevices, "PCs covered")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *keyPath == "" || *email == "" || *id == "" {
		return errors.New("sign: -key, -email and -id are required")
	}
	if *years < 0 || *major < 0 || *devices < 1 {
		return errors.New("sign: -years and -major cannot be negative and -devices must be at least 1")
	}
	raw, err := os.ReadFile(*keyPath)
	if err != nil {
		return err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(seed) != ed25519.PrivateKeySize {
		return errors.New("sign: that is not a private key file made by keygen")
	}
	c := license.Claims{ID: *id, Email: *email, Issued: now.UTC().Truncate(time.Second), Major: *major, Devices: *devices}
	if *years > 0 {
		c.Expires = c.Issued.AddDate(*years, 0, 0)
	}
	fmt.Fprintln(out, license.Sign(ed25519.PrivateKey(seed), c))
	return nil
}
