//go:build windows

package appinfo

import "golang.org/x/sys/windows/registry"

const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\Trimmel`

func registeredDir() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return ""
	}
	return dir
}
