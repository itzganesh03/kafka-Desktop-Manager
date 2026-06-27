// Package kafka wraps the Windows Kafka .bat command-line tools so the rest of
// the application can drive Kafka without spawning terminals manually.
package kafka

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kafkadesktop/internal/config"
)

// Manager is the central entry point for all Kafka operations. It is safe for
// concurrent use.
type Manager struct {
	mu  sync.RWMutex
	cfg *config.Config

	history *History

	// long-running services
	ZooKeeper *Service
	Broker    *Service
}

// NewManager creates a Manager bound to the given config.
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		cfg:     cfg,
		history: NewHistory(200),
	}
	m.ZooKeeper = newService("ZooKeeper", m)
	m.Broker = newService("Kafka Broker", m)
	return m
}

// Config returns the current configuration (read-locked copy of the pointer).
func (m *Manager) Config() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// SetConfig swaps the active configuration.
func (m *Manager) SetConfig(cfg *config.Config) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}

// History returns the command history store.
func (m *Manager) History() *History { return m.history }

// binDir returns <kafka_path>\bin\windows.
func (m *Manager) binDir() string {
	return filepath.Join(m.Config().KafkaPath, "bin", "windows")
}

// configDir returns <kafka_path>\config.
func (m *Manager) configDir() string {
	return filepath.Join(m.Config().KafkaPath, "config")
}

// tool returns the absolute path to a .bat tool in bin\windows.
func (m *Manager) tool(name string) string {
	return filepath.Join(m.binDir(), name)
}

// bootstrap returns the configured bootstrap server.
func (m *Manager) bootstrap() string {
	return m.Config().BootstrapServer
}

// CommandRecord is a single executed command for the history list.
type CommandRecord struct {
	Display string    // human-readable command line
	Tool    string    // the .bat tool path
	Args    []string  // arguments
	When    time.Time // execution time
}

// run executes a Kafka .bat tool with the given args, capturing combined
// output. The command is recorded in history. A timeout guards against hangs.
func (m *Manager) run(ctx context.Context, tool string, args ...string) (string, error) {
	full := m.tool(tool)
	display := formatCommand(tool, args)
	m.history.Add(CommandRecord{Display: display, Tool: full, Args: args, When: time.Now()})

	cmd := exec.CommandContext(ctx, full, args...)
	cmd.Dir = m.Config().KafkaPath
	cmd.SysProcAttr = hiddenWindow()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("%s failed: %w\n%s", tool, err, out.String())
	}
	return out.String(), nil
}

// runTimeout runs a tool with a default timeout (admin/list operations are
// expected to be quick).
func (m *Manager) runTimeout(tool string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return m.run(ctx, tool, args...)
}

// RunRecorded re-executes a previously recorded command. The long-running
// server scripts are refused here (use the Services page to control those).
func (m *Manager) RunRecorded(rec CommandRecord) (string, error) {
	base := filepath.Base(rec.Tool)
	if base == "zookeeper-server-start.bat" || base == "kafka-server-start.bat" {
		return "", fmt.Errorf("use the Services page to start %s", base)
	}
	if base == consumerTool {
		return "", fmt.Errorf("use the Consumer page to run a streaming consumer")
	}
	return m.runTimeout(base, rec.Args...)
}

// formatCommand builds a readable command line for display/history.
func formatCommand(tool string, args []string) string {
	var b strings.Builder
	b.WriteString(tool)
	for _, a := range args {
		b.WriteByte(' ')
		if strings.ContainsAny(a, " \t") {
			b.WriteByte('"')
			b.WriteString(a)
			b.WriteByte('"')
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}
