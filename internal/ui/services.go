package ui

import (
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/kafka"
)

// maxConsoleLines caps how many log lines are rendered in a console at once
// (the full buffer is still kept in the Service for export).
const maxConsoleLines = 600

// buildServices builds the service control + live logs page.
func (u *AppUI) buildServices() fyne.CanvasObject {
	zkPanel, zkFlush := u.servicePanel(u.mgr.ZooKeeper)
	brokerPanel, brokerFlush := u.servicePanel(u.mgr.Broker)

	header := container.NewVBox(sectionTitle("Services & Logs", theme.MediaPlayIcon()), widget.NewSeparator())
	body := container.NewGridWithColumns(2, zkPanel, brokerPanel)
	page := container.NewBorder(header, nil, nil, nil, body)

	// Throttled flush: logs are appended to a buffer by the service; we re-render
	// the consoles at most a few times per second to stay smooth under load.
	u.onShow["services"] = func() {
		u.startAutoRefresh("services", page, 300*time.Millisecond, func() {
			fyne.Do(func() {
				zkFlush()
				brokerFlush()
			})
		})
	}
	return page
}

// servicePanel builds the control + log console for a single service and
// returns the panel plus a flush function that re-renders the console.
func (u *AppUI) servicePanel(svc *kafka.Service) (fyne.CanvasObject, func()) {
	dot := statusDot(false, false)
	stateLbl := widget.NewLabel(stateText(svc.State()))

	logView := widget.NewMultiLineEntry()
	logView.Wrapping = fyne.TextWrapOff
	logView.SetMinRowsVisible(10)

	var (
		mu         sync.Mutex
		dirty      = true
		autoFollow = true
	)

	// flush re-renders the console from the (capped) service buffer if changed.
	flush := func() {
		mu.Lock()
		if !dirty {
			mu.Unlock()
			return
		}
		dirty = false
		follow := autoFollow
		mu.Unlock()

		lines := svc.Logs()
		if len(lines) > maxConsoleLines {
			lines = lines[len(lines)-maxConsoleLines:]
		}
		text := strings.Join(lines, "\n")
		if text == logView.Text {
			return
		}
		logView.SetText(text)
		if follow {
			logView.CursorRow = strings.Count(text, "\n")
			logView.Refresh()
		}
	}

	svc.SubscribeLogs(func(string) {
		mu.Lock()
		dirty = true
		mu.Unlock()
	})

	autoCheck := widget.NewCheck("Auto-scroll", func(b bool) {
		mu.Lock()
		autoFollow = b
		mu.Unlock()
	})
	autoCheck.SetChecked(true)

	startBtn := widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), nil)
	stopBtn := widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), nil)
	restartBtn := widget.NewButtonWithIcon("Restart", theme.ViewRefreshIcon(), nil)
	clearBtn := widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), func() {
		svc.ClearLogs()
		mu.Lock()
		dirty = true
		mu.Unlock()
		logView.SetText("")
	})
	exportBtn := widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), func() {
		u.exportText(svc.Name+" logs", strings.Join(svc.Logs(), "\n"))
	})

	startBtn.OnTapped = func() {
		go func() {
			if err := svc.Start(); err != nil {
				fyne.Do(func() { u.errorDialog(err) })
			}
		}()
	}
	stopBtn.OnTapped = func() {
		go func() {
			if err := svc.Stop(); err != nil {
				fyne.Do(func() { u.errorDialog(err) })
			}
		}()
	}
	restartBtn.OnTapped = func() {
		go func() {
			if err := svc.Restart(); err != nil {
				fyne.Do(func() { u.errorDialog(err) })
			}
		}()
	}

	applyState := func(st kafka.State) {
		fyne.Do(func() {
			dot.FillColor = stateColor(st)
			dot.Refresh()
			stateLbl.SetText(stateText(st))
			running := st == kafka.StateRunning || st == kafka.StateStarting
			if running {
				startBtn.Disable()
				stopBtn.Enable()
			} else {
				startBtn.Enable()
				stopBtn.Disable()
			}
		})
		if st == kafka.StateRunning {
			u.notify(svc.Name, svc.Name+" is running")
		}
	}
	applyState(svc.State())
	svc.SubscribeState(applyState)

	controls := container.NewHBox(startBtn, stopBtn, restartBtn, clearBtn, exportBtn, autoCheck)
	head := container.NewHBox(container.NewCenter(dot),
		widget.NewLabelWithStyle(svc.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), stateLbl)

	top := container.NewVBox(head, controls, widget.NewSeparator())
	console := container.NewScroll(logView)
	return card("", container.NewBorder(top, nil, nil, nil, console)), flush
}
