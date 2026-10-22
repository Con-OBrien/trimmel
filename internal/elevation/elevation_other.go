//go:build !windows

package elevation

import (
	"os"
)

func IsAdmin() bool { return os.Geteuid() == 0 }


func OtherAccount() (bool, string) { return false, "" }
