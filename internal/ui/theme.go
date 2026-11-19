package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// palette holds every color used by the "glass" look. Widgets get them via
// glassTheme; hand-drawn canvas objects read the active palette (pal) directly.
type palette struct {
	bgTop, bgBottom color.NRGBA // backdrop gradient
	glowA, glowB    color.NRGBA // soft radial light blobs over the backdrop

	glass       color.NRGBA // translucent panel fill
	glassBorder color.NRGBA // hairline panel border
	shadow      color.NRGBA // faux drop shadow under panels

	text, muted color.NRGBA

	primary   color.NRGBA
	button    color.NRGBA
	hover     color.NRGBA
	input     color.NRGBA
	inputLine color.NRGBA
	separator color.NRGBA
	selection color.NRGBA
	header    color.NRGBA
	overlay   color.NRGBA // dialogs & menus (opaque so they stay readable)
	disabled  color.NRGBA
	scrollBar color.NRGBA
}

var darkPalette = palette{
	bgTop:       color.NRGBA{R: 0x0a, G: 0x11, B: 0x24, A: 0xff},
	bgBottom:    color.NRGBA{R: 0x0f, G: 0x1e, B: 0x3d, A: 0xff},
	glowA:       color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0x40},
	glowB:       color.NRGBA{R: 0x7c, G: 0x3a, B: 0xed, A: 0x2a},
	glass:       color.NRGBA{R: 0x1e, G: 0x2a, B: 0x48, A: 0x99},
	glassBorder: color.NRGBA{R: 0x94, G: 0xa3, B: 0xb8, A: 0x30},
	shadow:      color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x40},
	text:        color.NRGBA{R: 0xe8, G: 0xee, B: 0xf8, A: 0xff},
	muted:       color.NRGBA{R: 0x94, G: 0xa3, B: 0xb8, A: 0xff},
	primary:     color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff},
	button:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x12},
	hover:       color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x1c},
	input:       color.NRGBA{R: 0x08, G: 0x10, B: 0x22, A: 0x99},
	inputLine:   color.NRGBA{R: 0x94, G: 0xa3, B: 0xb8, A: 0x38},
	separator:   color.NRGBA{R: 0x94, G: 0xa3, B: 0xb8, A: 0x24},
	selection:   color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0x55},
	header:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0a},
	overlay:     color.NRGBA{R: 0x13, G: 0x1d, B: 0x35, A: 0xff},
	disabled:    color.NRGBA{R: 0x64, G: 0x74, B: 0x8b, A: 0xff},
	scrollBar:   color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x30},
}

var lightPalette = palette{
	bgTop:       color.NRGBA{R: 0xf1, G: 0xf5, B: 0xfd, A: 0xff},
	bgBottom:    color.NRGBA{R: 0xe0, G: 0xe8, B: 0xf6, A: 0xff},
	glowA:       color.NRGBA{R: 0x93, G: 0xc5, B: 0xfd, A: 0x66},
	glowB:       color.NRGBA{R: 0xc4, G: 0xb5, B: 0xfd, A: 0x4d},
	glass:       color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xb8},
	glassBorder: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xe6},
	shadow:      color.NRGBA{R: 0x1e, G: 0x3a, B: 0x8a, A: 0x14},
	text:        color.NRGBA{R: 0x0f, G: 0x17, B: 0x2a, A: 0xff},
	muted:       color.NRGBA{R: 0x64, G: 0x74, B: 0x8b, A: 0xff},
	primary:     color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff},
	button:      color.NRGBA{R: 0x0f, G: 0x17, B: 0x2a, A: 0x0b},
	hover:       color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0x16},
	input:       color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xe6},
	inputLine:   color.NRGBA{R: 0xcb, G: 0xd5, B: 0xe1, A: 0xff},
	separator:   color.NRGBA{R: 0x0f, G: 0x17, B: 0x2a, A: 0x14},
	selection:   color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0x30},
	header:      color.NRGBA{R: 0xf1, G: 0xf5, B: 0xf9, A: 0xcc},
	overlay:     color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	disabled:    color.NRGBA{R: 0x94, G: 0xa3, B: 0xb8, A: 0xff},
	scrollBar:   color.NRGBA{R: 0x0f, G: 0x17, B: 0x2a, A: 0x30},
}

