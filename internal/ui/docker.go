package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/config"
	"kafkadesktop/internal/kafka"
)

const (
	modeLocalLabel  = "Local installation (.bat scripts)"
	modeDockerLabel = "Docker (KRaft container)"
)

// dockerSettingsCard builds the "Kafka Mode" card on the Settings page: Docker
// detection, the Local/Docker mode switch and the "Install Kafka via Docker"
// button. Docker options are only enabled when Docker is detected.
func (u *AppUI) dockerSettingsCard() fyne.CanvasObject {
	status := widget.NewLabel("Checking for Docker…")
	status.Wrapping = fyne.TextWrapWord

	var (
		mu     sync.Mutex
		info   kafka.DockerInfo
		quiet  bool // suppresses OnChanged while we set the radio programmatically
		radio  *widget.RadioGroup
		detect func()
	)

	installBtn := widget.NewButtonWithIcon("Install Kafka via Docker", theme.DownloadIcon(), func() {
		u.showDockerInstall(func() { u.rebuildShell("settings") })
	})
	installBtn.Importance = widget.HighImportance
	installBtn.Disable()

	setRadio := func(mode string) {
		quiet = true
		if mode == config.ModeDocker {
			radio.SetSelected(modeDockerLabel)
		} else {
			radio.SetSelected(modeLocalLabel)
		}
		quiet = false
	}

	radio = widget.NewRadioGroup([]string{modeLocalLabel, modeDockerLabel}, func(sel string) {
		if quiet {
			return
		}
		want := config.ModeLocal
		if sel == modeDockerLabel {
			want = config.ModeDocker
		}
		cur := u.cfg.Mode
		if cur == "" {
			cur = config.ModeLocal
		}
		if want == cur {
			return
		}
		// Revert immediately; the switch only takes effect once confirmed.
		setRadio(cur)

		if want == config.ModeDocker {
			mu.Lock()
			di := info
			mu.Unlock()
			if !di.Usable() {
				u.errorDialog(fmt.Errorf("docker mode is not available: %s", di.Summary()))
				return
			}
			if !kafka.ComposeFileExists(u.cfg.DockerDirOrDefault()) {
				u.confirm("Kafka is not installed in Docker yet",
					"No docker-compose.yml was found in "+u.cfg.DockerDirOrDefault()+".\n\nInstall Kafka via Docker now?",
					func() { u.showDockerInstall(func() { u.rebuildShell("settings") }) })
				return
			}
		} else if len(config.ValidateKafkaPath(u.cfg.KafkaPath)) > 0 {
			u.errorDialog(fmt.Errorf("set a valid local Kafka Path above and save it before switching to local mode"))
			return
		}
		u.switchMode(want)
	})
	radio.Horizontal = true
	setRadio(u.cfg.Mode)

	recheck := widget.NewButtonWithIcon("Re-check", theme.ViewRefreshIcon(), func() { detect() })
	openDir := widget.NewButtonWithIcon("Open Docker Folder", theme.FolderOpenIcon(), func() {
		u.openInExplorer(u.cfg.DockerDirOrDefault())
	})

	detect = func() {
		status.SetText("Checking for Docker…")
		installBtn.Disable()
		go func() {
			di := kafka.DetectDocker()
			mu.Lock()
			info = di
			mu.Unlock()
			fyne.Do(func() {
				if di.Usable() {
					status.SetText("✅ " + di.Summary())
					installBtn.Enable()
				} else {
					status.SetText("⚠ " + di.Summary())
					installBtn.Disable()
				}
			})
		}()
	}
	detect()

	hint := widget.NewLabel("Docker mode runs a single-node Kafka broker (KRaft, no ZooKeeper) in a container " +
		"named \"kafka\" on localhost:9092. Topics, Producer, Consumer and Consumer Groups then run their " +
		"commands with `docker exec kafka /opt/kafka/bin/*.sh`.")
	hint.Wrapping = fyne.TextWrapWord
	hint.Importance = widget.LowImportance

	return card("Kafka Mode", container.NewVBox(
		container.NewBorder(nil, nil, nil, recheck, status),
		radio,
		container.NewHBox(installBtn, openDir),
		hint,
	))
}

// switchMode persists a new run mode and rebuilds the shell so every page
// picks it up. Running services must be stopped first (both modes use 9092).
func (u *AppUI) switchMode(mode string) {
	if u.mgr.Broker.State() != kafka.StateStopped || u.mgr.ZooKeeper.State() != kafka.StateStopped {
		u.errorDialog(fmt.Errorf("stop the running ZooKeeper / Kafka broker first (Services & Logs page), then switch mode"))
		return
	}
	u.cfg.Mode = mode
	if err := u.cfg.Save(); err != nil {
		u.errorDialog(err)
		return
	}
	u.mgr.SetConfig(u.cfg)
	u.mgr.InvalidateTopicCache()
	u.rebuildShell("settings")
	if mode == config.ModeDocker {
		go u.mgr.Broker.AttachDocker()
		u.toast("Switched to Docker mode")
	} else {
		u.toast("Switched to local mode")
	}
}

// rebuildShell drops all cached pages and rebuilds the main window content,
// then shows the given page.
func (u *AppUI) rebuildShell(page string) {
	u.mgr.ZooKeeper.ClearSubscribers()
	u.mgr.Broker.ClearSubscribers()
	u.pages = map[string]fyne.CanvasObject{}
	u.onShow = map[string]func(){}
	u.win.SetContent(u.buildShell())
	for i, it := range u.navData {
		if it.id == page {
			u.nav.Select(i)
		}
	}
}

