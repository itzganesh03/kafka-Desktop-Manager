package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/kafka"
)

// buildGroups builds the consumer-groups page.
func (u *AppUI) buildGroups() fyne.CanvasObject {
	var groups []string

	list := widget.NewList(
		func() int { return len(groups) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.AccountIcon()), widget.NewLabel("group"))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*fyne.Container).Objects[1].(*widget.Label).SetText(groups[i])
		},
	)

	detail := container.NewStack(widget.NewLabel("Select a consumer group to see offsets and lag."))

	reload := func() {
		go func() {
			names, err := u.mgr.ListGroups()
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(err)
					return
				}
				groups = names
				list.Refresh()
			})
		}()
	}

	list.OnSelected = func(i widget.ListItemID) {
		group := groups[i]
		go func() {
			rows, err := u.mgr.DescribeGroup(group)
			state, members := u.mgr.GroupState(group)
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(err)
					return
				}
				detail.Objects = []fyne.CanvasObject{u.groupDetail(group, state, members, rows)}
				detail.Refresh()
			})
		}()
	}

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() { reload() })
	header := container.NewVBox(
		container.NewBorder(nil, nil, sectionTitle("Consumer Groups", theme.AccountIcon()), refreshBtn),
		widget.NewSeparator(),
	)

	left := card("Groups", list)
	split := container.NewHSplit(left, container.NewPadded(detail))
	split.SetOffset(0.28)

	page := container.NewBorder(header, nil, nil, nil, split)
	reload()
	return page
}

// groupDetail renders the offsets/lag table for a single group.
func (u *AppUI) groupDetail(group, state string, members int, rows []kafka.GroupOffset) fyne.CanvasObject {
	summary := widget.NewRichTextFromMarkdown(fmt.Sprintf(
		"### %s\n\n**State:** %s   **Members:** %d   **Assignments:** %d",
		group, state, members, len(rows)))

	tbl := widget.NewTable(
		func() (int, int) { return len(rows) + 1, 5 },
		func() fyne.CanvasObject {
			return container.NewStack(widget.NewLabel("cell"))
		},
		func(id widget.TableCellID, o fyne.CanvasObject) {
			wrap := o.(*fyne.Container)
			lbl := wrap.Objects[0].(*widget.Label)
			if id.Row == 0 {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.SetText([]string{"Topic", "Partition", "Current", "Log End", "Lag"}[id.Col])
				lbl.Importance = widget.MediumImportance
				return
			}
			r := rows[id.Row-1]
			lbl.TextStyle = fyne.TextStyle{}
			lbl.Importance = widget.MediumImportance
			switch id.Col {
			case 0:
				lbl.SetText(r.Topic)
			case 1:
				lbl.SetText(strconv.Itoa(r.Partition))
			case 2:
				lbl.SetText(r.CurrentOffset)
			case 3:
				lbl.SetText(r.LogEndOffset)
			case 4:
				lbl.SetText(strconv.FormatInt(r.Lag, 10))
				if r.Lag > 0 {
					lbl.Importance = widget.DangerImportance // red lag
				} else {
					lbl.Importance = widget.SuccessImportance
				}
			}
		},
	)
	for i, w := range []float32{280, 90, 110, 110, 90} {
		tbl.SetColumnWidth(i, w)
	}

	return container.NewBorder(summary, nil, nil, nil, tbl)
}
