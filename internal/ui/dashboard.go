package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/kafka"
)

// metricCard is a glass stat tile: a tinted icon tile on the left, then a
// small caption, a large value (with optional status dot) and a sub-line.
type metricCard struct {
	root  fyne.CanvasObject
	tile  *iconTile
	value *canvas.Text
	sub   *canvas.Text
	dot   *canvas.Circle
	pulse *fyne.Animation
}

func newMetricCard(title string, icon fyne.Resource, accentName fyne.ThemeColorName, big, withDot bool) *metricCard {
	tile := newIconTile(icon, accentName, 54)

	titleTxt := themedText(title, true)
	titleTxt.TextStyle = fyne.TextStyle{Bold: true}
	titleTxt.TextSize = 11

	value := canvas.NewText("—", pal.text)
	value.TextStyle = fyne.TextStyle{Bold: true}
	if big {
		value.TextSize = 28
	} else {
		value.TextSize = 18
	}

	sub := themedText("", true)
	sub.TextSize = 11

	mc := &metricCard{tile: tile, value: value, sub: sub}

	// Values use either a text color (follow the theme) or a state color (keep).
	paint(func() {
		switch value.Color {
		case color.Color(darkPalette.text), color.Color(lightPalette.text):
			value.Color = pal.text
		case color.Color(darkPalette.muted), color.Color(lightPalette.muted):
			value.Color = pal.muted
		}
		value.Refresh()
	})

	var valueRow fyne.CanvasObject = value
	if withDot {
		mc.dot = canvas.NewCircle(red)
		dotWrap := container.NewGridWrap(fyne.NewSize(10, 10), mc.dot)
		valueRow = container.NewHBox(container.NewCenter(dotWrap), value)
	}

	text := vCentered(container.NewVBox(titleTxt, valueRow, sub))
	body := container.NewBorder(nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 14), vCentered(tile.root)), nil, text)
	minH := canvas.NewRectangle(color.Transparent)
	minH.SetMinSize(fyne.NewSize(0, 78))
	mc.root = glassPanel(container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4),
		container.NewStack(minH, body)))
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

func (m *metricCard) setAccent(name fyne.ThemeColorName) {
	m.tile.setAccent(name)
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
	installCard := newMetricCard("KAFKA INSTALLATION", theme.FolderIcon(), colorTeal, false, false)
	zkCard := newMetricCard("ZOOKEEPER", theme.StorageIcon(), colorRed, false, true)
	brokerCard := newMetricCard("KAFKA BROKER", theme.ComputerIcon(), colorRed, false, true)
	addrCard := newMetricCard("BROKER ADDRESS", theme.MailSendIcon(), colorBlue, false, false)
	topicsCard := newMetricCard("TOPICS", theme.ListIcon(), colorPurple, true, false)
	groupsCard := newMetricCard("CONSUMER GROUPS", theme.AccountIcon(), colorOrange, true, false)

	if u.cfg.IsDocker() {
		installCard.setValue("Docker · "+kafka.DockerContainer, textPrimary)
	} else {
		installCard.setValue(shortPath(u.cfg.KafkaPath), textPrimary)
	}
	addrCard.setValue(u.cfg.BootstrapServer, textPrimary)

	grid := container.NewGridWithColumns(3,
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

	// fetchLag runs the describe synchronously and applies the result. Used by
	// the 1-second auto-refresh loop so calls never overlap.
	fetchLag := func(group string) {
		if group == "" {
			return
		}
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
	}
	// describeLag is the async wrapper for manual actions (select / button).
	describeLag := func(group string) {
		if group == "" {
			return
		}
		go fetchLag(group)
	}
	groupSelect.OnChanged = describeLag

	autoCheck := widget.NewCheck("Auto 1s", nil)
	var autoStop chan struct{}

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
		container.NewHBox(autoCheck, totalLagLbl), groupSelect)
	lagScroll := container.NewScroll(lagTable)
	lagScroll.SetMinSize(fyne.NewSize(0, 220))
	lagPanel := card("Consumer Group Lag", container.NewBorder(
		container.NewVBox(lagControls, widget.NewSeparator()), nil, nil, nil, lagScroll))

	// applyService updates a service card from its (in-memory, cheap) state.
	applyService := func(c *metricCard, st kafka.State) {
		col := stateColor(st)
		fyne.Do(func() {
			c.setAccent(stateAccent(st))
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
		docker := u.cfg.IsDocker()
		if docker {
			fyne.Do(func() {
				zkCard.setAccent(colorBlue)
				zkCard.setValue("Not used", textMuted)
				zkCard.setSub("KRaft mode (Docker)")
				zkCard.stopPulse(accent(colorBlue))
			})
		} else {
			applyService(zkCard, u.mgr.ZooKeeper.State())
		}
		applyService(brokerCard, u.mgr.Broker.State())
		fyne.Do(func() {
			if docker {
				installCard.setValue("Docker · "+kafka.DockerContainer, textPrimary)
				installCard.setSub(u.cfg.DockerImageOrDefault())
			} else {
				installCard.setValue(shortPath(u.cfg.KafkaPath), textPrimary)
				installCard.setSub("")
			}
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
	page := container.NewBorder(header, nil, nil, nil, container.NewVScroll(content))

	// Auto-refresh the lag panel ~every second (like `watch -n1`). Runs the
	// describe sequentially so JVM calls never overlap, and pauses while the
	// dashboard isn't visible. Toggled by the "Auto 1s" checkbox.
	autoCheck.OnChanged = func(on bool) {
		if !on {
			if autoStop != nil {
				close(autoStop)
				autoStop = nil
			}
			return
		}
		if autoStop != nil {
			return // already running
		}
		autoStop = make(chan struct{})
		stop := autoStop
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				if g := groupSelect.Selected; g != "" && u.isPageVisible(page) {
					fetchLag(g) // synchronous — no overlap
				}
				select {
				case <-stop:
					return
				case <-time.After(1 * time.Second):
				}
			}
		}()
	}

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

// text colors for hand-drawn canvas text; applyTheme sets them from pal.
var (
	textPrimary = darkPalette.text
	textMuted   = darkPalette.muted
)

func stateColor(s kafka.State) color.NRGBA { return accent(stateAccent(s)) }

// stateAccent maps a service state to its accent color name.
func stateAccent(s kafka.State) fyne.ThemeColorName {
	switch s {
	case kafka.StateRunning:
		return colorGreen
	case kafka.StateStarting, kafka.StateStopping:
		return colorYellow
	default:
		return colorRed
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
	green  = accentColors[colorGreen]
	yellow = accentColors[colorYellow]
	red    = accentColors[colorRed]
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
