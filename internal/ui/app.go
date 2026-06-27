// Package ui implements the Fyne desktop interface for the Kafka manager.
package ui

import (
	"os"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/config"
	"kafkadesktop/internal/kafka"
)

// AppUI is the root of the desktop application.
type AppUI struct {
	fyneApp fyne.App
	win     fyne.Window

	cfg *config.Config
	mgr *kafka.Manager

	content *fyne.Container // swappable page area
	nav     *widget.List
	navData []navItem
	pages   map[string]fyne.CanvasObject

	// lazily built pages cache rebuild hooks
	onShow map[string]func()

	// auto-refresh guards keyed by page name
	refreshMu     sync.Mutex
	refreshActive map[string]bool
}

type navItem struct {
	id    string
	title string
	icon  fyne.Resource
}

// Run shows the animated splash, then the main window, blocking until it closes.
func Run() {
	// Pin the UI scale so rendering is consistent on every launch. Fyne
	// otherwise auto-detects the display scale, which can vary between runs
	// (multi-monitor / DPI-query timing) and make the UI render huge one time
	// and normal the next. Users can still override by setting FYNE_SCALE.
	if os.Getenv("FYNE_SCALE") == "" {
		_ = os.Setenv("FYNE_SCALE", "1")
	}

	a := app.NewWithID("github.com/itzganesh03")
	a.SetIcon(appIcon)

	cfg, _ := config.Load()
	applyTheme(a, cfg)

	ui := &AppUI{
		fyneApp:       a,
		cfg:           cfg,
		mgr:           kafka.NewManager(cfg),
		pages:         map[string]fyne.CanvasObject{},
		onShow:        map[string]func(){},
		refreshActive: map[string]bool{},
	}

	ui.win = a.NewWindow("One Way Kafka Manager")
	ui.win.SetIcon(appIcon)
	ui.win.SetMaster() // closing the main window quits the app
	// Keep the default size within the working area of common hi-DPI displays
	// (a 1080p screen at 150% scale is only ~1280x720 device-independent px),
	// otherwise CenterOnScreen can push the title bar / close button off-screen.
	ui.win.Resize(fyne.NewSize(1024, 640))
	ui.win.CenterOnScreen()
	ui.registerShortcuts()

	// Prepare main content (kept hidden until the splash finishes).
	isShell := config.Exists() && len(config.ValidateKafkaPath(cfg.KafkaPath)) == 0
	if isShell {
		ui.win.SetContent(ui.buildShell())
	} else {
		ui.win.SetContent(ui.buildWizard())
	}

	// Animated splash, then reveal the main window.
	showSplash(a, func() {
		ui.win.Show()
		if isShell {
			ui.maybeAutoStart()
		}
	})

	a.Run()
}

// applyTheme sets the app theme from config.
func applyTheme(a fyne.App, cfg *config.Config) {
	if cfg != nil && cfg.Theme == "light" {
		a.Settings().SetTheme(lightTheme{})
	} else {
		a.Settings().SetTheme(darkTheme{})
	}
}

// buildShell constructs the main navigation + content layout.
func (u *AppUI) buildShell() fyne.CanvasObject {
	u.navData = []navItem{
		{"dashboard", "Dashboard", theme.HomeIcon()},
		{"services", "Services & Logs", theme.MediaPlayIcon()},
		{"topics", "Topics", theme.ListIcon()},
		{"producer", "Producer", theme.MailSendIcon()},
		{"consumer", "Consumer", theme.DownloadIcon()},
		{"groups", "Consumer Groups", theme.AccountIcon()},
		{"history", "Command History", theme.HistoryIcon()},
		{"settings", "Settings", theme.SettingsIcon()},
	}

	u.nav = widget.NewList(
		func() int { return len(u.navData) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.HomeIcon()), widget.NewLabel("template"))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container)
			row.Objects[0].(*widget.Icon).SetResource(u.navData[i].icon)
			row.Objects[1].(*widget.Label).SetText(u.navData[i].title)
		},
	)
	u.nav.OnSelected = func(id widget.ListItemID) {
		u.showPage(u.navData[id].id)
	}

	title := widget.NewLabelWithStyle("  Kafka Manager", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	side := container.NewBorder(title, nil, nil, nil, u.nav)

	u.content = container.NewStack(widget.NewLabel("Loading..."))

	split := container.NewHSplit(side, u.content)
	split.SetOffset(0.18)

	// Default to dashboard.
	u.nav.Select(0)
	return split
}

// showPage swaps the content area to the named page, building it on demand.
func (u *AppUI) showPage(id string) {
	page, ok := u.pages[id]
	if !ok {
		page = u.buildPage(id)
		u.pages[id] = page
	}
	u.content.Objects = []fyne.CanvasObject{page}
	u.content.Refresh()
	if fn := u.onShow[id]; fn != nil {
		fn()
	}
}

// buildPage constructs a page by id.
func (u *AppUI) buildPage(id string) fyne.CanvasObject {
	switch id {
	case "dashboard":
		return u.buildDashboard()
	case "services":
		return u.buildServices()
	case "topics":
		return u.buildTopics()
	case "producer":
		return u.buildProducer()
	case "consumer":
		return u.buildConsumer()
	case "groups":
		return u.buildGroups()
	case "history":
		return u.buildHistory()
	case "settings":
		return u.buildSettings()
	}
	return widget.NewLabel("Unknown page: " + id)
}

// reloadShell rebuilds the entire shell (after settings/path change).
func (u *AppUI) reloadShell() {
	u.pages = map[string]fyne.CanvasObject{}
	u.onShow = map[string]func(){}
	u.refreshMu.Lock()
	u.refreshActive = map[string]bool{}
	u.refreshMu.Unlock()
	applyTheme(u.fyneApp, u.cfg)
	u.win.SetContent(u.buildShell())
}

// registerShortcuts wires keyboard shortcuts: Ctrl+1..8 switch pages,
// Ctrl+R / F5 refresh the current page.
func (u *AppUI) registerShortcuts() {
	c := u.win.Canvas()
	pages := []string{"dashboard", "services", "topics", "producer", "consumer", "groups", "history", "settings"}
	keys := []fyne.KeyName{fyne.Key1, fyne.Key2, fyne.Key3, fyne.Key4, fyne.Key5, fyne.Key6, fyne.Key7, fyne.Key8}
	for i, k := range keys {
		idx := i
		c.AddShortcut(&desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) {
			if u.nav != nil {
				u.nav.Select(idx)
			}
		})
		_ = pages
	}
	c.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyF5 {
			u.refreshCurrent()
		}
	})
}

// refreshCurrent re-runs the on-show hook for the visible page if present.
func (u *AppUI) refreshCurrent() {
	for id, page := range u.pages {
		if u.isPageVisible(page) {
			if fn := u.onShow[id]; fn != nil {
				fn()
			}
			return
		}
	}
}

// maybeAutoStart launches services if configured to do so.
func (u *AppUI) maybeAutoStart() {
	if u.cfg.AutoStartZK {
		go func() { _ = u.mgr.ZooKeeper.Start() }()
	}
	if u.cfg.AutoStartKafka {
		go func() { _ = u.mgr.Broker.Start() }()
	}
}
