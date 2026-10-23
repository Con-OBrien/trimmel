package license

import (
	"encoding/base64"
	"os"
)

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func stdB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func writeFile(path, s string) error { return os.WriteFile(path, []byte(s), 0o600) }
