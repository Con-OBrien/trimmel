//go:build windows

package cleanup

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32               = windows.NewLazySystemDLL("shell32.dll")
	procSHQueryRecycleBin = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBin = shell32.NewProc("SHEmptyRecycleBinW")
)

// SHQUERYRBINFO. Go's natural alignment matches the 64-bit Windows layout
// (4-byte cbSize, 4 bytes padding, two int64s = 24 bytes).
type shQueryRBInfo struct {
	cbSize      uint32
	i64Size     int64
	i64NumItems int64
}

const (
	sherbNoConfirmation = 0x1
	sherbNoProgressUI   = 0x2
	sherbNoSound        = 0x4
	eUnexpected         = 0x8000FFFF // returned when the bin is already empty
)

func queryRecycleBin() (uint64, int64, error) {
	info := shQueryRBInfo{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	hr, _, _ := procSHQueryRecycleBin.Call(0, uintptr(unsafe.Pointer(&info)))
	if hr != 0 {
		return 0, 0, fmt.Errorf("SHQueryRecycleBin failed: 0x%08X", uint32(hr))
	}
	return uint64(info.i64Size), info.i64NumItems, nil
}

func emptyRecycleBin() (uint64, int64, error) {
	size, items, err := queryRecycleBin()
	if err != nil {
		return 0, 0, err
	}
	if items == 0 {
		return 0, 0, nil
	}
	hr, _, _ := procSHEmptyRecycleBin.Call(0, 0, sherbNoConfirmation|sherbNoProgressUI|sherbNoSound)
	if hr != 0 && uint32(hr) != eUnexpected {
		return 0, 0, fmt.Errorf("SHEmptyRecycleBin failed: 0x%08X", uint32(hr))
	}
	return size, items, nil
}
