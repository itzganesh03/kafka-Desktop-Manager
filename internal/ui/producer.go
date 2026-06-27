package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildProducer builds the producer page.
func (u *AppUI) buildProducer() fyne.CanvasObject {
	topicSelect := widget.NewSelect(nil, nil)
	topicSelect.PlaceHolder = "Select a topic"
	if u.cfg.DefaultTopic != "" {
		topicSelect.SetSelected(u.cfg.DefaultTopic)
	}

	loadTopics := func(force bool) {
		go func() {
			if force {
				u.mgr.InvalidateTopicCache()
			}
			names, err := u.mgr.CachedTopics()
			if err != nil {
				return
			}
			fyne.Do(func() { topicSelect.Options = names; topicSelect.Refresh() })
		}()
	}
	loadTopics(false)

	editor := widget.NewMultiLineEntry()
	editor.SetPlaceHolder("Paste or type a message. Valid JSON is sent as one record; otherwise each non-empty line is a separate message.")
	editor.SetMinRowsVisible(10)
	editor.Wrapping = fyne.TextWrapOff // code-like view so indented JSON stays aligned

	singleMsg := widget.NewCheck("Force: send entire editor as one message", nil)

	history := []string{}
	historyList := widget.NewList(
		func() int { return len(history) },
		func() fyne.CanvasObject { return widget.NewLabel("item") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(truncate(history[i], 80))
		},
	)
	historyList.OnSelected = func(i widget.ListItemID) {
		editor.SetText(history[i])
		historyList.UnselectAll()
	}

	prettyBtn := widget.NewButtonWithIcon("Pretty", theme.DocumentCreateIcon(), func() {
		formatted, ok := tryFormatJSON(editor.Text)
		if !ok {
			u.toast("Not valid JSON")
			return
		}
		editor.SetText(formatted)
	})
	minifyBtn := widget.NewButtonWithIcon("Minify", theme.MenuDropDownIcon(), func() {
		compact, ok := minifyJSON(editor.Text)
		if !ok {
			u.toast("Not valid JSON")
			return
		}
		editor.SetText(compact)
	})
	clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() { editor.SetText("") })

	sendBtn := widget.NewButtonWithIcon("Send", theme.MailSendIcon(), func() {
		topic := topicSelect.Selected
		if topic == "" {
			u.toast("Select a topic first")
			return
		}
		text := editor.Text
		if strings.TrimSpace(text) == "" {
			u.toast("Nothing to send")
			return
		}
		var msgs []string
		switch {
		case singleMsg.Checked:
			msgs = []string{text}
		case isSingleJSON(text):
			// A complete JSON document (possibly pretty-printed across many
			// lines) must be sent as ONE record, minified to a single line.
			if compact, ok := minifyJSON(text); ok {
				msgs = []string{compact}
			} else {
				msgs = []string{text}
			}
		default:
			// Plain text: one record per non-empty line.
			for _, line := range strings.Split(text, "\n") {
				if strings.TrimSpace(line) != "" {
					msgs = append(msgs, line)
				}
			}
		}
		go func() {
			_, err := u.mgr.SendMessages(topic, msgs)
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(err)
					return
				}
				history = append([]string{text}, history...)
				historyList.Refresh()
				u.toast(fmt.Sprintf("Sent %s to %s", plural(len(msgs), "record"), topic))
			})
		}()
	})
	sendBtn.Importance = widget.HighImportance

	controls := container.NewHBox(sendBtn, prettyBtn, minifyBtn, clearBtn)
	top := container.NewVBox(
		sectionTitle("Producer", theme.MailSendIcon()),
		container.NewBorder(nil, nil, widget.NewLabel("Topic:"), widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() { loadTopics(true) }), topicSelect),
		singleMsg,
		controls,
		widget.NewSeparator(),
	)

	left := container.NewBorder(top, nil, nil, nil, container.NewScroll(editor))
	right := card("History", container.NewBorder(nil, nil, nil, nil, historyList))

	split := container.NewHSplit(left, right)
	split.SetOffset(0.7)
	return split
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// tryFormatJSON indents valid JSON (preserving key order), reporting success.
func tryFormatJSON(s string) (string, bool) {
	t := strings.TrimSpace(s)
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(t), "", "  "); err != nil {
		return "", false
	}
	return buf.String(), true
}

// minifyJSON compacts valid JSON to a single line, reporting success.
func minifyJSON(s string) (string, bool) {
	t := strings.TrimSpace(s)
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(t)); err != nil {
		return "", false
	}
	return buf.String(), true
}

// isSingleJSON reports whether the whole text is one valid JSON value.
func isSingleJSON(s string) bool {
	return json.Valid([]byte(strings.TrimSpace(s)))
}