// showDockerInstall shows the "Install Kafka via Docker" dialog. It runs the
// setup steps (verify docker, create folder, write docker-compose.yml, pull,
// up -d) with a live log, then switches the app to Docker mode. onDone runs
// after the user closes the dialog following a successful install.
func (u *AppUI) showDockerInstall(onDone func()) {
	dirEntry := widget.NewEntry()
	dirEntry.SetText(u.cfg.DockerDirOrDefault())
	imageEntry := widget.NewEntry()
	imageEntry.SetText(u.cfg.DockerImageOrDefault())

	preview := widget.NewMultiLineEntry()
	preview.TextStyle = fyne.TextStyle{Monospace: true}
	preview.Wrapping = fyne.TextWrapOff
	preview.SetText(kafka.ComposeYAML(imageEntry.Text))
	preview.SetMinRowsVisible(10)
	preview.Disable()
	imageEntry.OnChanged = func(s string) { preview.SetText(kafka.ComposeYAML(strings.TrimSpace(s))) }

	logView := widget.NewMultiLineEntry()
	logView.TextStyle = fyne.TextStyle{Monospace: true}
	logView.Wrapping = fyne.TextWrapOff
	logView.SetMinRowsVisible(12)

	var (
		logMu   sync.Mutex
		logBuf  []string
		cancel  context.CancelFunc
		success bool
	)
	appendLine := func(line string) {
		logMu.Lock()
		logBuf = append(logBuf, line)
		if len(logBuf) > 800 {
			logBuf = logBuf[len(logBuf)-800:]
		}
		text := strings.Join(logBuf, "\n")
		logMu.Unlock()
		fyne.Do(func() {
			logView.SetText(text)
			logView.CursorRow = strings.Count(text, "\n")
			logView.Refresh()
		})
	}

	progress := widget.NewProgressBarInfinite()
	progress.Stop()
	progress.Hide()
	stepLbl := widget.NewLabel("")

	installBtn := widget.NewButtonWithIcon("Install & Start", theme.DownloadIcon(), nil)
	installBtn.Importance = widget.HighImportance
	closeBtn := widget.NewButtonWithIcon("Close", theme.CancelIcon(), nil)

	form := widget.NewForm(
		widget.NewFormItem("Docker Folder", dirEntry),
		widget.NewFormItem("Kafka Image", imageEntry),
	)
	steps := widget.NewLabel("Steps: 1) docker version / docker compose version  2) create the folder  " +
		"3) write docker-compose.yml  4) docker compose pull  5) docker compose up -d")
	steps.Wrapping = fyne.TextWrapWord

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Install Log", theme.ListIcon(), container.NewScroll(logView)),
		container.NewTabItemWithIcon("docker-compose.yml", theme.FileIcon(), container.NewScroll(preview)),
	)

	body := container.NewBorder(
		container.NewVBox(form, steps, container.NewBorder(nil, nil, nil, nil, progress), stepLbl),
		container.NewHBox(installBtn, closeBtn),
		nil, nil, tabs)

	d := dialog.NewCustomWithoutButtons("Install Kafka via Docker", body, u.win)
	d.Resize(fyne.NewSize(860, 620))

	closeBtn.OnTapped = func() {
		if cancel != nil {
			cancel() // abort a running pull/up
		}
		d.Hide()
		if success && onDone != nil {
			onDone()
		}
	}

	installBtn.OnTapped = func() {
		dir := strings.TrimSpace(dirEntry.Text)
		image := strings.TrimSpace(imageEntry.Text)
		if dir == "" || image == "" {
			u.errorDialog(fmt.Errorf("docker folder and image are required"))
			return
		}
		installBtn.Disable()
		dirEntry.Disable()
		imageEntry.Disable()
		closeBtn.SetText("Cancel")
		progress.Show()
		progress.Start()
		stepLbl.SetText("Installing… (the first image download can take a few minutes)")
		tabs.SelectIndex(0)

		ctx, c := context.WithCancel(context.Background())
		cancel = c
		go func() {
			err := u.mgr.InstallDocker(ctx, dir, image, appendLine)
			if err == nil {
				u.cfg.Mode = config.ModeDocker
				u.cfg.DockerDir = dir
				u.cfg.DockerImage = image
				err = u.cfg.Save()
				u.mgr.SetConfig(u.cfg)
				u.mgr.InvalidateTopicCache()
			}
			fyne.Do(func() {
				progress.Stop()
				progress.Hide()
				cancel = nil
				closeBtn.SetText("Close")
				if err != nil {
					appendLine("")
					appendLine("✗ " + err.Error())
					stepLbl.SetText("Install failed — see the log. Fix the problem and try again.")
					installBtn.Enable()
					dirEntry.Enable()
					imageEntry.Enable()
					return
				}
				success = true
				closeBtn.SetText("Done")
				closeBtn.Importance = widget.HighImportance
				closeBtn.Refresh()
				stepLbl.SetText("✅ Kafka is running in Docker. The app is now in Docker mode.")
				u.toast("Kafka installed via Docker")
			})
			if err == nil {
				u.mgr.Broker.AttachDocker()
			}
		}()
	}

	d.Show()
}
