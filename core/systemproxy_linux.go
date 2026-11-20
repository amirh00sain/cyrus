//go:build linux && !android

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Linux has no single system proxy. What is configured here, all best effort
// and all undone again on Disconnect:
//
//   - GNOME/GTK/Firefox/Chromium family: gsettings org.gnome.system.proxy
//   - KDE Plasma 5/6: kioslaverc via kwriteconfig5/6
//   - Everything else, notably Hyprland/Sway/i3 which have no settings
//     daemon: the proxy environment variables, pushed into the systemd user
//     manager, the D-Bus activation environment and (Hyprland) the compositor
//     so that every program started from now on inherits them, plus
//     ~/.config/opera-vpn/proxy.env that a shell can `source`.
//
// Programs that were already running keep their old environment; that is a
// property of Unix, not of this app. TUN mode has no such limitation.

func platformSupportsSystemProxy() bool { return true }

type proxyBackup struct {
	GSettingsMode string `json:"gsettings_mode,omitempty"`
	KDEProxyType  string `json:"kde_proxy_type,omitempty"`
}

func configDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

func backupPath() string { return filepath.Join(configDir(), "opera-vpn", "proxy-backup.json") }
func envFilePath() string { return filepath.Join(configDir(), "opera-vpn", "proxy.env") }
func environmentDPath() string {
	return filepath.Join(configDir(), "environment.d", "opera-vpn.conf")
}

func have(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

func isHyprland() bool {
	return strings.Contains(strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")), "hyprland") ||
		os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != ""
}

func gsettingsGet(schema, key string) (string, bool) {
	if !have("gsettings") {
		return "", false
	}
	out, err := exec.Command("gsettings", "get", schema, key).Output()
	if err != nil {
		return "", false // schema not installed
	}
	return strings.Trim(strings.TrimSpace(string(out)), "'"), true
}

func kdeTool() string {
	for _, t := range []string{"kwriteconfig6", "kwriteconfig5"} {
		if have(t) {
			return t
		}
	}
	return ""
}

func kdeConfigFile() string { return filepath.Join(configDir(), "kioslaverc") }

func readKDEProxyType() string {
	for _, t := range []string{"kreadconfig6", "kreadconfig5"} {
		if have(t) {
			out, err := exec.Command(t, "--file", kdeConfigFile(), "--group", "Proxy Settings", "--key", "ProxyType").Output()
			if err == nil {
				return strings.TrimSpace(string(out))
			}
		}
	}
	return ""
}

func enableSystemProxy(address string, bypassIR bool) error {
	host, port := splitAddress(address)
	url := "http://" + host + ":" + port

	// Keep the user's own values (only the first time, so a second Enable
	// never overwrites the original with our own proxy).
	if _, err := os.Stat(backupPath()); err != nil {
		b := proxyBackup{}
		if mode, ok := gsettingsGet("org.gnome.system.proxy", "mode"); ok {
			b.GSettingsMode = mode
		}
		b.KDEProxyType = readKDEProxyType()
		if data, err := json.Marshal(b); err == nil {
			_ = os.MkdirAll(filepath.Dir(backupPath()), 0o755)
			_ = os.WriteFile(backupPath(), data, 0o600)
		}
	}

	applied := 0

	if _, ok := gsettingsGet("org.gnome.system.proxy", "mode"); ok {
		if err := gnomeProxy(host, port, bypassIR); err != nil {
			logProxy("gsettings: %v", err)
		} else {
			applied++
			logProxy("GNOME/GTK settings updated")
		}
	}

	if t := kdeTool(); t != "" {
		if err := kdeProxy(t, host, port, bypassIR); err != nil {
			logProxy("%s: %v", t, err)
		} else {
			applied++
			logProxy("KDE settings updated")
		}
	}

	if err := writeProxyEnvFiles(url, bypassIR); err != nil {
		logProxy("environment files: %v", err)
	} else {
		applied++
	}
	pushSessionEnvironment(url, bypassIR)
	if isHyprland() {
		logProxy("Hyprland: new programs inherit the proxy; restart running ones, or use TUN mode for everything")
	}

	if applied == 0 {
		return fmt.Errorf("could not configure any proxy backend")
	}
	return nil
}

func disableSystemProxy() error {
	var b proxyBackup
	if data, err := os.ReadFile(backupPath()); err == nil {
		_ = json.Unmarshal(data, &b)
	}

	if _, ok := gsettingsGet("org.gnome.system.proxy", "mode"); ok {
		mode := b.GSettingsMode
		if mode == "" || mode == "manual" {
			mode = "none"
		}
		_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", mode).Run()
	}
	if t := kdeTool(); t != "" {
		typ := b.KDEProxyType
		if typ == "" || typ == "1" {
			typ = "0"
		}
		_ = exec.Command(t, "--file", kdeConfigFile(), "--group", "Proxy Settings", "--key", "ProxyType", typ).Run()
		notifyKDE()
	}

	_ = os.Remove(envFilePath())
	_ = os.Remove(environmentDPath())
	pushSessionEnvironment("", false)
	_ = os.Remove(backupPath())
	return nil
}

func recoverSystemProxy() {
	if _, err := os.Stat(backupPath()); err == nil {
		_ = disableSystemProxy()
	}
}

func gnomeProxy(host, port string, bypassIR bool) error {
	rows := [][3]string{
		{"org.gnome.system.proxy.http", "host", host},
		{"org.gnome.system.proxy.http", "port", port},
		{"org.gnome.system.proxy.https", "host", host},
		{"org.gnome.system.proxy.https", "port", port},
		{"org.gnome.system.proxy", "mode", "manual"},
	}
	for _, r := range rows {
		if out, err := exec.Command("gsettings", "set", r[0], r[1], r[2]).CombinedOutput(); err != nil {
			return fmt.Errorf("%s %s: %w: %s", r[0], r[1], err, strings.TrimSpace(string(out)))
		}
	}
	ignore := "'localhost', '127.0.0.0/8', '::1', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'"
	if bypassIR {
		ignore += ", '*.ir'"
	}
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "ignore-hosts", "["+ignore+"]").Run()
	return nil
}

func kdeProxy(tool, host, port string, bypassIR bool) error {
	file := kdeConfigFile()
	noProxy := "localhost,127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
	if bypassIR {
		noProxy += ",.ir"
	}
	rows := [][2]string{
		{"ProxyType", "1"},
		{"httpProxy", fmt.Sprintf("http://%s %s", host, port)},
		{"httpsProxy", fmt.Sprintf("http://%s %s", host, port)},
		{"NoProxyFor", noProxy},
	}
	for _, r := range rows {
		if out, err := exec.Command(tool, "--file", file, "--group", "Proxy Settings", "--key", r[0], r[1]).CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", r[0], err, strings.TrimSpace(string(out)))
		}
	}
	notifyKDE()
	return nil
}

