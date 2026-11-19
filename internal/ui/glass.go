package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Fyne cannot blur what is behind a widget, so the "glass" look is built from
// a softly lit gradient backdrop plus translucent, hairline-bordered panels.

const panelRadius = 16

// repaints re-color hand-drawn canvas objects, which (unlike widgets) don't
// follow the Fyne theme by themselves. Switching theme runs them all, so pages
// keep their state instead of being rebuilt.
var repaints []func()

// paint applies fn now and again after every theme switch.
func paint(fn func()) {
	fn()
	repaints = append(repaints, fn)
}

// themedText is canvas text colored from the palette (muted or primary).
func themedText(s string, muted bool) *canvas.Text {
	t := canvas.NewText(s, nil)
	paint(func() {
		t.Color = pal.text
		if muted {
			t.Color = pal.muted
		}
		t.Refresh()
	})
	return t
}

// glassBackdrop is the full-window gradient with two soft colored glows.
func glassBackdrop() fyne.CanvasObject {
	base := canvas.NewLinearGradient(nil, nil, 135)

	glowA := canvas.NewRadialGradient(nil, color.Transparent)
	glowA.CenterOffsetX, glowA.CenterOffsetY = -0.35, -0.45

	glowB := canvas.NewRadialGradient(nil, color.Transparent)
	glowB.CenterOffsetX, glowB.CenterOffsetY = 0.4, 0.45

	paint(func() {
		base.StartColor, base.EndColor = pal.bgTop, pal.bgBottom
		glowA.StartColor = pal.glowA
		glowB.StartColor = pal.glowB
		base.Refresh()
		glowA.Refresh()
		glowB.Refresh()
	})
	return container.NewStack(base, glowA, glowB)
}

// glassPanel wraps content in a translucent rounded panel with a faux shadow.
func glassPanel(content fyne.CanvasObject) fyne.CanvasObject {
	shadow := canvas.NewRectangle(nil)
	shadow.CornerRadius = panelRadius

	bg := canvas.NewRectangle(nil)
	bg.CornerRadius = panelRadius
	bg.StrokeWidth = 1

	paint(func() {
		shadow.FillColor = pal.shadow
		bg.FillColor, bg.StrokeColor = pal.glass, pal.glassBorder
		shadow.Refresh()
		bg.Refresh()
	})

	inner := container.New(padAll(10), content)
	return container.New(&shadowLayout{}, shadow, bg, inner)
}

// shadowLayout stacks its objects, offsetting the first (the shadow) a few
// pixels down so it peeks out below the panel edge.
type shadowLayout struct{}

func (*shadowLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for i, o := range objs {
		if i == 0 {
			o.Move(fyne.NewPos(2, 4))
			o.Resize(fyne.NewSize(size.Width-4, size.Height-1))
			continue
		}
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

func (*shadowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objs[1:] {
		min = min.Max(o.MinSize())
	}
	return min
}

// padAll returns an even padded layout of p pixels on every side.
func padAll(p float32) fyne.Layout { return &evenPad{p} }

type evenPad struct{ p float32 }

func (e *evenPad) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(e.p, e.p))
		o.Resize(fyne.NewSize(size.Width-2*e.p, size.Height-2*e.p))
	}
}

func (e *evenPad) MinSize(objs []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objs {
		min = min.Max(o.MinSize())
	}
	return min.Add(fyne.NewSize(2*e.p, 2*e.p))
}

// vcenter places each object full-width, vertically centered at its min height.
type vcenter struct{}

func (vcenter) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, (size.Height-h)/2))
		o.Resize(fyne.NewSize(size.Width, h))
	}
}

func (vcenter) MinSize(objs []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objs {
		min = min.Max(o.MinSize())
	}
	return min
}

func vCentered(o fyne.CanvasObject) fyne.CanvasObject { return container.New(vcenter{}, o) }

// iconTile is a rounded tinted square holding an accent-colored icon.
type iconTile struct {
	root fyne.CanvasObject
	bg   *canvas.Rectangle
	icon *widget.Icon
	src  fyne.Resource
}

func newIconTile(res fyne.Resource, name fyne.ThemeColorName, size float32) *iconTile {
	t := &iconTile{src: res}
	t.bg = canvas.NewRectangle(color.Transparent)
	t.bg.CornerRadius = size / 4
	t.bg.SetMinSize(fyne.NewSize(size, size))
	t.icon = widget.NewIcon(res)
	iconBox := container.NewCenter(container.NewGridWrap(fyne.NewSize(size*0.5, size*0.5), t.icon))
	t.root = container.NewStack(t.bg, iconBox)
	t.setAccent(name)
	return t
}

func (t *iconTile) setAccent(name fyne.ThemeColorName) {
	t.bg.FillColor = withAlpha(accent(name), 0x2e)
	t.bg.Refresh()
	t.icon.SetResource(theme.NewColoredResource(t.src, name))
}

