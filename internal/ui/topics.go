package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/kafka"
)

// buildTopics builds the topic management page.
func (u *AppUI) buildTopics() fyne.CanvasObject {
	var all []kafka.TopicInfo
	var shown []kafka.TopicInfo
	var selected string

	search := widget.NewEntry()
	search.SetPlaceHolder("Search topics...")

	table := widget.NewTable(
		func() (int, int) { return len(shown) + 1, 3 },
		func() fyne.CanvasObject { return widget.NewLabel("cell") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			if id.Row == 0 {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.SetText([]string{"Topic", "Partitions", "Replication"}[id.Col])
				return
			}
			lbl.TextStyle = fyne.TextStyle{}
			ti := shown[id.Row-1]
			switch id.Col {
			case 0:
				lbl.SetText(ti.Name)
			case 1:
				lbl.SetText(strconv.Itoa(ti.Partitions))
			case 2:
				lbl.SetText(strconv.Itoa(ti.ReplicationFactor))
			}
		},
	)
	table.SetColumnWidth(0, 420)
	table.SetColumnWidth(1, 120)
	table.SetColumnWidth(2, 120)
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 1 && id.Row-1 < len(shown) {
			selected = shown[id.Row-1].Name
		}
	}

	applyFilter := func() {
		q := strings.ToLower(strings.TrimSpace(search.Text))
		shown = shown[:0]
		for _, ti := range all {
			if q == "" || strings.Contains(strings.ToLower(ti.Name), q) {
				shown = append(shown, ti)
			}
		}
		table.Refresh()
	}
	search.OnChanged = func(string) { applyFilter() }

	countLbl := widget.NewLabel("")
	reload := func() {
		go func() {
			infos, err := u.mgr.ListTopicInfos()
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(err)
					return
				}
				all = infos
				countLbl.SetText(fmt.Sprintf("%d topics", len(all)))
				applyFilter()
			})
		}()
	}

	createBtn := widget.NewButtonWithIcon("Create", theme.ContentAddIcon(), func() {
		u.showCreateTopic(reload)
	})
	describeBtn := widget.NewButtonWithIcon("Describe", theme.InfoIcon(), func() {
		if selected == "" {
			u.toast("Select a topic first")
			return
		}
		u.showDescribeTopic(selected)
	})
	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if selected == "" {
			u.toast("Select a topic first")
			return
		}
		u.confirm("Delete topic", "Delete topic '"+selected+"'?", func() {
			go func() {
				_, err := u.mgr.DeleteTopic(selected)
				fyne.Do(func() {
					if err != nil {
						u.errorDialog(err)
						return
					}
					u.toast("Deleted " + selected)
					reload()
				})
			}()
		})
	})
	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() { reload() })
	utilBtn := widget.NewButtonWithIcon("Utilities", theme.SettingsIcon(), func() {
		if selected == "" {
			u.toast("Select a topic first")
			return
		}
		u.showTopicUtilities(selected, reload)
	})

	toolbar := container.NewHBox(createBtn, describeBtn, deleteBtn, utilBtn, refreshBtn)
	header := container.NewVBox(
		container.NewBorder(nil, nil, sectionTitle("Topics", theme.ListIcon()), countLbl),
		toolbar,
		search,
		widget.NewSeparator(),
	)

	page := container.NewBorder(header, nil, nil, nil, table)
	reload()
	return page
}

// showCreateTopic shows the create-topic dialog.
func (u *AppUI) showCreateTopic(onDone func()) {
	name := widget.NewEntry()
	name.SetPlaceHolder("my-topic")
	parts := widget.NewEntry()
	parts.SetText("1")
	rf := widget.NewEntry()
	rf.SetText("1")

	form := []*widget.FormItem{
		widget.NewFormItem("Topic Name", name),
		widget.NewFormItem("Partitions", parts),
		widget.NewFormItem("Replication Factor", rf),
	}
	d := dialog.NewForm("Create Topic", "Create", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		p, _ := strconv.Atoi(parts.Text)
		r, _ := strconv.Atoi(rf.Text)
		go func() {
			_, err := u.mgr.CreateTopic(strings.TrimSpace(name.Text), p, r)
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(err)
					return
				}
				u.toast("Created " + name.Text)
				onDone()
			})
		}()
	}, u.win)
	d.Resize(fyne.NewSize(420, 240))
	d.Show()
}

