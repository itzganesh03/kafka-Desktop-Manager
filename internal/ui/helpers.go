package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// toast shows a transient notification at the bottom of the window.
func (u *AppUI) toast(msg string) {
	bg := canvas.NewRectangle(withAlpha(pal.primary, 0xf2))
	bg.CornerRadius = 12
	label := canvas.NewText(msg, color.White)
	label.TextStyle = fyne.TextStyle{Bold: true}
	content := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 10, 18, 18), label))

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
	c := canvas.NewCircle(red)
	if transitioning {
		c.FillColor = yellow
	} else if running {
		c.FillColor = green
	}
	c.Resize(fyne.NewSize(12, 12))
	return c
}

// card wraps content in a translucent glass panel with an optional title.
func card(title string, content fyne.CanvasObject) fyne.CanvasObject {
	var inner fyne.CanvasObject = content
	if title != "" {
		head := themedText(title, false)
		head.TextStyle = fyne.TextStyle{Bold: true}
		head.TextSize = 14
		inner = container.NewBorder(
			container.New(layout.NewCustomPaddedLayout(2, 8, 4, 0), head), nil, nil, nil, content)
	}
	return glassPanel(inner)
}

// sectionTitle returns a large bold page heading led by an accent icon tile.
func sectionTitle(text string, icon fyne.Resource) fyne.CanvasObject {
	t := themedText(text, false)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 22
	if icon != nil {
		tile := newIconTile(icon, colorBlue, 34)
		return container.NewHBox(vCentered(tile.root), vCentered(t))
	}
	return container.NewHBox(vCentered(t))
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
