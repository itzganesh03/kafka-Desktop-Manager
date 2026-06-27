package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/kafka"
)

// card sizing for the dashboard metric tiles.
const (
	cardW = 340
	cardH = 120
)

// metricCard is a compact Grafana-style stat tile: a rounded panel with a
// colored accent stripe, a title, a large value and an optional status dot.
type metricCard struct {
	root   fyne.CanvasObject
	stripe *canvas.Rectangle
	value  *canvas.Text
	sub    *canvas.Text
	dot    *canvas.Circle
	pulse  *fyne.Animation
}

func newMetricCard(title string, icon fyne.Resource, accent color.NRGBA, big, withDot bool) *metricCard {
	bg := canvas.NewRectangle(color.NRGBA{R: 0x1a, G: 0x20, B: 0x29, A: 0xff})
	bg.CornerRadius = 14
	bg.StrokeColor = color.NRGBA{R: 0x2a, G: 0x32, B: 0x3d, A: 0xff}
	bg.StrokeWidth = 1

	stripe := canvas.NewRectangle(accent)
	stripe.CornerRadius = 6
	stripe.SetMinSize(fyne.NewSize(5, 10))

	titleTxt := canvas.NewText(title, color.NRGBA{R: 0x8a, G: 0x92, B: 0x9c, A: 0xff})
	titleTxt.TextStyle = fyne.TextStyle{Bold: true}
	titleTxt.TextSize = 12

	var head fyne.CanvasObject = titleTxt
	if icon != nil {
		ic := widget.NewIcon(icon)
		head = container.NewHBox(ic, titleTxt)
	}

	value := canvas.NewText("—", color.NRGBA{R: 0xe6, G: 0xed, B: 0xf3, A: 0xff})
	value.TextStyle = fyne.TextStyle{Bold: true}
	if big {
		value.TextSize = 34
	} else {
		value.TextSize = 16
	}

	sub := canvas.NewText("", color.NRGBA{R: 0x7a, G: 0x84, B: 0x90, A: 0xff})
	sub.TextSize = 12

	mc := &metricCard{stripe: stripe, value: value, sub: sub}

	var valueRow fyne.CanvasObject = value
	if withDot {
		mc.dot = canvas.NewCircle(color.NRGBA{R: 0xe0, G: 0x4f, B: 0x4f, A: 0xff})
		dotWrap := container.NewGridWrap(fyne.NewSize(14, 14), mc.dot)
		valueRow = container.NewHBox(container.NewCenter(dotWrap), value)
	}

	body := container.NewVBox(head, layoutSpacer(), valueRow, sub)
	inner := container.NewBorder(nil, nil, stripe, nil, container.NewPadded(body))
	mc.root = container.NewStack(bg, container.NewPadded(inner))
	return mc
}

// layoutSpacer returns a flexible vertical spacer.
func layoutSpacer() fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, 6))
	return r
}

// setValue sets the large value text (UI thread).
func (m *metricCard) setValue(s string, c color.NRGBA) {
	m.value.Text = s
	m.value.Color = c
	m.value.Refresh()
}

func (m *metricCard) setSub(s string) {
	m.sub.Text = s
	m.sub.Refresh()
}

func (m *metricCard) setAccent(c color.NRGBA) {
	m.stripe.FillColor = c
	m.stripe.Refresh()
}

// startPulse animates the status dot gently between full and faded accent.
func (m *metricCard) startPulse(c color.NRGBA) {
	if m.dot == nil || m.pulse != nil {
		if m.dot != nil {
			m.dot.FillColor = c
			m.dot.Refresh()
		}
		return
	}
	faded := c
	faded.A = 0x44
	a := canvas.NewColorRGBAAnimation(c, faded, 1100*time.Millisecond, func(cc color.Color) {
		m.dot.FillColor = cc
		canvas.Refresh(m.dot)
	})
	a.AutoReverse = true
	a.RepeatCount = fyne.AnimationRepeatForever
	a.Start()
	m.pulse = a
}

func (m *metricCard) stopPulse(static color.NRGBA) {
	if m.pulse != nil {
		m.pulse.Stop()
		m.pulse = nil
	}
	if m.dot != nil {
		m.dot.FillColor = static
		m.dot.Refresh()
	}
}