func notifyKDE() {
	// Tell running KIO slaves to re-read kioslaverc.
	if have("dbus-send") {
		_ = exec.Command("dbus-send", "--session", "--type=signal", "/KIO/Scheduler",
			"org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:").Run()
	}
}


const noProxyBase = "localhost,127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"

func envPairs(url string, bypassIR bool) [][2]string {
	noProxyList := noProxyBase
	if bypassIR {
		noProxyList += ",.ir"
	}
	return [][2]string{
		{"http_proxy", url}, {"https_proxy", url},
		{"HTTP_PROXY", url}, {"HTTPS_PROXY", url},
		{"no_proxy", noProxyList}, {"NO_PROXY", noProxyList},
	}
}

func writeProxyEnvFiles(url string, bypassIR bool) error {
	if err := os.MkdirAll(filepath.Dir(envFilePath()), 0o755); err != nil {
		return err
	}
	var sh, envd strings.Builder
	for _, kv := range envPairs(url, bypassIR) {
		fmt.Fprintf(&sh, "export %s=%s\n", kv[0], kv[1])
		fmt.Fprintf(&envd, "%s=%s\n", kv[0], kv[1])
	}
	if err := os.WriteFile(envFilePath(), []byte(sh.String()), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(environmentDPath()), 0o755); err == nil {
		_ = os.WriteFile(environmentDPath(), []byte(envd.String()), 0o644)
	}
	return nil
}

// pushSessionEnvironment sets (url != "") or clears (url == "") the proxy
// variables for everything started later from this login session.
func pushSessionEnvironment(url string, bypassIR bool) {
	var assignments []string
	for _, kv := range envPairs(url, bypassIR) {
		v := kv[1]
		if url == "" {
			v = ""
		}
		assignments = append(assignments, kv[0]+"="+v)
	}
	if have("systemctl") {
		_ = exec.Command("systemctl", append([]string{"--user", "set-environment"}, assignments...)...).Run()
	}
	if have("dbus-update-activation-environment") {
		_ = exec.Command("dbus-update-activation-environment", append([]string{"--systemd"}, assignments...)...).Run()
	}
	if isHyprland() && have("hyprctl") {
		for _, kv := range envPairs(url, bypassIR) {
			v := kv[1]
			if url == "" {
				v = ""
			}
			_ = exec.Command("hyprctl", "keyword", "env", kv[0]+","+v).Run()
		}
	}
}
