# Opera VPN UI

A cross-platform GUI for the `opera-proxy` engine, built with **Go + Fyne**.
One codebase produces a universal Android APK, a Linux AppImage and Windows
x64 / x86 executables.

## Features

- Central power circle: **green = connected**, **red = disconnected** (amber while connecting).
- Routing modes:
  - **Proxy** – local HTTP/SOCKS proxy
  - **System proxy** – sets the OS-wide proxy (hidden on Android)
  - **TUN** – routes all traffic (VpnService bridge on Android, `tun2socks` on desktop)
  - Android intentionally shows only **TUN** and **Proxy**.
- Live connection stats under the circle: **download**, **upload**, **ping** and session totals.
- **Servers** tab with a ready-made list (no live search) and per-server **Ping** (real TCP connect to discovered IPs).
- The server list is embedded from the engine's own **discovery** output (`servers/discovered.csv`).
- Tabs: Overview, Servers, Settings, Logs, About.
- **No plan, no registry, no login** — anonymous registration is an internal engine detail only.

## Server list

Generated with the engine itself:

```
go build -o opera-cli ./cmd/opera-proxy-cli
./opera-cli -list-proxies-all-out discovered.csv
```

The SurfEasy API currently exposes three regions (EU / AM / AS) with nine
endpoints. They are stored in `servers/discovered.csv` and embedded at build
time via `go:embed`. The UI displays them as selectable locations. To refresh,
re-run the command above and replace the CSV, then update `servers/catalog.go`
if the endpoints changed.

## Layout

```
main.go                 GUI entry point
ui/                     Fyne views and custom widgets
core/                   controller, state, speed monitor, system proxy, TUN
servers/                embedded discovery CSV + catalog + ping
engine/                 original opera-proxy engine (reusable Start API)
dialer, handler, ...    original engine support packages
cmd/opera-proxy-cli/    original headless CLI (optional)
.github/workflows/build.yml
```

## Build locally

```bash
go build -o opera-vpn .
go run .
```

## CI outputs

`.github/workflows/build.yml` runs on tags (`v*`) and manually. It uses
[fyne-cross](https://github.com/fyne-io/fyne-cross) (Docker) for cross builds:

| Job | Output |
| --- | --- |
| android | `OperaVPN-universal.apk` (arm, arm64, x86, x86_64 in one APK) |
| linux | `OperaVPN-x86_64.AppImage` |
| windows | `OperaVPN-windows-x64.exe`, `OperaVPN-windows-x86.exe` |

Artifacts are uploaded to the workflow run and attached to the GitHub release for tags.

## Android

The Android build uses a portrait layout: a compact icon-only navigation rail
keeps the tabs in a sidebar, while cards and speed stats stack vertically in a
single column. `container.NewBorder` is used instead of a draggable splitter.

Routing modes on Android are only **Proxy** and **TUN** (System proxy is hidden).

- **Proxy** starts the local HTTP proxy in-process and shows the live connection state.
- **TUN** requires a native `VpnService` bridge. Register one with
  `core.SetAndroidVPNBridge(...)` from the Android entry point; until then the
  UI reports that the VPN service bridge is unavailable instead of crashing.
  All engine workers are wrapped with panic recovery (`core.SafeGo`, `ui.uiDo`)
  and the engine skips POSIX signal handling on Android.

## Notes

- TUN mode on desktop needs `tun2socks` installed and root privileges.
- The original CLI argument list is unchanged and still available via `cmd/opera-proxy-cli`.

## Routing modes

| Mode   | Windows | Linux (incl. Hyprland) | Android |
|--------|---------|------------------------|---------|
| Proxy  | local HTTP proxy `127.0.0.1:18080` | same | same |
| System | WinINET/Internet Settings (restored on disconnect, also after a crash) | GNOME/GTK `gsettings`, KDE `kioslaverc`, proxy environment variables (systemd user env, D-Bus activation env, `hyprctl keyword env` on Hyprland, `~/.config/opera-vpn/proxy.env`) | – |
| TUN    | Wintun adapter via an elevated helper (UAC prompt) | `pkexec` helper creates the TUN device and routes | `VpnService` (system VPN consent dialog) |

TUN runs the engine in SOCKS mode and forwards packets with
[tun2socks](https://github.com/xjasonlyu/tun2socks). UDP/53 is answered through
DNS-over-HTTPS over the same upstream proxy; other UDP is dropped (browsers fall
back from QUIC to TCP). The upstream Opera servers bypass the tunnel (Linux and
Windows: host routes; Android: the app is excluded from its own VPN).

On Hyprland, programs that are already running keep their old environment, so
the system proxy only reaches newly started apps; use TUN to cover everything.
TUN needs polkit (`pkexec`) and an authentication agent such as
`hyprpolkitagent`.

## Building

```sh
go mod tidy                 # pulls tun2socks and friends (go.sum is regenerated)
bash scripts/fetch-wintun.sh   # Windows builds only: embeds wintun.dll
```

Android: `fyne-cross android ...` builds the UI; `scripts/patch-apk.sh in.apk out.apk`
then adds the Java `VpnService` (`android/java`), declares it in the manifest and
re-signs the APK. The GitHub workflow does all of this.

## Iranian sites (.ir)

"Connect .ir sites directly" (More tab, on by default) keeps `*.ir` off the VPN:

- Proxy, System proxy and Android TUN: the engine connects `*.ir` hosts directly
  (in TUN the DNS answers for `.ir` names are remembered, since apps only hand over IPs).
  The system proxy bypass lists also get `*.ir`.
- Windows/Linux TUN: the engine cannot dial outside its own tunnel, so every
  Iranian IPv4 range (`core/irdata/ir-cidrs.txt`, fetched by `scripts/fetch-ir.sh`
  at build time) is routed via the normal gateway instead.
