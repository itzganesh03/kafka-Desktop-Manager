package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/kafka"
)

// buildHistory builds the command-history page.
func (u *AppUI) buildHistory() fyne.CanvasObject {
	var records []kafka.CommandRecord

	list := widget.NewList(
		func() int { return len(records) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				container.NewHBox(
					widget.NewButtonWithIcon("", theme.ContentCopyIcon(), nil),
					widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil),
				),
				widget.NewLabel("cmd"))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container)
			rec := records[i]
			row.Objects[0].(*widget.Label).SetText(rec.When.Format("15:04:05") + "  " + rec.Display)
			btns := row.Objects[1].(*fyne.Container)
			btns.Objects[0].(*widget.Button).OnTapped = func() {
				u.fyneApp.Clipboard().SetContent(rec.Display)
				u.toast("Command copied")
			}
			btns.Objects[1].(*widget.Button).OnTapped = func() {
				u.rerun(rec)
			}
		},
	)

	reload := func() {
		records = u.mgr.History().List()
		list.Refresh()
	}
	u.mgr.History().Subscribe(func() { fyne.Do(reload) })

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() { reload() })
	header := container.NewVBox(
		container.NewBorder(nil, nil, sectionTitle("Command History", theme.HistoryIcon()), refreshBtn),
		widget.NewLabel("Every executed Kafka command is recorded here. Re-run with one click."),
		widget.NewSeparator(),
	)

	reload()
	return container.NewBorder(header, nil, nil, nil, list)
}

// rerun re-executes a recorded command (admin tools only; services are skipped).
func (u *AppUI) rerun(rec kafka.CommandRecord) {
	u.confirm("Re-run command", rec.Display, func() {
		go func() {
			out, err := u.mgr.RunRecorded(rec)
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(err)
					return
				}
				u.toast("Command executed")
				if out != "" {
					u.showOutput(rec.Display, out)
				}
			})
		}()
	})
}

// showOutput displays command output in a scrollable dialog.
func (u *AppUI) showOutput(title, out string) {
	entry := widget.NewMultiLineEntry()
	entry.SetText(out)
	entry.Wrapping = fyne.TextWrapOff
	sc := container.NewScroll(entry)
	sc.SetMinSize(fyne.NewSize(640, 360))
	d := dialog.NewCustom(title, "Close", sc, u.win)
	d.Resize(fyne.NewSize(700, 440))
	d.Show()
}