// statusPill is a small rounded badge: colored dot + label.
type statusPill struct {
	root fyne.CanvasObject
	bg   *canvas.Rectangle
	dot  *canvas.Circle
	text *canvas.Text
}

func newStatusPill(label string) *statusPill {
	p := &statusPill{}
	p.bg = canvas.NewRectangle(color.Transparent)
	p.bg.CornerRadius = 13
	p.bg.StrokeWidth = 1
	p.dot = canvas.NewCircle(color.Transparent)
	p.text = canvas.NewText(label, pal.text)
	p.text.TextSize = 12
	p.text.TextStyle = fyne.TextStyle{Bold: true}
	dot := container.NewCenter(container.NewGridWrap(fyne.NewSize(8, 8), p.dot))
	row := container.NewHBox(dot, vCentered(p.text))
	p.root = container.NewStack(p.bg, container.New(layout.NewCustomPaddedLayout(4, 4, 10, 12), row))
	return p
}

// set updates the label and recolors the pill around c.
func (p *statusPill) set(label string, c color.NRGBA) {
	p.text.Text = label
	p.text.Color = c
	p.text.Refresh()
	p.dot.FillColor = c
	p.dot.Refresh()
	p.bg.FillColor = withAlpha(c, 0x1f)
	p.bg.StrokeColor = withAlpha(c, 0x55)
	p.bg.Refresh()
}

// navButton is a sidebar entry: a rounded pill that turns solid primary when
// selected and tints on hover.
type navButton struct {
	widget.BaseWidget
	bg       *canvas.Rectangle
	icon     *widget.Icon
	src      fyne.Resource
	label    *canvas.Text
	selected bool
	hovered  bool
	onTap    func()
}

func newNavButton(icon fyne.Resource, title string, onTap func()) *navButton {
	b := &navButton{src: icon, onTap: onTap}
	b.ExtendBaseWidget(b)
	b.bg = canvas.NewRectangle(color.Transparent)
	b.bg.CornerRadius = 10
	b.icon = widget.NewIcon(icon)
	b.label = canvas.NewText(title, pal.text)
	b.label.TextSize = 13.5
	paint(b.apply)
	return b
}

func (b *navButton) CreateRenderer() fyne.WidgetRenderer {
	ic := container.NewGridWrap(fyne.NewSize(18, 18), b.icon)
	row := container.NewBorder(nil, nil, vCentered(ic), nil, vCentered(b.label))
	return widget.NewSimpleRenderer(container.NewStack(b.bg,
		container.New(layout.NewCustomPaddedLayout(9, 9, 12, 12), row)))
}

func (b *navButton) setSelected(on bool) {
	b.selected = on
	b.apply()
}

func (b *navButton) apply() {
	switch {
	case b.selected:
		b.bg.FillColor = pal.primary
		b.label.Color = color.White
		b.label.TextStyle = fyne.TextStyle{Bold: true}
		b.icon.SetResource(theme.NewColoredResource(b.src, theme.ColorNameForegroundOnPrimary))
	case b.hovered:
		b.bg.FillColor = pal.hover
		b.label.Color = pal.text
		b.label.TextStyle = fyne.TextStyle{}
		b.icon.SetResource(theme.NewColoredResource(b.src, theme.ColorNamePrimary))
	default:
		b.bg.FillColor = color.Transparent
		b.label.Color = pal.text
		b.label.TextStyle = fyne.TextStyle{}
		b.icon.SetResource(theme.NewColoredResource(b.src, theme.ColorNamePlaceHolder))
	}
	b.bg.Refresh()
	b.label.Refresh()
}

func (b *navButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *navButton) MouseIn(*desktop.MouseEvent)    { b.hovered = true; b.apply() }
func (b *navButton) MouseMoved(*desktop.MouseEvent) {}
func (b *navButton) MouseOut()                      { b.hovered = false; b.apply() }
func (b *navButton) Cursor() desktop.Cursor         { return desktop.PointerCursor }

// navBar is the vertical sidebar menu; Select mirrors widget.List.Select.
type navBar struct {
	root       fyne.CanvasObject
	buttons    []*navButton
	selected   int
	OnSelected func(int)
}

func newNavBar(items []navItem) *navBar {
	n := &navBar{selected: -1}
	box := container.NewVBox()
	for i, it := range items {
		idx := i
		b := newNavButton(it.icon, it.title, func() { n.Select(idx) })
		n.buttons = append(n.buttons, b)
		box.Add(b)
	}
	n.root = box
	return n
}

func (n *navBar) Select(i int) {
	if i < 0 || i >= len(n.buttons) {
		return
	}
	for j, b := range n.buttons {
		b.setSelected(j == i)
	}
	n.selected = i
	if n.OnSelected != nil {
		n.OnSelected(i)
	}
}
