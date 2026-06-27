package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// darkTheme is a modern dark theme with a teal/blue accent, inspired by
// Docker Desktop / MongoDB Compass.
type darkTheme struct{}

var _ fyne.Theme = (*darkTheme)(nil)

func (darkTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x12, G: 0x16, B: 0x1c, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xe6, G: 0xed, B: 0xf3, A: 0xff}
	case theme.ColorNameButton, theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x1d, G: 0x23, B: 0x2c, A: 0xff}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x55, G: 0x5d, B: 0x66, A: 0xff}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x2a, G: 0x32, B: 0x3d, A: 0xff}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x2e, G: 0xb8, B: 0xa6, A: 0xff} // teal accent
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0x2e, G: 0xb8, B: 0xa6, A: 0xaa}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x88}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x2a, G: 0x32, B: 0x3d, A: 0xff}
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x17, G: 0x1c, B: 0x24, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x7a, G: 0x84, B: 0x90, A: 0xff}
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (darkTheme) Font(style fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(style) }

func (darkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (darkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 13
	}
	return theme.DefaultTheme().Size(name)
}

// lightTheme is the default light variant for users who prefer it.
type lightTheme struct{}

var _ fyne.Theme = (*lightTheme)(nil)

func (lightTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 0x16, G: 0x9b, B: 0x8a, A: 0xff}
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}
func (lightTheme) Font(style fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(style) }
func (lightTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}
func (lightTheme) Size(name fyne.ThemeSizeName) float32 { return theme.DefaultTheme().Size(name) }
