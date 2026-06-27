package ui

import (
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"kafkadesktop/internal/kafka"
)

// caps for the consumer view. These keep the Fyne text widget responsive: it
// becomes very slow with large text, so we bound both how many messages and how
// many characters are rendered at once. The full payloads stay in the buffer
// for Copy/Export.
const (
	maxConsumerBuffer    = 5000   // messages retained in memory
	maxConsumerShown     = 250    // most-recent messages considered for display
	maxDisplayLineLen    = 2000   // per-message display cap (chars)
	maxTotalDisplayChars = 120000 // total rendered text cap (chars)
	maxPrettyLen         = 20000  // only pretty-print JSON below this size
)

// buildConsumer builds the consumer page.
func (u *AppUI) buildConsumer() fyne.CanvasObject {
	var (
		mu         sync.Mutex
		messages   []string
		consumer   *kafka.Consumer
		autoFollow = true
		pretty     = true
		dirty      = false
		total      int
	)

	topicSelect := widget.NewSelect(nil, nil)
	topicSelect.PlaceHolder = "Select a topic"
	if u.cfg.DefaultTopic != "" {
		topicSelect.SetSelected(u.cfg.DefaultTopic)
	}
	go func() {
		names, err := u.mgr.CachedTopics()
		if err == nil {
			fyne.Do(func() { topicSelect.Options = names; topicSelect.Refresh() })
		}
	}()

	fromBeginning := widget.NewCheck("From beginning", nil)
	search := widget.NewEntry()
	search.SetPlaceHolder("Filter messages...")

	output := widget.NewMultiLineEntry()
	output.Wrapping = fyne.TextWrapWord
	output.SetMinRowsVisible(12)

	statusLbl := widget.NewLabel("🔴 Stopped")
	countLbl := widget.NewLabel("0 messages")

	markDirty := func() {
		mu.Lock()
		dirty = true
		mu.Unlock()
	}

	// flush re-renders the output if dirty (called on a throttled ticker).
	flush := func() {
		mu.Lock()
		if !dirty {
			mu.Unlock()
			return
		}
		dirty = false
		q := strings.ToLower(strings.TrimSpace(search.Text))
		follow := autoFollow
		pp := pretty
		// Render only the most recent window of messages.
		start := 0
		if len(messages) > maxConsumerShown {
			start = len(messages) - maxConsumerShown
		}
		window := messages[start:]
		// Build newest-first so the total-char cap keeps the most recent
		// messages, then reverse for display.
		var parts []string
		size := 0
		for i := len(window) - 1; i >= 0; i-- {
			m := window[i]
			if q != "" && !strings.Contains(strings.ToLower(m), q) {
				continue
			}
			disp := sanitizeMessage(m, pp)
			if size+len(disp) > maxTotalDisplayChars {
				parts = append(parts, "… (older messages hidden — use Export for the full log)")
				break
			}
			size += len(disp)
			parts = append(parts, disp)
		}
		// reverse
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		text := strings.Join(parts, "\n")
		mu.Unlock()

		if text == output.Text {
			return
		}
		output.SetText(text)
		if follow {
			output.CursorRow = strings.Count(text, "\n")
			output.Refresh()
		}
	}
	search.OnChanged = func(string) { markDirty() }

	startBtn := widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), nil)
	stopBtn := widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), nil)
	pauseBtn := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), nil)
	resumeBtn := widget.NewButtonWithIcon("Resume", theme.MediaPlayIcon(), nil)
	stopBtn.Disable()
	pauseBtn.Disable()
	resumeBtn.Disable()

	autoCheck := widget.NewCheck("Auto-scroll", func(b bool) {
		mu.Lock()
		autoFollow = b
		mu.Unlock()
	})
	autoCheck.SetChecked(true)

	prettyCheck := widget.NewCheck("Pretty JSON", func(b bool) {
		mu.Lock()
		pretty = b
		dirty = true
		mu.Unlock()
	})
	prettyCheck.SetChecked(true)

	clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
		mu.Lock()
		messages = nil
		total = 0
		dirty = true
		mu.Unlock()
		countLbl.SetText("0 messages")
		output.SetText("")
	})
	copyBtn := widget.NewButtonWithIcon("Copy All", theme.ContentCopyIcon(), func() {
		u.fyneApp.Clipboard().SetContent(output.Text)
		u.toast("Copied")
	})
	exportBtn := widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), func() {
		mu.Lock()
		text := strings.Join(messages, "\n")
		mu.Unlock()
		u.exportText("messages", text)
	})

	setRunning := func(running bool) {
		fyne.Do(func() {
			if running {
				statusLbl.SetText("🟢 Consuming")
				startBtn.Disable()
				stopBtn.Enable()
				pauseBtn.Enable()
				resumeBtn.Disable()
			} else {
				statusLbl.SetText("🔴 Stopped")
				startBtn.Enable()
				stopBtn.Disable()
				pauseBtn.Disable()
				resumeBtn.Disable()
			}
		})
	}

	startBtn.OnTapped = func() {
		topic := topicSelect.Selected
		if topic == "" {
			u.toast("Select a topic first")
			return
		}
		consumer = u.mgr.NewConsumer(topic,
			func(line string) {
				mu.Lock()
				messages = append(messages, line)
				if len(messages) > maxConsumerBuffer {
					messages = messages[len(messages)-maxConsumerBuffer:]
				}
				total++
				dirty = true
				mu.Unlock()
			},
			func() { setRunning(false) },
		)
		if err := consumer.Start(fromBeginning.Checked); err != nil {
			u.errorDialog(err)
			return
		}
		setRunning(true)
	}
	stopBtn.OnTapped = func() {
		if consumer != nil {
			_ = consumer.Stop()
		}
	}
	pauseBtn.OnTapped = func() {
		if consumer != nil {
			consumer.Pause()
			pauseBtn.Disable()
			resumeBtn.Enable()
			statusLbl.SetText("⏸ Paused")
		}
	}
	resumeBtn.OnTapped = func() {
		if consumer != nil {
			consumer.Resume()
			pauseBtn.Enable()
			resumeBtn.Disable()
			statusLbl.SetText("🟢 Consuming")
		}
	}

	topRow := container.NewBorder(nil, nil, widget.NewLabel("Topic:"), fromBeginning, topicSelect)
	controls := container.NewHBox(startBtn, stopBtn, pauseBtn, resumeBtn, autoCheck, prettyCheck, clearBtn, copyBtn, exportBtn)
	statusRow := container.NewHBox(statusLbl, widget.NewSeparator(), countLbl)

	header := container.NewVBox(
		sectionTitle("Consumer", theme.DownloadIcon()),
		topRow,
		controls,
		statusRow,
		search,
		widget.NewSeparator(),
	)
	page := container.NewBorder(header, nil, nil, nil, container.NewScroll(output))

	// Throttled render + count update (~4 fps) keeps the UI smooth at high
	// message rates instead of rebuilding the text on every record.
	u.onShow["consumer"] = func() {
		u.startAutoRefresh("consumer", page, 250*time.Millisecond, func() {
			mu.Lock()
			n := total
			d := dirty
			mu.Unlock()
			if !d {
				return
			}
			fyne.Do(func() {
				countLbl.SetText(plural(n, "message"))
				flush()
			})
		})
	}
	return page
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// sanitizeMessage prepares a single record for display: pretty-prints small
// JSON, renders binary payloads as a short hex preview (so the UI never tries
// to lay out megabytes of garbage), and truncates over-long text.
func sanitizeMessage(m string, pretty bool) string {
	if isBinaryString(m) {
		preview := hex.EncodeToString([]byte(m))
		if len(preview) > 64 {
			preview = preview[:64]
		}
		return "[binary/non-text • " + strconv.Itoa(len(m)) + " bytes] " + preview + "…"
	}
	if pretty && len(m) <= maxPrettyLen {
		if formatted, ok := tryFormatJSON(m); ok {
			m = formatted + "\n"
		}
	}
	if len(m) > maxDisplayLineLen {
		return m[:maxDisplayLineLen] + "… (truncated — use Export for full)"
	}
	return m
}

// isBinaryString reports whether s looks like non-text/binary data: invalid
// UTF-8, or more than ~5% control/replacement characters.
func isBinaryString(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	nonPrint, total := 0, 0
	for _, r := range s {
		total++
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0xFFFD {
			nonPrint++
		}
	}
	return total > 0 && nonPrint*20 > total
}
