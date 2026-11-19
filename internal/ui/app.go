// Package ui implements the Fyne desktop interface for the Kafka manager.
package ui

import (
	"image/color"
	"net/url"
	"os"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
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
	nav     *navBar
	navData []navItem
	pages   map[string]fyne.CanvasObject
	current string       // id of the visible page
	crumb   *canvas.Text // top bar page name

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

	a := app.NewWithID("com.phoenix.kafkadesktop")
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
	isShell := config.Exists() && cfg.Ready()
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
	dark := cfg == nil || cfg.Theme != "light"
	if dark {
		pal = darkPalette
	} else {
		pal = lightPalette
	}
	textPrimary, textMuted = pal.text, pal.muted
	a.Settings().SetTheme(glassTheme{dark: dark})
}

// buildShell constructs the main navigation + content layout.
func (u *AppUI) buildShell() fyne.CanvasObject {
	repaints = nil // drop hooks of the replaced content (the setup wizard)
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

	u.nav = newNavBar(u.navData)
	u.nav.OnSelected = func(id int) {
		u.showPage(u.navData[id].id)
	}

	// Sidebar: brand header, menu, credit footer — all on one glass panel.
	logo := canvas.NewImageFromResource(appIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(30, 30))
	brandName := themedText("Kafka Manager", false)
	brandName.TextStyle = fyne.TextStyle{Bold: true}
	brandName.TextSize = 16
	brandSub := themedText("One Way", true)
	brandSub.TextSize = 11
	brand := container.NewBorder(nil, nil, logo, nil,
		vCentered(container.New(layout.NewCustomPaddedLayout(0, 0, 6, 0),
			container.NewVBox(brandName, brandSub))))
	menu := container.NewVBox(container.New(layout.NewCustomPaddedLayout(4, 14, 4, 0), brand), u.nav.root)
	sideWidth := canvas.NewRectangle(color.Transparent)
	sideWidth.SetMinSize(fyne.NewSize(220, 0))
	side := container.NewStack(sideWidth, glassPanel(
		container.NewBorder(menu, u.sidebarFooter(), nil, nil)))

	u.content = container.NewStack(widget.NewLabel("Loading..."))
	main := container.NewBorder(u.topBar(), nil, nil, nil,
		container.New(layout.NewCustomPaddedLayout(10, 0, 0, 0), u.content))

	body := container.NewBorder(nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), side), nil, main)
	shell := container.NewStack(glassBackdrop(),
		container.New(layout.NewCustomPaddedLayout(12, 12, 12, 12), body))

	// Default to dashboard.
	u.nav.Select(0)
	return shell
}

// topBar is the glass strip above the page: breadcrumb, live service pills
// and the light/dark toggle.
func (u *AppUI) topBar() fyne.CanvasObject {
	home := themedText("Kafka Manager", true)
	home.TextSize = 13
	sep := themedText("›", true)
	sep.TextSize = 13
	u.crumb = themedText("", false)
	u.crumb.TextSize = 13
	u.crumb.TextStyle = fyne.TextStyle{Bold: true}
	crumbs := container.NewHBox(vCentered(home), vCentered(sep), vCentered(u.crumb))

	zk := newStatusPill("ZooKeeper")
	broker := newStatusPill("Broker")
	bind := func(p *statusPill, name string, svc *kafka.Service) {
		apply := func(st kafka.State) {
			fyne.Do(func() { p.set(name+" · "+dashStateText(st), stateColor(st)) })
		}
		apply(svc.State())
		svc.SubscribeState(apply)
	}
	bind(zk, "ZooKeeper", u.mgr.ZooKeeper)
	bind(broker, "Broker", u.mgr.Broker)

	themeBtn := widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
		next := "light"
		if u.cfg.Theme == "light" {
			next = "dark"
		}
		u.cfg.Theme = next
		if err := u.cfg.Save(); err != nil {
			u.errorDialog(err)
		}
		u.setTheme(next)
	})
	themeBtn.Importance = widget.LowImportance
	paint(func() {
		// label names the theme the button switches to
		if u.cfg.Theme == "light" {
			themeBtn.SetText("Dark")
		} else {
			themeBtn.SetText("Light")
		}
	})

	right := container.NewHBox(vCentered(zk.root), vCentered(broker.root), themeBtn)
	if u.cfg.IsDocker() {
		// No ZooKeeper in Docker (KRaft) mode.
		docker := newStatusPill("Docker")
		docker.set("Docker mode", accent(colorBlue))
		right = container.NewHBox(vCentered(docker.root), vCentered(broker.root), themeBtn)
	}
	return glassPanel(container.NewBorder(nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 6, 0), crumbs), right))
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
	u.current = id
	if u.crumb != nil {
		for _, it := range u.navData {
			if it.id == id {
				u.crumb.Text = it.title
				u.crumb.Refresh()
			}
		}
	}
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

// setTheme switches the theme in place. Widgets restyle themselves; the
// hand-drawn glass pieces are re-colored via their paint hooks. Pages are not
// rebuilt, so running consumers, selections and typed text survive.
func (u *AppUI) setTheme(name string) {
	u.cfg.Theme = name
	applyTheme(u.fyneApp, u.cfg)
	for _, fn := range repaints {
		fn()
	}
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

// sidebarFooter builds the "Developed by" credit pinned to the bottom of the
// navigation sidebar, with a clickable GitHub link.
func (u *AppUI) sidebarFooter() fyne.CanvasObject {
	credit := themedText("Developed by Ganesh Modi", true)
	credit.TextSize = 11
	credit.TextStyle = fyne.TextStyle{Italic: true}
	credit.Alignment = fyne.TextAlignCenter

	ghURL, _ := url.Parse("https://github.com/itzganesh03")
	link := widget.NewHyperlink("github.com/itzganesh03", ghURL)
	link.Alignment = fyne.TextAlignCenter

	return container.NewVBox(
		widget.NewSeparator(),
		container.New(layout.NewCustomPaddedLayout(6, 0, 0, 0), credit),
		container.NewCenter(link),
	)
}

// maybeAutoStart launches services if configured to do so.
func (u *AppUI) maybeAutoStart() {
	if u.cfg.IsDocker() {
		go func() {
			// Pick up a container that is already running, otherwise start it
			// if auto-start is on.
			u.mgr.Broker.AttachDocker()
			if u.cfg.AutoStartKafka && u.mgr.Broker.State() == kafka.StateStopped {
				_ = u.mgr.Broker.Start()
			}
		}()
		return
	}
	if u.cfg.AutoStartZK {
		go func() { _ = u.mgr.ZooKeeper.Start() }()
	}
	if u.cfg.AutoStartKafka {
		go func() { _ = u.mgr.Broker.Start() }()
	}
}
