package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// runMetadataDelete confirms, stops the services if needed, then clears the
// configured Kafka data-log and app-log folders. This recovers from a broker
// that won't initialize due to corrupt logs/metadata.
func (u *AppUI) runMetadataDelete() {
	dirs := u.mgr.MetadataDirs()
	if len(dirs) == 0 {
		u.errorDialog(fmt.Errorf("no metadata folders configured.\n\nSet the Kafka data-log folder and log folder in Settings first"))
		return
	}
	msg := "This will permanently DELETE all files inside:\n\n• " +
		strings.Join(dirs, "\n• ") +
		"\n\nZooKeeper and the broker will be stopped first.\nUse this to recover when the broker won't start."

	u.confirm("Delete Kafka Metadata", msg, func() {
		go func() {
			fyne.Do(func() { u.toast("Stopping Kafka & releasing file locks…") })
			// Stop the services we track…
			_ = u.mgr.Broker.Stop()
			_ = u.mgr.ZooKeeper.Stop()
			// …and force-kill any lingering Kafka/ZooKeeper JVMs (started
			// externally or in a previous session) that still lock the files.
			_ = u.mgr.KillKafkaProcesses()
			for i := 0; i < 40 && !u.mgr.ServicesStopped(); i++ {
				time.Sleep(250 * time.Millisecond)
			}
			// give Windows a moment to release file handles after the kill
			time.Sleep(2 * time.Second)

			report, err := u.mgr.DeleteMetadata()
			fyne.Do(func() {
				if err != nil {
					u.errorDialog(fmt.Errorf("%s\n%v", report, err))
					return
				}
				u.showOutput("Metadata Delete — done", report)
				u.toast("Metadata cleared — you can start the broker again")
			})
		}()
	})
}

// suggestMetadataDelete is shown after the broker fails to start twice.
func (u *AppUI) suggestMetadataDelete() {
	d := dialog.NewConfirm("Broker failed to start",
		"The Kafka broker failed to start twice.\n\nThis is usually caused by corrupted Kafka logs/metadata.\n\nClear the Kafka metadata folders now and try starting again?",
		func(ok bool) {
			if ok {
				u.runMetadataDelete()
			}
		}, u.win)
	d.SetConfirmText("Clear Metadata")
	d.SetDismissText("Not now")
	d.Show()
}
