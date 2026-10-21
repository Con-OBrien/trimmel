//go:build !windows

package procs

func readVersionInfo(path string) (desc, vendor string) { return "", "" }
