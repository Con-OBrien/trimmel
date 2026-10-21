//go:build windows

package procs

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	versionDLL     = windows.NewLazySystemDLL("version.dll")
	pGetInfoSize   = versionDLL.NewProc("GetFileVersionInfoSizeW")
	pGetInfo       = versionDLL.NewProc("GetFileVersionInfoW")
	pVerQueryValue = versionDLL.NewProc("VerQueryValueW")
)

// readVersionInfo returns the description and publisher embedded in an exe's
// version resource, e.g. "Google Chrome" / "Google LLC".
func readVersionInfo(path string) (desc, vendor string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	size, _, _ := pGetInfoSize.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 {
		return
	}
	buf := make([]byte, size)
	if r, _, _ := pGetInfo.Call(uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&buf[0]))); r == 0 {
		return
	}

	query := func(sub string) (unsafe.Pointer, uint32) {
		sp, _ := syscall.UTF16PtrFromString(sub)
		var ptr unsafe.Pointer
		var n uint32
		r, _, _ := pVerQueryValue.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(sp)),
			uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&n)))
		if r == 0 || ptr == nil {
			return nil, 0
		}
		return ptr, n
	}

	// First language/codepage pair, falling back to US English / Unicode.
	lang := "040904b0"
	if ptr, n := query(`\VarFileInfo\Translation`); ptr != nil && n >= 4 {
		t := (*[2]uint16)(ptr)
		lang = fmt.Sprintf("%04x%04x", t[0], t[1])
	}
	str := func(key string) string {
		ptr, n := query(`\StringFileInfo\` + lang + `\` + key)
		if ptr == nil || n == 0 {
			return ""
		}
		return strings.TrimSpace(windows.UTF16PtrToString((*uint16)(ptr)))
	}
	desc = str("FileDescription")
	if desc == "" {
		desc = str("ProductName")
	}
	return desc, str("CompanyName")
}