// pal is the active palette, swapped by applyTheme before the UI is built.
var pal = darkPalette

// Custom theme color names for accents, so icons can be tinted with
// theme.NewColoredResource.
const (
	colorTeal   fyne.ThemeColorName = "kdm-teal"
	colorBlue   fyne.ThemeColorName = "kdm-blue"
	colorPurple fyne.ThemeColorName = "kdm-purple"
	colorOrange fyne.ThemeColorName = "kdm-orange"
	colorGreen  fyne.ThemeColorName = "kdm-green"
	colorYellow fyne.ThemeColorName = "kdm-yellow"
	colorRed    fyne.ThemeColorName = "kdm-red"
)

var accentColors = map[fyne.ThemeColorName]color.NRGBA{
	colorTeal:   {R: 0x14, G: 0xb8, B: 0xa6, A: 0xff},
	colorBlue:   {R: 0x3b, G: 0x82, B: 0xf6, A: 0xff},
	colorPurple: {R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff},
	colorOrange: {R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff},
	colorGreen:  {R: 0x22, G: 0xc5, B: 0x5e, A: 0xff},
	colorYellow: {R: 0xea, G: 0xb3, B: 0x08, A: 0xff},
	colorRed:    {R: 0xef, G: 0x44, B: 0x44, A: 0xff},
}

// accent returns the color for an accent name.
func accent(name fyne.ThemeColorName) color.NRGBA { return accentColors[name] }

// withAlpha returns c with its alpha replaced.
func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// glassTheme is a translucent "glassmorphism" theme in dark or light flavor.
type glassTheme struct{ dark bool }

var _ fyne.Theme = glassTheme{}

func (t glassTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if c, ok := accentColors[name]; ok {
		return c
	}
	p := lightPalette
	variant := theme.VariantLight
	if t.dark {
		p, variant = darkPalette, theme.VariantDark
	}
	switch name {
	case theme.ColorNameBackground:
		return p.bgTop
	case theme.ColorNameForeground:
		return p.text
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError,
		theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return color.White
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return p.primary
	case theme.ColorNameFocus:
		return withAlpha(p.primary, 0xaa)
	case theme.ColorNameButton:
		return p.button
	case theme.ColorNameDisabledButton:
		return withAlpha(p.button, p.button.A/2)
	case theme.ColorNameHover, theme.ColorNamePressed:
		return p.hover
	case theme.ColorNameInputBackground:
		return p.input
	case theme.ColorNameInputBorder:
		return p.inputLine
	case theme.ColorNameDisabled:
		return p.disabled
	case theme.ColorNamePlaceHolder:
		return p.muted
	case theme.ColorNameSeparator:
		return p.separator
	case theme.ColorNameSelection:
		return p.selection
	case theme.ColorNameHeaderBackground:
		return p.header
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return p.overlay
	case theme.ColorNameShadow:
		return p.shadow
	case theme.ColorNameScrollBar:
		return p.scrollBar
	case theme.ColorNameError:
		return accentColors[colorRed]
	case theme.ColorNameSuccess:
		return accentColors[colorGreen]
	case theme.ColorNameWarning:
		return accentColors[colorOrange]
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (glassTheme) Font(style fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(style) }

func (glassTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (glassTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 9
	case theme.SizeNameText:
		return 13
	case theme.SizeNameInputRadius:
		return 9
	case theme.SizeNameSelectionRadius:
		return 8
	case theme.SizeNameScrollBarRadius:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return theme.DefaultTheme().Size(name)
}
