package main

import "os"

func writeText(path, s string) error { return os.WriteFile(path, []byte(s), 0o600) }