// showDescribeTopic shows full topic details.
func (u *AppUI) showDescribeTopic(topic string) {
	go func() {
		d, err := u.mgr.DescribeTopic(topic)
		fyne.Do(func() {
			if err != nil {
				u.errorDialog(err)
				return
			}
			summary := widget.NewRichTextFromMarkdown(fmt.Sprintf(
				"**Topic:** %s\n\n**Partitions:** %d  **Replication:** %d\n\n**Configs:** %s",
				d.Name, d.Partitions, d.ReplicationFactor, valueOrDash(d.Configs)))

			rows := d.PartitionRows
			tbl := widget.NewTable(
				func() (int, int) { return len(rows) + 1, 4 },
				func() fyne.CanvasObject { return widget.NewLabel("cell") },
				func(id widget.TableCellID, o fyne.CanvasObject) {
					lbl := o.(*widget.Label)
					if id.Row == 0 {
						lbl.TextStyle = fyne.TextStyle{Bold: true}
						lbl.SetText([]string{"Partition", "Leader", "Replicas", "ISR"}[id.Col])
						return
					}
					lbl.TextStyle = fyne.TextStyle{}
					r := rows[id.Row-1]
					switch id.Col {
					case 0:
						lbl.SetText(strconv.Itoa(r.Partition))
					case 1:
						lbl.SetText(r.Leader)
					case 2:
						lbl.SetText(r.Replicas)
					case 3:
						lbl.SetText(r.ISR)
					}
				},
			)
			for i, w := range []float32{90, 90, 140, 140} {
				tbl.SetColumnWidth(i, w)
			}
			content := container.NewBorder(summary, nil, nil, nil, tbl)
			dlg := dialog.NewCustom("Topic Details", "Close", content, u.win)
			dlg.Resize(fyne.NewSize(640, 460))
			dlg.Show()
		})
	}()
}

// showTopicUtilities shows the per-topic utility actions.
func (u *AppUI) showTopicUtilities(topic string, onDone func()) {
	run := func(label string, fn func() (string, error)) {
		go func() {
			_, err := fn()
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(err)
					return
				}
				u.toast(label + " done")
				onDone()
			})
		}()
	}

	purge := widget.NewButtonWithIcon("Purge Topic", theme.ContentClearIcon(), func() {
		u.confirm("Purge", "Purge all messages from '"+topic+"'? (delete + recreate)", func() {
			run("Purge", func() (string, error) { return u.mgr.PurgeTopic(topic) })
		})
	})
	recreate := widget.NewButtonWithIcon("Recreate Topic", theme.ViewRefreshIcon(), func() {
		u.confirm("Recreate", "Delete and recreate '"+topic+"'?", func() {
			run("Recreate", func() (string, error) { return u.mgr.RecreateTopic(topic) })
		})
	})
	del := widget.NewButtonWithIcon("Delete Topic", theme.DeleteIcon(), func() {
		u.confirm("Delete", "Delete '"+topic+"'?", func() {
			run("Delete", func() (string, error) { return u.mgr.DeleteTopic(topic) })
		})
	})
	sample := widget.NewButtonWithIcon("Create Sample Messages", theme.MailSendIcon(), func() {
		run("Sample messages", func() (string, error) { return u.mgr.CreateSampleMessages(topic, 10) })
	})
	resetEarliest := widget.NewButtonWithIcon("Reset Offsets → Earliest", theme.MediaSkipPreviousIcon(), func() {
		grp := widget.NewEntry()
		dialog.ShowForm("Reset Offsets to Earliest", "Reset", "Cancel",
			[]*widget.FormItem{widget.NewFormItem("Consumer Group", grp)}, func(ok bool) {
				if !ok || strings.TrimSpace(grp.Text) == "" {
					return
				}
				run("Reset offsets", func() (string, error) { return u.mgr.ResetOffsetsToEarliest(grp.Text, topic) })
			}, u.win)
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("Topic: "+topic, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		purge, recreate, del, sample, resetEarliest,
	)
	dlg := dialog.NewCustom("Topic Utilities", "Close", content, u.win)
	dlg.Resize(fyne.NewSize(380, 360))
	dlg.Show()
}

func valueOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
