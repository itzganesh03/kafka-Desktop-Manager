package kafka

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// KillKafkaProcesses force-terminates any running Kafka broker / ZooKeeper JVMs,
// even ones this app instance did not launch (e.g. started in a previous run or
// an external terminal). This releases the file locks that otherwise block
// Metadata Delete. It deliberately matches only Kafka/ZooKeeper java processes,
// not every java.exe on the machine.
func (m *Manager) KillKafkaProcesses() error {
	const ps = `Get-CimInstance Win32_Process -Filter "Name='java.exe'" | ` +
		`Where-Object { $_.CommandLine -match 'kafka\.Kafka|QuorumPeerMain|kafka-server-start|zookeeper-server-start|server\.properties|zookeeper\.properties' } | ` +
		`ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = hiddenWindow()
	return cmd.Run()
}

// MetadataDirs returns the configured Kafka data-log and app-log directories
// that exist (non-empty config values).
func (m *Manager) MetadataDirs() []string {
	cfg := m.Config()
	var dirs []string
	for _, d := range []string{cfg.DataLogDir, cfg.AppLogDir} {
		if strings.TrimSpace(d) != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// ServicesStopped reports whether both ZooKeeper and the broker are stopped.
func (m *Manager) ServicesStopped() bool {
	return m.ZooKeeper.State() == StateStopped && m.Broker.State() == StateStopped
}

// DeleteMetadata clears the contents of the configured data-log and app-log
// directories (a common fix when the broker won't initialize due to corrupt
// logs). ZooKeeper and the broker MUST be stopped first, or files are locked.
// Returns a human-readable report.
func (m *Manager) DeleteMetadata() (string, error) {
	if !m.ServicesStopped() {
		return "", fmt.Errorf("stop ZooKeeper and the Kafka broker before deleting metadata")
	}
	dirs := m.MetadataDirs()
	if len(dirs) == 0 {
		return "", fmt.Errorf("no metadata folders configured — set the data-log and log folders in Settings")
	}
	var report strings.Builder
	cleared := false
	for _, d := range dirs {
		info, err := os.Stat(d)
		if err != nil {
			if os.IsNotExist(err) {
				// A wrong/old path shouldn't abort the whole operation.
				fmt.Fprintf(&report, "⚠ %s: folder not found — skipped\n", d)
				continue
			}
			fmt.Fprintf(&report, "✗ %s: %v\n", d, err)
			return report.String(), fmt.Errorf("failed accessing %s: %w", d, err)
		}
		if !info.IsDir() {
			fmt.Fprintf(&report, "⚠ %s: not a folder — skipped\n", d)
			continue
		}
		n, err := clearDir(d)
		if err != nil {
			fmt.Fprintf(&report, "✗ %s: %v\n", d, err)
			return report.String(), fmt.Errorf("failed clearing %s: %w", d, err)
		}
		cleared = true
		fmt.Fprintf(&report, "✓ cleared %d item(s) from %s\n", n, d)
	}
	if !cleared {
		fmt.Fprint(&report, "\nNothing was cleared. Check the Data Log / Log folder paths in Settings.")
	}
	return report.String(), nil
}

// clearDir removes everything inside dir (but keeps dir itself). It retries a
// few times because Windows may briefly hold file handles after a process exits.
func clearDir(dir string) (int, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("not a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		var rmErr error
		for attempt := 0; attempt < 6; attempt++ {
			if rmErr = os.RemoveAll(p); rmErr == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if rmErr != nil {
			return removed, fmt.Errorf("%s still locked (is a service running?): %w", e.Name(), rmErr)
		}
		removed++
	}
	return removed, nil
}