// buildDashboard builds the live status dashboard.
func (u *AppUI) buildDashboard() fyne.CanvasObject {
	installCard := newMetricCard("KAFKA INSTALLATION", theme.FolderIcon(), accentTeal, false, false)
	zkCard := newMetricCard("ZOOKEEPER", theme.StorageIcon(), red, false, true)
	brokerCard := newMetricCard("KAFKA BROKER", theme.ComputerIcon(), red, false, true)
	addrCard := newMetricCard("BROKER ADDRESS", theme.ComputerIcon(), accentBlue, false, false)
	topicsCard := newMetricCard("TOPICS", theme.ListIcon(), accentPurple, true, false)
	groupsCard := newMetricCard("CONSUMER GROUPS", theme.AccountIcon(), accentOrange, true, false)

	installCard.setValue(shortPath(u.cfg.KafkaPath), textPrimary)
	addrCard.setValue(u.cfg.BootstrapServer, textPrimary)

	grid := container.NewGridWrap(fyne.NewSize(cardW, cardH),
		installCard.root, zkCard.root, brokerCard.root,
		addrCard.root, topicsCard.root, groupsCard.root,
	)

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), nil)
	updatedLbl := widget.NewLabel("")
	header := container.NewBorder(nil, nil,
		sectionTitle("Dashboard", theme.HomeIcon()),
		container.NewHBox(updatedLbl, refreshBtn))

	// --- Consumer group lag panel ---
	var lagRows []kafka.GroupOffset
	groupSelect := widget.NewSelect(nil, nil)
	groupSelect.PlaceHolder = "Select a consumer group"
	totalLagLbl := canvas.NewText("", textMuted)
	totalLagLbl.TextStyle = fyne.TextStyle{Bold: true}
	totalLagLbl.TextSize = 14

	lagTable := widget.NewTable(
		func() (int, int) { return len(lagRows) + 1, 5 },
		func() fyne.CanvasObject { return widget.NewLabel("cell") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			if id.Row == 0 {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.Importance = widget.MediumImportance
				lbl.SetText([]string{"Topic", "Partition", "Current", "Log End", "Lag"}[id.Col])
				return
			}
			r := lagRows[id.Row-1]
			lbl.TextStyle = fyne.TextStyle{}
			lbl.Importance = widget.MediumImportance
			switch id.Col {
			case 0:
				lbl.SetText(r.Topic)
			case 1:
				lbl.SetText(fmt.Sprintf("%d", r.Partition))
			case 2:
				lbl.SetText(r.CurrentOffset)
			case 3:
				lbl.SetText(r.LogEndOffset)
			case 4:
				lbl.SetText(fmt.Sprintf("%d", r.Lag))
				if r.Lag > 0 {
					lbl.Importance = widget.DangerImportance
				} else {
					lbl.Importance = widget.SuccessImportance
				}
			}
		},
	)
	for i, w := range []float32{300, 90, 110, 110, 90} {
		lagTable.SetColumnWidth(i, w)
	}

	describeLag := func(group string) {
		if group == "" {
			return
		}
		go func() {
			rows, err := u.mgr.DescribeGroup(group)
			fyne.Do(func() {
				if err != nil {
					totalLagLbl.Text = "error"
					totalLagLbl.Color = red
					totalLagLbl.Refresh()
					return
				}
				var total int64
				for _, r := range rows {
					if r.Lag > 0 {
						total += r.Lag
					}
				}
				lagRows = rows
				lagTable.Refresh()
				totalLagLbl.Text = fmt.Sprintf("Total lag: %d", total)
				if total > 0 {
					totalLagLbl.Color = red
				} else {
					totalLagLbl.Color = green
				}
				totalLagLbl.Refresh()
			})
		}()
	}
	groupSelect.OnChanged = describeLag

	loadGroups := func() {
		if u.mgr.Broker.State() != kafka.StateRunning {
			return
		}
		go func() {
			names, err := u.mgr.ListGroups()
			if err != nil {
				return
			}
			fyne.Do(func() {
				groupSelect.Options = names
				groupSelect.Refresh()
			})
		}()
	}

	lagRefreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		loadGroups()
		describeLag(groupSelect.Selected)
	})
	lagControls := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewLabel("Consumer Group:"), lagRefreshBtn),
		totalLagLbl, groupSelect)
	lagScroll := container.NewScroll(lagTable)
	lagScroll.SetMinSize(fyne.NewSize(0, 220))
	lagPanel := card("Consumer Group Lag", container.NewBorder(
		container.NewVBox(lagControls, widget.NewSeparator()), nil, nil, nil, lagScroll))

	// applyService updates a service card from its (in-memory, cheap) state.
	applyService := func(c *metricCard, st kafka.State) {
		col := stateColor(st)
		fyne.Do(func() {
			c.setAccent(col)
			c.setValue(dashStateText(st), col)
			if st == kafka.StateRunning || st == kafka.StateStarting {
				c.startPulse(col)
			} else {
				c.stopPulse(col)
			}
		})
	}

	// fastRefresh: in-memory service state only (no JVM spawned) — runs often.
	fastRefresh := func() {
		applyService(zkCard, u.mgr.ZooKeeper.State())
		applyService(brokerCard, u.mgr.Broker.State())
		fyne.Do(func() {
			installCard.setValue(shortPath(u.cfg.KafkaPath), textPrimary)
			addrCard.setValue(u.cfg.BootstrapServer, textPrimary)
		})
	}

	// countRefresh: expensive (spawns Kafka tools) — runs rarely / on demand.
	countRefresh := func() {
		if u.mgr.Broker.State() != kafka.StateRunning {
			fyne.Do(func() {
				topicsCard.setValue("—", textMuted)
				topicsCard.setSub("broker stopped")
				groupsCard.setValue("—", textMuted)
				groupsCard.setSub("broker stopped")
			})
			return
		}
		fyne.Do(func() {
			topicsCard.setSub("updating…")
			groupsCard.setSub("updating…")
		})
		topics, terr := u.mgr.ListTopics()
		groups, gerr := u.mgr.ListGroups()
		fyne.Do(func() {
			if terr == nil {
				topicsCard.setValue(fmt.Sprintf("%d", len(topics)), textPrimary)
				topicsCard.setSub("topics")
			} else {
				topicsCard.setValue("—", textMuted)
				topicsCard.setSub("error")
			}
			if gerr == nil {
				groupsCard.setValue(fmt.Sprintf("%d", len(groups)), textPrimary)
				groupsCard.setSub("groups")
			} else {
				groupsCard.setValue("—", textMuted)
				groupsCard.setSub("error")
			}
			updatedLbl.SetText("updated " + time.Now().Format("15:04:05"))
		})
	}

	refreshBtn.OnTapped = func() {
		go fastRefresh()
		go countRefresh()
		loadGroups()
		describeLag(groupSelect.Selected)
	}

	content := container.NewVBox(
		container.NewPadded(grid),
		container.NewPadded(lagPanel),
	)
	page := container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil,
		container.NewVScroll(content))

	// Two cadences: cheap state every 2s, expensive counts every 25s. Both are
	// (re)started whenever the dashboard becomes visible and stop when hidden.
	u.onShow["dashboard"] = func() {
		u.startAutoRefresh("dashboard-fast", page, 2*time.Second, fastRefresh)
		u.startAutoRefresh("dashboard-counts", page, 25*time.Second, func() {
			countRefresh()
			loadGroups()
			if groupSelect.Selected != "" {
				describeLag(groupSelect.Selected)
			}
		})
	}
	return page
}

