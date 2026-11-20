//go:build windows

package core

import (
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	inetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	backupKey       = `Software\OperaVPN\ProxyBackup`

	proxyBypass = "localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;" +
		"172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;" +
		"172.31.*;192.168.*;<local>"
)

func platformSupportsSystemProxy() bool { return true }

// The values we touch, in the order they are backed up and restored.
var (
	dwordValues  = []string{"ProxyEnable"}
	stringValues = []string{"ProxyServer", "ProxyOverride", "AutoConfigURL"}
)

func enableSystemProxy(address string, bypassIR bool) error {
	host, port := splitAddress(address)
	settings, err := registry.OpenKey(registry.CURRENT_USER, inetSettingsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer settings.Close()

	// Remember the user's own configuration once, so Disable can put it back
	// exactly (also after a crash, see recoverSystemProxy).
	if _, err := registry.OpenKey(registry.CURRENT_USER, backupKey, registry.QUERY_VALUE); err != nil {
		if err := saveProxyBackup(settings); err != nil {
			logProxy("could not back up the current proxy settings: %v", err)
		}
	}

	// Chrome, Edge, Firefox ("use system settings"), WinHTTP-aware apps and
	// most .NET/Go programs read these WinINET values. The engine speaks HTTP
	// proxy, so the same address is used for http and https.
	if err := settings.SetStringValue("ProxyServer", "http="+host+":"+port+";https="+host+":"+port); err != nil {
		return err
	}
	override := proxyBypass
	if bypassIR {
		override = "*.ir;" + override
	}
	if err := settings.SetStringValue("ProxyOverride", override); err != nil {
		return err
	}
	// A PAC script would win over the manual proxy; park it while connected.
	_ = settings.DeleteValue("AutoConfigURL")
	if err := settings.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	notifyWininet()
	return nil
}

func disableSystemProxy() error {
	settings, err := registry.OpenKey(registry.CURRENT_USER, inetSettingsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer settings.Close()

	if !restoreProxyBackup(settings) {
		// No backup: just switch the manual proxy off.
		if err := settings.SetDWordValue("ProxyEnable", 0); err != nil {
			return err
		}
	}
	notifyWininet()
	return nil
}

func recoverSystemProxy() {
	if _, err := registry.OpenKey(registry.CURRENT_USER, backupKey, registry.QUERY_VALUE); err != nil {
		return // nothing left behind
	}
	_ = disableSystemProxy()
}

func saveProxyBackup(settings registry.Key) error {
	bk, _, err := registry.CreateKey(registry.CURRENT_USER, backupKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer bk.Close()
	for _, name := range dwordValues {
		if v, _, err := settings.GetIntegerValue(name); err == nil {
			_ = bk.SetDWordValue("had_"+name, 1)
			_ = bk.SetDWordValue("val_"+name, uint32(v))
		} else {
			_ = bk.SetDWordValue("had_"+name, 0)
		}
	}
	for _, name := range stringValues {
		if v, _, err := settings.GetStringValue(name); err == nil {
			_ = bk.SetDWordValue("had_"+name, 1)
			_ = bk.SetStringValue("val_"+name, v)
		} else {
			_ = bk.SetDWordValue("had_"+name, 0)
		}
	}
	return nil
}

// restoreProxyBackup writes the saved values back and deletes the backup.
func restoreProxyBackup(settings registry.Key) bool {
	bk, err := registry.OpenKey(registry.CURRENT_USER, backupKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	for _, name := range dwordValues {
		if had, _, err := bk.GetIntegerValue("had_" + name); err == nil && had == 1 {
			if v, _, err := bk.GetIntegerValue("val_" + name); err == nil {
				_ = settings.SetDWordValue(name, uint32(v))
			}
		} else {
			_ = settings.DeleteValue(name)
		}
	}
	for _, name := range stringValues {
		if had, _, err := bk.GetIntegerValue("had_" + name); err == nil && had == 1 {
			if v, _, err := bk.GetStringValue("val_" + name); err == nil {
				_ = settings.SetStringValue(name, v)
			}
		} else {
			_ = settings.DeleteValue(name)
		}
	}
	bk.Close()
	_ = registry.DeleteKey(registry.CURRENT_USER, backupKey)
	return true
}

var (
	wininet               = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOption = wininet.NewProc("InternetSetOptionW")
)

const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

// notifyWininet tells running applications that the proxy settings changed,
// so they pick the new values up without a restart.
func notifyWininet() {
	_, _, _ = procInternetSetOption.Call(0, internetOptionSettingsChanged, 0, 0)
	_, _, _ = procInternetSetOption.Call(0, internetOptionRefresh, 0, 0)
}
