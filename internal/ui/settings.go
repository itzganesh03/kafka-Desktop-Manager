package ui

import (
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/config"
)

// buildSettings builds the settings page.
func (u *AppUI) buildSettings() fyne.CanvasObject {
	pathEntry := widget.NewEntry()
	pathEntry.SetText(u.cfg.KafkaPath)
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
	defaultTopic := widget.NewEntry()
	defaultTopic.SetText(u.cfg.DefaultTopic)

	dataLog := widget.NewEntry()
	if u.cfg.DataLogDir != "" {
		dataLog.SetText(u.cfg.DataLogDir)
	} else {
		dataLog.SetText(config.DetectDataLogDir(u.cfg.KafkaPath))
	}
	dataLog.SetPlaceHolder(`C:\kafka\kafka-logs`)
	appLog := widget.NewEntry()
	if u.cfg.AppLogDir != "" {
		appLog.SetText(u.cfg.AppLogDir)
	} else {
		appLog.SetText(config.DefaultAppLogDir(u.cfg.KafkaPath))
	}
	appLog.SetPlaceHolder(`C:\kafka\logs`)
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

	autoZK := widget.NewCheck("Auto-start ZooKeeper on launch", nil)
	autoZK.SetChecked(u.cfg.AutoStartZK)
	autoKafka := widget.NewCheck("Auto-start Kafka broker on launch", nil)
	autoKafka.SetChecked(u.cfg.AutoStartKafka)

	themeSelect := widget.NewSelect([]string{"dark", "light"}, nil)
	if u.cfg.Theme == "" {
		themeSelect.SetSelected("dark")
	} else {
		themeSelect.SetSelected(u.cfg.Theme)
	}

	form := widget.NewForm(
		widget.NewFormItem("Kafka Path", container.NewBorder(nil, nil, nil, browse, pathEntry)),
		widget.NewFormItem("Bootstrap Server", bootstrap),
		widget.NewFormItem("ZooKeeper Port", zkPort),
		widget.NewFormItem("Default Topic", defaultTopic),
		widget.NewFormItem("Data Log Folder", container.NewBorder(nil, nil, nil, browseInto(dataLog), dataLog)),
		widget.NewFormItem("Log Folder", container.NewBorder(nil, nil, nil, browseInto(appLog), appLog)),
		widget.NewFormItem("Theme", themeSelect),
		widget.NewFormItem("", autoZK),
		widget.NewFormItem("", autoKafka),
	)

	saveBtn := widget.NewButtonWithIcon("Save Settings", theme.DocumentSaveIcon(), func() {
		path := strings.TrimSpace(pathEntry.Text)
		if missing := config.ValidateKafkaPath(path); len(missing) > 0 {
			u.errorDialog(errMissing(missing))
			return
		}
		themeChanged := u.cfg.Theme != themeSelect.Selected
		u.cfg.KafkaPath = path
		u.cfg.BootstrapServer = strings.TrimSpace(bootstrap.Text)
		u.cfg.ZookeeperPort = strings.TrimSpace(zkPort.Text)
		u.cfg.DefaultTopic = strings.TrimSpace(defaultTopic.Text)
		u.cfg.DataLogDir = strings.TrimSpace(dataLog.Text)
		u.cfg.AppLogDir = strings.TrimSpace(appLog.Text)
		u.cfg.AutoStartZK = autoZK.Checked
		u.cfg.AutoStartKafka = autoKafka.Checked
		u.cfg.Theme = themeSelect.Selected
		if err := u.cfg.Save(); err != nil {
			u.errorDialog(err)
			return
		}
		u.mgr.SetConfig(u.cfg)
		u.toast("Settings saved")
		if themeChanged {
			u.reloadShell()
		}
	})
	saveBtn.Importance = widget.HighImportance

	openKafka := widget.NewButtonWithIcon("Open Kafka Folder", theme.FolderOpenIcon(), func() {
		u.openInExplorer(u.cfg.KafkaPath)
	})
	openLogs := widget.NewButtonWithIcon("Open Log Folder", theme.FolderOpenIcon(), func() {
		u.openInExplorer(filepath.Join(u.cfg.KafkaPath, "logs"))
	})

	body := container.NewVBox(
		sectionTitle("Settings", theme.SettingsIcon()),
		widget.NewSeparator(),
		card("Kafka Configuration", form),
		container.NewHBox(saveBtn),
		widget.NewSeparator(),
		card("Shortcuts", container.NewHBox(openKafka, openLogs)),
	)
	return container.NewVScroll(container.NewPadded(body))
}

// openInExplorer opens a folder in Windows Explorer.
func (u *AppUI) openInExplorer(path string) {
	cmd := exec.Command("explorer", path)
	_ = cmd.Start()
}

func errMissing(missing []string) error {
	return &missingErr{missing}
}

type missingErr struct{ missing []string }

func (e *missingErr) Error() string {
	return "invalid Kafka path. Missing:\n- " + strings.Join(e.missing, "\n- ")
}
