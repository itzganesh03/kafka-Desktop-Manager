package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// toast shows a transient notification at the bottom of the window.
func (u *AppUI) toast(msg string) {
	bg := canvas.NewRectangle(color.NRGBA{R: 0x2e, G: 0xb8, B: 0xa6, A: 0xff})
	bg.CornerRadius = 6
	label := widget.NewLabelWithStyle(msg, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content := container.NewStack(bg, container.NewPadded(label))

	pop := widget.NewPopUp(content, u.win.Canvas())
	cw := u.win.Canvas().Size()
	ps := content.MinSize()
	pop.Move(fyne.NewPos((cw.Width-ps.Width)/2, cw.Height-ps.Height-40))
	pop.Show()
	go func() {
		time.Sleep(2500 * time.Millisecond)
		fyne.Do(pop.Hide)
	}()
}

// errorDialog shows an error message.
func (u *AppUI) errorDialog(err error) {
	if err == nil {
		return
	}
	dialog.ShowError(err, u.win)
}

// notify sends an OS-level (Windows) notification.
func (u *AppUI) notify(title, body string) {
	u.fyneApp.SendNotification(fyne.NewNotification(title, body))
}

// statusDot returns a colored circle reflecting a running/stopped/transition
// state, plus a label.
func statusDot(running, transitioning bool) *canvas.Circle {
	c := canvas.NewCircle(color.NRGBA{R: 0xe0, G: 0x4f, B: 0x4f, A: 0xff}) // red
	if transitioning {
		c.FillColor = color.NRGBA{R: 0xe6, G: 0xb0, B: 0x3a, A: 0xff} // yellow
	} else if running {
		c.FillColor = color.NRGBA{R: 0x3f, G: 0xb9, B: 0x50, A: 0xff} // green
	}
	c.Resize(fyne.NewSize(12, 12))
	return c
}

// card wraps content in a rounded "card" with a title.
func card(title string, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0x1a, G: 0x20, B: 0x29, A: 0xff})
	bg.CornerRadius = 10
	bg.StrokeColor = color.NRGBA{R: 0x2a, G: 0x32, B: 0x3d, A: 0xff}
	bg.StrokeWidth = 1

	var inner fyne.CanvasObject = content
	if title != "" {
		head := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		inner = container.NewBorder(head, nil, nil, nil, content)
	}
	return container.NewStack(bg, container.NewPadded(inner))
}

// sectionTitle returns a large bold heading with an icon.
func sectionTitle(text string, icon fyne.Resource) fyne.CanvasObject {
	t := canvas.NewText(text, theme.DefaultTheme().Color(theme.ColorNameForeground, theme.VariantDark))
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 20
	if icon != nil {
		return container.NewHBox(widget.NewIcon(icon), t)
	}
	return container.NewHBox(t)
}

// confirm shows a yes/no confirmation dialog and calls onYes if confirmed.
func (u *AppUI) confirm(title, message string, onYes func()) {
	dialog.ShowConfirm(title, message, func(ok bool) {
		if ok {
			onYes()
		}
	}, u.win)
}

// exportText opens a save dialog and writes content to the chosen file.
func (u *AppUI) exportText(title, content string) {
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer w.Close()
		if _, werr := w.Write([]byte(content)); werr != nil {
			u.errorDialog(werr)
			return
		}
		u.toast("Saved to " + w.URI().Name())
	}, u.win)
	save.Show()
	_ = title
}
