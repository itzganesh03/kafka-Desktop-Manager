package kafka

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

const consumerTool = "kafka-console-consumer.bat"

// Consumer wraps a streaming kafka-console-consumer process.
type Consumer struct {
	mgr   *Manager
	topic string

	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	paused  bool

	onMessage func(string)
	onStop    func()
}

// NewConsumer creates a consumer for a topic. onMessage receives each record;
// onStop (optional) is called when the process exits.
func (m *Manager) NewConsumer(topic string, onMessage func(string), onStop func()) *Consumer {
	return &Consumer{mgr: m, topic: topic, onMessage: onMessage, onStop: onStop}
}

// Running reports whether the consumer process is active.
func (c *Consumer) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Paused reports whether message delivery is paused.
func (c *Consumer) Paused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// Start launches the consumer. fromBeginning controls --from-beginning.
func (c *Consumer) Start(fromBeginning bool) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("consumer already running")
	}
	c.mu.Unlock()

	args := []string{"--bootstrap-server", c.mgr.bootstrap(), "--topic", c.topic}
	if fromBeginning {
		args = append(args, "--from-beginning")
	}
	c.mgr.history.Add(CommandRecord{
		Display: formatCommand(consumerTool, args),
		Tool:    c.mgr.tool(consumerTool),
		Args:    args,
		When:    time.Now(),
	})

	cmd := exec.Command(c.mgr.tool(consumerTool), args...)
	cmd.Dir = c.mgr.Config().KafkaPath
	cmd.SysProcAttr = hiddenWindow()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// Discard stderr (consumer prints progress there).
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	c.mu.Lock()
	c.cmd = cmd
	c.running = true
	c.paused = false
	c.mu.Unlock()

	go c.pump(stdout)
	go func() { _, _ = io.Copy(io.Discard, stderr) }()
	go func() {
		_ = cmd.Wait()
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		if c.onStop != nil {
			c.onStop()
		}
	}()
	return nil
}

func (c *Consumer) pump(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		c.mu.Lock()
		paused := c.paused
		cb := c.onMessage
		c.mu.Unlock()
		if !paused && cb != nil {
			cb(line)
		}
	}
}

// Pause stops forwarding messages to the callback (process keeps running).
func (c *Consumer) Pause() {
	c.mu.Lock()
	c.paused = true
	c.mu.Unlock()
}

// Resume resumes forwarding messages.
func (c *Consumer) Resume() {
	c.mu.Lock()
	c.paused = false
	c.mu.Unlock()
}

// Stop terminates the consumer process.
func (c *Consumer) Stop() error {
	c.mu.Lock()
	cmd := c.cmd
	running := c.running
	c.mu.Unlock()
	if !running || cmd == nil || cmd.Process == nil {
		return nil
	}
	return killTree(cmd.Process.Pid)
}
