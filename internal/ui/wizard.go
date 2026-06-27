package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/config"
)

// buildWizard constructs the first-run setup wizard.
func (u *AppUI) buildWizard() fyne.CanvasObject {
	heading := sectionTitle("Welcome to Kafka Desktop Manager", theme.HomeIcon())
	sub := widget.NewLabel("Let's set up your Kafka installation to get started.")

	pathEntry := widget.NewEntry()
	pathEntry.SetText(u.cfg.KafkaPath)
	pathEntry.SetPlaceHolder(`C:\kafka`)

	browse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		dlg := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			pathEntry.SetText(uri.Path())
		}, u.win)
		dlg.Show()
	})

	bootstrap := widget.NewEntry()
	bootstrap.SetText(u.cfg.BootstrapServer)
	zkPort := widget.NewEntry()
	zkPort.SetText(u.cfg.ZookeeperPort)

	// Folder selectors used by the "Metadata Delete" recovery action.
	dataLogEntry := widget.NewEntry()
	dataLogEntry.SetPlaceHolder(`C:\kafka\kafka-logs`)
	if u.cfg.DataLogDir != "" {
		dataLogEntry.SetText(u.cfg.DataLogDir)
	} else {
		dataLogEntry.SetText(config.DetectDataLogDir(u.cfg.KafkaPath))
	}
	appLogEntry := widget.NewEntry()
	appLogEntry.SetPlaceHolder(`C:\kafka\logs`)
	if u.cfg.AppLogDir != "" {
		appLogEntry.SetText(u.cfg.AppLogDir)
	} else {
		appLogEntry.SetText(config.DefaultAppLogDir(u.cfg.KafkaPath))
	}
	browseInto := func(target *widget.Entry) *widget.Button {
		return widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
			dlg := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
				if err != nil || uri == nil {
					return
				}
				target.SetText(uri.Path())
			}, u.win)
			dlg.Show()
		})
	}

	status := widget.NewRichTextWithText("")
	status.Wrapping = fyne.TextWrapWord

	validateBtn := widget.NewButtonWithIcon("Validate", theme.ConfirmIcon(), func() {
		path := strings.TrimSpace(pathEntry.Text)
		missing := config.ValidateKafkaPath(path)
		if len(missing) == 0 {
			// auto-fill the metadata folders from this install if still empty
			if strings.TrimSpace(dataLogEntry.Text) == "" {
				dataLogEntry.SetText(config.DetectDataLogDir(path))
			}
			if strings.TrimSpace(appLogEntry.Text) == "" {
				appLogEntry.SetText(config.DefaultAppLogDir(path))
			}
			status.ParseMarkdown("✅ **Valid Kafka installation found.** You can finish setup.")
		} else {
			status.ParseMarkdown("❌ **Missing required files:**\n\n- " + strings.Join(missing, "\n- "))
		}
	})

	var finish *widget.Button
	finish = widget.NewButtonWithIcon("Finish Setup", theme.ConfirmIcon(), func() {
		path := strings.TrimSpace(pathEntry.Text)
		missing := config.ValidateKafkaPath(path)
		if len(missing) > 0 {
			dialog.ShowError(fmt.Errorf("invalid Kafka path. Missing:\n- %s", strings.Join(missing, "\n- ")), u.win)
			return
		}
		u.cfg.KafkaPath = path
		u.cfg.BootstrapServer = strings.TrimSpace(bootstrap.Text)
		u.cfg.ZookeeperPort = strings.TrimSpace(zkPort.Text)
		u.cfg.DataLogDir = strings.TrimSpace(dataLogEntry.Text)
		u.cfg.AppLogDir = strings.TrimSpace(appLogEntry.Text)
		if err := u.cfg.Save(); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.mgr.SetConfig(u.cfg)
		u.toast("Setup complete!")
		u.win.SetContent(u.buildShell())
	})
	finish.Importance = widget.HighImportance

	form := widget.NewForm(
		widget.NewFormItem("Kafka Directory", container.NewBorder(nil, nil, nil, browse, pathEntry)),
		widget.NewFormItem("Bootstrap Server", bootstrap),
		widget.NewFormItem("ZooKeeper Port", zkPort),
		widget.NewFormItem("Data Log Folder", container.NewBorder(nil, nil, nil, browseInto(dataLogEntry), dataLogEntry)),
		widget.NewFormItem("Log Folder", container.NewBorder(nil, nil, nil, browseInto(appLogEntry), appLogEntry)),
	)

	hint := widget.NewRichTextFromMarkdown(
		"The folder must contain:\n\n" +
			"- `bin\\windows\\zookeeper-server-start.bat`\n" +
			"- `bin\\windows\\kafka-server-start.bat`\n" +
			"- `config\\zookeeper.properties`\n" +
			"- `config\\server.properties`\n\n" +
			"**Data Log Folder** = Kafka `log.dirs` (partition data, e.g. `kafka-logs`).\n" +
			"**Log Folder** = where `server.log`/`zookeeper` logs are written (`logs`).\n" +
			"These are cleared by *Metadata Delete* when the broker won't start.")

	body := container.NewVBox(
		heading, sub, widget.NewSeparator(),
		form,
		container.NewHBox(validateBtn, finish),
		widget.NewSeparator(),
		status,
		hint,
	)

	return container.NewPadded(container.NewVScroll(body))
}
