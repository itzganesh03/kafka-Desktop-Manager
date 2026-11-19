package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// runMetadataDelete confirms, stops the services if needed, then clears the
// configured Kafka data-log and app-log folders. This recovers from a broker
// that won't initialize due to corrupt logs/metadata.
func (u *AppUI) runMetadataDelete() {
	if u.cfg.IsDocker() {
		u.runDockerReset()
		return
	}
	dirs := u.mgr.MetadataDirs()
	if len(dirs) == 0 {
		u.errorDialog(fmt.Errorf("no metadata folders configured.\n\nSet the Kafka data-log folder and log folder in Settings first"))
		return
	}
	msg := "This will permanently DELETE all files inside:\n\n• " +
		strings.Join(dirs, "\n• ") +
		"\n\nZooKeeper and the broker will be stopped first.\nUse this to recover when the broker won't start."

	u.confirm("Delete Kafka Metadata", msg, func() {
		// Show a non-blocking progress dialog so it never looks "hung".
		status := widget.NewLabel("Stopping ZooKeeper & Kafka broker…")
		prog := dialog.NewCustomWithoutButtons("Clearing Kafka metadata",
			container.NewVBox(widget.NewProgressBarInfinite(), status), u.win)
		prog.Show()

		go func() {
			// 1. Kill broker + ZooKeeper immediately (tracked PIDs + any stray
			//    JVMs). KillKafkaProcesses has its own hard timeout.
			_ = u.mgr.KillKafkaProcesses()

			// 2. Brief wait for Windows to release the file handles.
			fyne.Do(func() { status.SetText("Releasing file locks…") })
			time.Sleep(1500 * time.Millisecond)

			// 3. Delete (retries any briefly-locked files internally).
			fyne.Do(func() { status.SetText("Deleting metadata files…") })
			report, err := u.mgr.DeleteMetadata()

			fyne.Do(func() {
				prog.Hide()
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

// runDockerReset is the Docker-mode equivalent of Metadata Delete: it runs
// `docker compose down -v`, removing the container and its kafka_data volume
// (all topics and messages). Start the broker again to get a fresh Kafka.
func (u *AppUI) runDockerReset() {
	msg := "This runs `docker compose down -v` in " + u.cfg.DockerDirOrDefault() + ".\n\n" +
		"The Kafka container is removed and its data volume is DELETED\n(all topics and messages are lost).\n\n" +
		"Press Start on the broker afterwards to create a fresh Kafka."
	u.confirm("Reset Docker Kafka Data", msg, func() {
		status := widget.NewLabel("Removing container and data volume…")
		prog := dialog.NewCustomWithoutButtons("Resetting Docker Kafka",
			container.NewVBox(widget.NewProgressBarInfinite(), status), u.win)
		prog.Show()
		go func() {
			var out strings.Builder
			err := u.mgr.ResetDockerData(func(line string) { out.WriteString(line + "\n") })
			u.mgr.InvalidateTopicCache()
			fyne.Do(func() {
				prog.Hide()
				if err != nil {
					u.errorDialog(fmt.Errorf("%s\n%v", out.String(), err))
					return
				}
				u.showOutput("Reset Docker Data — done", out.String())
				u.toast("Docker data cleared — you can start the broker again")
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
