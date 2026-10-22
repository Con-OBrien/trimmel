//go:build windows

package elevation

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsAdmin reports whether the process token is elevated.
func IsAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// OtherAccount reports whether this process runs as a different Windows user
// than the one signed in to the desktop. That happens when a standard user
// approves the admin prompt with an administrator's credentials: per-user
// data (temp files, browser caches, startup items, Recycle Bin) then belongs
// to the administrator, not the person using the PC. It returns the name of
// the account the app runs as.
func OtherAccount() (bool, string) {
	me, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false, ""
	}
	desktop := desktopUserSID()
	if desktop == nil || windows.EqualSid(me.User.Sid, desktop) {
		return false, ""
	}
	name := me.User.Sid.String()
	if acct, dom, _, err := me.User.Sid.LookupAccount(""); err == nil {
		name = dom + `\` + acct
	}
	return true, name
}

// desktopUserSID returns the owner of the running Explorer shell, or nil if
// it can't be determined.
func desktopUserSID() *windows.SID {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var sessionID uint32
	_ = windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &sessionID)

	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if !strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "explorer.exe") {
			continue
		}
		var sid uint32
		if windows.ProcessIdToSessionId(e.ProcessID, &sid) != nil || sid != sessionID {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, e.ProcessID)
		if err != nil {
			continue
		}
		var tok windows.Token
		err = windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok)
		windows.CloseHandle(h)
		if err != nil {
			continue
		}
		u, err := tok.GetTokenUser()
		tok.Close()
		if err != nil {
			continue
		}
		return u.User.Sid
	}
	return nil
}
