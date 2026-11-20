package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette of the dark "terminal" look: near-black surfaces, mint accent,
// amber for measurements, teal/cyan for the selected navigation item.
var (
	colorBackground = color.NRGBA{R: 0x0A, G: 0x0D, B: 0x0C, A: 0xFF}
	colorCard       = color.NRGBA{R: 0x10, G: 0x15, B: 0x14, A: 0xFF}
	colorNav        = color.NRGBA{R: 0x0F, G: 0x14, B: 0x13, A: 0xFF}
	colorBorder     = color.NRGBA{R: 0x22, G: 0x2B, B: 0x28, A: 0xFF}
	colorForeground = color.NRGBA{R: 0xE6, G: 0xED, B: 0xEA, A: 0xFF}
	colorMuted      = color.NRGBA{R: 0x8A, G: 0x9A, B: 0x94, A: 0xFF}
	colorDim        = color.NRGBA{R: 0x4F, G: 0x5F, B: 0x59, A: 0xFF}
	colorGreen      = color.NRGBA{R: 0x5B, G: 0xE4, B: 0x9B, A: 0xFF}
	colorGreenFill  = color.NRGBA{R: 0x0E, G: 0x20, B: 0x18, A: 0xFF}
	colorAmber      = color.NRGBA{R: 0xE6, G: 0xB4, B: 0x50, A: 0xFF}
	colorRed        = color.NRGBA{R: 0xF0, G: 0x61, B: 0x6D, A: 0xFF}
	colorPill       = color.NRGBA{R: 0x0B, G: 0x4F, B: 0x5A, A: 0xFF}
	colorCyan       = color.NRGBA{R: 0x5C, G: 0xE1, B: 0xE6, A: 0xFF}
	colorIconBg     = color.NRGBA{R: 0x1A, G: 0x22, B: 0x20, A: 0xFF}
	colorTransparent = color.NRGBA{}
)

type appTheme struct{ fyne.Theme }

func newAppTheme() fyne.Theme { return appTheme{Theme: theme.DefaultTheme()} }

func (t appTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground, theme.ColorNameHeaderBackground,
		theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return colorBackground
	case theme.ColorNameForeground:
		return colorForeground
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameSuccess:
		return colorGreen
	case theme.ColorNameButton, theme.ColorNameInputBackground:
		return colorCard
	case theme.ColorNameDisabledButton, theme.ColorNameDisabled:
		return colorDim
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x5B, G: 0xE4, B: 0x9B, A: 0x14}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0x5B, G: 0xE4, B: 0x9B, A: 0x2A}
	case theme.ColorNameSeparator, theme.ColorNameInputBorder:
		return colorBorder
	case theme.ColorNamePlaceHolder:
		return colorMuted
	case theme.ColorNameSelection:
		return colorPill
	case theme.ColorNameError:
		return colorRed
	case theme.ColorNameWarning:
		return colorAmber
	case theme.ColorNameShadow:
		return colorTransparent
	case theme.ColorNameScrollBar:
		return colorBorder
	}
	// Always the dark variant, whatever the OS preference is.
	return t.Theme.Color(name, theme.VariantDark)
}

func (t appTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 14
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 3
	}
	return t.Theme.Size(name)
}
