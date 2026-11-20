package ui

import (
	"fmt"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
)

// Stroke icons (24x24 grid, Lucide-style geometry), recoloured on demand.
// Paths use explicit separators so every SVG parser reads them the same way.
var iconBody = map[string]string{
	"power":      `<path d="M18.36 6.64 A9 9 0 1 1 5.64 6.64"/><path d="M12 2 L12 12"/>`,
	"cloud":      `<path d="M17.5 19 H9 a7 7 0 1 1 6.71 -9 h1.79 a4.5 4.5 0 1 1 0 9 Z"/>`,
	"cloud-off":  `<path d="M2 2 L22 22"/><path d="M5.782 5.782 A7 7 0 0 0 9 19 H17.5 a4.5 4.5 0 0 0 1.307 -0.193"/><path d="M21.532 16.5 A4.5 4.5 0 0 0 17.5 10 h-1.79 A7.008 7.008 0 0 0 10 5.07"/>`,
	"shield":     `<path d="M20 13 c0 5 -3.5 7.5 -7.66 8.95 a1 1 0 0 1 -0.67 -0.01 C7.5 20.5 4 18 4 13 V6 a1 1 0 0 1 1 -1 c2 0 4.5 -1.2 6.24 -2.72 a1.17 1.17 0 0 1 1.52 0 C14.51 3.81 17 5 19 5 a1 1 0 0 1 1 1 z"/><path d="M9 12 l2 2 l4 -4"/>`,
	"loader":     `<path d="M21 12 a9 9 0 1 1 -6.219 -8.56"/>`,
	"alert":      `<circle cx="12" cy="12" r="10"/><path d="M12 8 L12 12"/><path d="M12 16 L12.01 16"/>`,
	"home":       `<path d="M15 21 v-8 a1 1 0 0 0 -1 -1 h-4 a1 1 0 0 0 -1 1 v8"/><path d="M3 10 a2 2 0 0 1 0.709 -1.528 l7 -5.999 a2 2 0 0 1 2.582 0 l7 5.999 A2 2 0 0 1 21 10 v9 a2 2 0 0 1 -2 2 H5 a2 2 0 0 1 -2 -2 z"/>`,
	"info":       `<circle cx="12" cy="12" r="10"/><path d="M12 16 L12 12"/><path d="M12 8 L12.01 8"/>`,
	"more":       `<circle cx="5" cy="12" r="1.5"/><circle cx="12" cy="12" r="1.5"/><circle cx="19" cy="12" r="1.5"/>`,
	"chevron":    `<path d="M9 18 l6 -6 l-6 -6"/>`,
	"chevs-down": `<path d="M7 6 l5 5 l5 -5"/><path d="M7 13 l5 5 l5 -5"/>`,
	"chevs-up":   `<path d="M17 11 l-5 -5 l-5 5"/><path d="M17 18 l-5 -5 l-5 5"/>`,
	"file":       `<path d="M15 2 H6 a2 2 0 0 0 -2 2 v16 a2 2 0 0 0 2 2 h12 a2 2 0 0 0 2 -2 V7 z"/><path d="M14 2 v4 a2 2 0 0 0 2 2 h4"/>`,
}

var (
	iconCacheMu sync.Mutex
	iconCache   = map[string]fyne.Resource{}
)

func hex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// svgIcon returns the named icon drawn in colour c.
func svgIcon(name string, c color.Color) fyne.Resource {
	h := hex(c)
	key := name + "-" + h[1:] + ".svg"
	iconCacheMu.Lock()
	defer iconCacheMu.Unlock()
	if r, ok := iconCache[key]; ok {
		return r
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" `+
		`fill="none" stroke="%s" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">%s</svg>`,
		h, iconBody[name])
	r := fyne.NewStaticResource(key, []byte(svg))
	iconCache[key] = r
	return r
}