// shortPath trims long paths for display.
func shortPath(p string) string {
	if len(p) <= 34 {
		return p
	}
	return p[:16] + "…" + p[len(p)-16:]
}

// accent palette
var (
	accentTeal   = color.NRGBA{R: 0x2e, G: 0xb8, B: 0xa6, A: 0xff}
	accentBlue   = color.NRGBA{R: 0x3b, G: 0x8e, B: 0xea, A: 0xff}
	accentPurple = color.NRGBA{R: 0x9b, G: 0x6d, B: 0xff, A: 0xff}
	accentOrange = color.NRGBA{R: 0xe6, G: 0x8a, B: 0x2e, A: 0xff}
	textPrimary  = color.NRGBA{R: 0xe6, G: 0xed, B: 0xf3, A: 0xff}
	textMuted    = color.NRGBA{R: 0x7a, G: 0x84, B: 0x90, A: 0xff}
)

func stateColor(s kafka.State) color.NRGBA {
	switch s {
	case kafka.StateRunning:
		return green
	case kafka.StateStarting, kafka.StateStopping:
		return yellow
	default:
		return red
	}
}

// dashStateText is the plain (no-emoji) status for colored canvas text.
func dashStateText(s kafka.State) string {
	switch s {
	case kafka.StateRunning:
		return "Running"
	case kafka.StateStarting:
		return "Starting…"
	case kafka.StateStopping:
		return "Stopping…"
	default:
		return "Stopped"
	}
}

// stateText is the emoji status used by widget.Labels elsewhere.
func stateText(s kafka.State) string {
	switch s {
	case kafka.StateRunning:
		return "🟢 Running"
	case kafka.StateStarting:
		return "🟡 Starting..."
	case kafka.StateStopping:
		return "🟡 Stopping..."
	default:
		return "🔴 Stopped"
	}
}

// status indicator colors
var (
	green  = color.NRGBA{R: 0x3f, G: 0xb9, B: 0x50, A: 0xff}
	yellow = color.NRGBA{R: 0xe6, G: 0xb0, B: 0x3a, A: 0xff}
	red    = color.NRGBA{R: 0xe0, G: 0x4f, B: 0x4f, A: 0xff}
)

// startAutoRefresh runs fn every interval while the named page is the active
// content. It stops when the window content changes away from the page. A guard
// prevents more than one loop per page running at once.
func (u *AppUI) startAutoRefresh(name string, page fyne.CanvasObject, interval time.Duration, fn func()) {
	u.refreshMu.Lock()
	if u.refreshActive[name] {
		u.refreshMu.Unlock()
		go fn() // already looping; just refresh once now
		return
	}
	u.refreshActive[name] = true
	u.refreshMu.Unlock()

	go fn() // initial
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		defer func() {
			u.refreshMu.Lock()
			u.refreshActive[name] = false
			u.refreshMu.Unlock()
		}()
		for range t.C {
			if !u.isPageVisible(page) {
				return
			}
			fn()
		}
	}()
}

// isPageVisible reports whether page is the currently shown content page.
func (u *AppUI) isPageVisible(page fyne.CanvasObject) bool {
	if u.content == nil || len(u.content.Objects) == 0 {
		return false
	}
	return u.content.Objects[0] == page
}
