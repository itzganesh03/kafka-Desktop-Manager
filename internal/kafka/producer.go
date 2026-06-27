package kafka

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const producerTool = "kafka-console-producer.bat"

// SendMessages publishes one or more messages to a topic via the console
// producer. Each element of messages becomes one Kafka record.
func (m *Manager) SendMessages(topic string, messages []string) (string, error) {
	if strings.TrimSpace(topic) == "" {
		return "", fmt.Errorf("topic is required")
	}
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages to send")
	}

	args := []string{"--bootstrap-server", m.bootstrap(), "--topic", topic}
	m.history.Add(CommandRecord{
		Display: formatCommand(producerTool, args),
		Tool:    m.tool(producerTool),
		Args:    args,
		When:    time.Now(),
	})

	cmd := exec.Command(m.tool(producerTool), args...)
	cmd.Dir = m.Config().KafkaPath
	cmd.SysProcAttr = hiddenWindow()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return "", err
	}

	// The console producer treats each newline-terminated line as a record.
	payload := strings.Join(messages, "\n") + "\n"
	if _, err := stdin.Write([]byte(payload)); err != nil {
		_ = stdin.Close()
		return out.String(), err
	}
	_ = stdin.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-time.After(20 * time.Second):
		_ = killTree(cmd.Process.Pid)
		err = fmt.Errorf("producer timed out")
	}
	if err != nil {
		return out.String(), fmt.Errorf("send failed: %w\n%s", err, out.String())
	}
	return out.String(), nil
}
