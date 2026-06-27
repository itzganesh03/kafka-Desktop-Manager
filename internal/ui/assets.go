package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// appIcon is the embedded application logo used for the window, taskbar and
// splash screen.
var appIcon fyne.Resource = fyne.NewStaticResource("icon.png", iconPNG)
