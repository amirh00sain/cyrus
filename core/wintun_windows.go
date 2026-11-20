//go:build windows

package core

import "embed"

// wintun/ holds wintun_amd64.dll and wintun_386.dll. They are not committed;
// the release workflow (and scripts/fetch-wintun.sh) download them from
// https://www.wintun.net/ before building. Without them the app still builds
// and asks the user to place wintun.dll next to the executable.
//
//go:embed all:wintun
var wintunFS embed.FS
