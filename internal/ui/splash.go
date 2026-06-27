package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// splashDuration is how long the animated splash is shown before the main
// window appears.
const splashDuration = 5 * time.Second

// showSplash displays a borderless animated splash window for splashDuration,
// then closes it and invokes onDone (which should reveal the main window).
// If the driver doesn't support splash windows, onDone is called immediately.
func showSplash(a fyne.App, onDone func()) {
	drv, ok := a.Driver().(desktop.Driver)
	if !ok {
		onDone()
		return
	}
	w := drv.CreateSplashWindow()
	stop := make(chan struct{})

	// Background.
	bg := canvas.NewRectangle(color.NRGBA{R: 0x0f, G: 0x14, B: 0x1a, A: 0xff})
	accentBar := canvas.NewRectangle(accentTeal)
	accentBar.SetMinSize(fyne.NewSize(0, 4))

	// Pulsing logo.
	logo := canvas.NewImageFromResource(appIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.Resize(fyne.NewSize(96, 96))
	logo.Move(fyne.NewPos(12, 12))
	logoBox := container.NewWithoutLayout(logo)
	logoWrap := container.NewGridWrap(fyne.NewSize(120, 120), logoBox)

	logoAnim := fyne.NewAnimation(1100*time.Millisecond, func(f float32) {
		s := 92 + 16*f
		logo.Resize(fyne.NewSize(s, s))
		logo.Move(fyne.NewPos((120-s)/2, (120-s)/2))
	})
	logoAnim.AutoReverse = true
	logoAnim.RepeatCount = fyne.AnimationRepeatForever
	logoAnim.Curve = fyne.AnimationEaseInOut
	logoAnim.Start()

	// Title (fades in).
	title := canvas.NewText("One Way Kafka Manager", color.NRGBA{R: 0x2e, G: 0xb8, B: 0xa6, A: 0x00})
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 26
	title.Alignment = fyne.TextAlignCenter
	titleAnim := canvas.NewColorRGBAAnimation(
		color.NRGBA{R: 0x2e, G: 0xb8, B: 0xa6, A: 0x00},
		color.NRGBA{R: 0x36, G: 0xd0, B: 0xbb, A: 0xff},
		900*time.Millisecond, func(c color.Color) { title.Color = c; title.Refresh() })
	titleAnim.Curve = fyne.AnimationEaseIn
	titleAnim.Start()

	subtitle := canvas.NewText("Initializing workspace…", color.NRGBA{R: 0x8a, G: 0x92, B: 0x9c, A: 0xff})
	subtitle.TextSize = 13
	subtitle.Alignment = fyne.TextAlignCenter

	// Chasing "one-way flow" dots.
	const nDots = 5
	dim := color.NRGBA{R: 0x2e, G: 0xb8, B: 0xa6, A: 0x40}
	bright := color.NRGBA{R: 0x4f, G: 0xe6, B: 0xd2, A: 0xff}
	dots := make([]*canvas.Circle, nDots)
	dotObjs := make([]fyne.CanvasObject, nDots)
	for i := range dots {
		c := canvas.NewCircle(dim)
		dots[i] = c
		dotObjs[i] = container.NewGridWrap(fyne.NewSize(11, 11), c)
	}
	dotRow := container.NewHBox(dotObjs...)

	// Determinate progress filling across splashDuration.
	progress := widget.NewProgressBar()
	progress.TextFormatter = func() string { return "" }
	progressWrap := container.NewGridWrap(fyne.NewSize(360, 12), progress)
	progAnim := fyne.NewAnimation(splashDuration, func(f float32) { progress.SetValue(float64(f)) })
	progAnim.Curve = fyne.AnimationLinear
	progAnim.Start()

	body := container.NewVBox(
		container.NewCenter(logoWrap),
		container.NewCenter(title),
		container.NewCenter(subtitle),
		widget.NewLabel(""),
		container.NewCenter(dotRow),
		container.NewCenter(progressWrap),
	)
	content := container.NewStack(bg,
		container.NewBorder(accentBar, nil, nil, nil, container.NewCenter(body)))

	w.SetContent(content)
	w.Resize(fyne.NewSize(540, 360))
	w.CenterOnScreen()
	w.Show()

	// Dot chase animation.
	go func() {
		t := time.NewTicker(150 * time.Millisecond)
		defer t.Stop()
		i := 0
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				idx := i % nDots
				fyne.Do(func() {
					for j, d := range dots {
						if j == idx {
							d.FillColor = bright
						} else {
							d.FillColor = dim
						}
						d.Refresh()
					}
				})
				i++
			}
		}
	}()

	// Cycling subtitle messages.
	go func() {
		msgs := []string{
			"Loading configuration…",
			"Preparing Kafka tools…",
			"Warming up consoles…",
			"Almost ready…",
		}
		t := time.NewTicker(1100 * time.Millisecond)
		defer t.Stop()
		i := 0
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				m := msgs[i%len(msgs)]
				fyne.Do(func() { subtitle.Text = m; subtitle.Refresh() })
				i++
			}
		}
	}()

	// Close after the duration and reveal the main window.
	go func() {
		time.Sleep(splashDuration)
		close(stop)
		logoAnim.Stop()
		fyne.Do(func() {
			w.Close()
			onDone()
		})
	}()
}
