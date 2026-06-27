package kafka

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// State is the lifecycle state of a managed service.
type State int

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

func (s State) String() string {
	switch s {
	case StateStarting:
		return "Starting"
	case StateRunning:
		return "Running"
	case StateStopping:
		return "Stopping"
	default:
		return "Stopped"
	}
}

// maxLogLines caps the in-memory log ring buffer per service.
const maxLogLines = 5000

// Service represents a long-running Kafka process (ZooKeeper or the broker)
// launched from a .bat script, with live log capture.
type Service struct {
	Name string

	mgr *Manager

	mu       sync.RWMutex
	state    State
	cmd      *exec.Cmd
	logs     []string
	readyHit bool

	// readyMarker, when seen in logs, transitions Starting -> Running.
	readyMarker string

	stateSubs []func(State)
	logSubs   []func(line string)
}

func newService(name string, m *Manager) *Service {
	s := &Service{Name: name, mgr: m, state: StateStopped}
	switch name {
	case "ZooKeeper":
		s.readyMarker = "binding to port"
	case "Kafka Broker":
		s.readyMarker = "started (kafka.server.kafkaserver)"
	}
	return s
}

// State returns the current service state.
func (s *Service) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Logs returns a copy of the buffered log lines.
func (s *Service) Logs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.logs))
	copy(out, s.logs)
	return out
}

// SubscribeState registers a callback fired on every state change.
func (s *Service) SubscribeState(fn func(State)) {
	s.mu.Lock()
	s.stateSubs = append(s.stateSubs, fn)
	s.mu.Unlock()
}

// SubscribeLogs registers a callback fired for each new log line.
func (s *Service) SubscribeLogs(fn func(line string)) {
	s.mu.Lock()
	s.logSubs = append(s.logSubs, fn)
	s.mu.Unlock()
}

func (s *Service) setState(st State) {
	s.mu.Lock()
	if s.state == st {
		s.mu.Unlock()
		return
	}
	s.state = st
	subs := append([]func(State){}, s.stateSubs...)
	s.mu.Unlock()
	for _, fn := range subs {
		fn(st)
	}
}

func (s *Service) appendLog(line string) {
	s.mu.Lock()
	s.logs = append(s.logs, line)
	if len(s.logs) > maxLogLines {
		s.logs = s.logs[len(s.logs)-maxLogLines:]
	}
	marker := s.readyMarker
	hit := s.readyHit
	subs := append([]func(string){}, s.logSubs...)
	s.mu.Unlock()

	for _, fn := range subs {
		fn(line)
	}

	if !hit && marker != "" && containsFold(line, marker) {
		s.mu.Lock()
		s.readyHit = true
		s.mu.Unlock()
		s.setState(StateRunning)
	}
}

// ClearLogs empties the log buffer.
func (s *Service) ClearLogs() {
	s.mu.Lock()
	s.logs = nil
	s.mu.Unlock()
}

// command builds the *exec.Cmd to launch this service.
func (s *Service) command() (*exec.Cmd, error) {
	var tool, props string
	switch s.Name {
	case "ZooKeeper":
		tool = "zookeeper-server-start.bat"
		props = "zookeeper.properties"
	case "Kafka Broker":
		tool = "kafka-server-start.bat"
		props = "server.properties"
	default:
		return nil, fmt.Errorf("unknown service %q", s.Name)
	}
	propPath := s.mgr.configDir() + "\\" + props
	cmd := exec.Command(s.mgr.tool(tool), propPath)
	cmd.Dir = s.mgr.Config().KafkaPath
	cmd.SysProcAttr = hiddenWindow()
	return cmd, nil
}

// Start launches the service if it is not already running.
func (s *Service) Start() error {
	s.mu.RLock()
	st := s.state
	s.mu.RUnlock()
	if st == StateRunning || st == StateStarting {
		return fmt.Errorf("%s is already %s", s.Name, st)
	}

	cmd, err := s.command()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.readyHit = false
	s.cmd = cmd
	s.mu.Unlock()

	s.mgr.history.Add(CommandRecord{
		Display: formatCommand(cmd.Path, cmd.Args[1:]),
		Tool:    cmd.Path,
		Args:    cmd.Args[1:],
		When:    time.Now(),
	})

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	s.setState(StateStarting)
	if err := cmd.Start(); err != nil {
		s.setState(StateStopped)
		return err
	}

	go s.pump(stdout)
	go s.pump(stderr)

	// Wait for exit in the background to reset state.
	go func() {
		_ = cmd.Wait()
		s.setState(StateStopped)
	}()

	return nil
}

func (s *Service) pump(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		s.appendLog(sc.Text())
	}
}

// Stop terminates the service process tree.
func (s *Service) Stop() error {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("%s is not running", s.Name)
	}
	s.setState(StateStopping)
	if err := killTree(cmd.Process.Pid); err != nil {
		return err
	}
	return nil
}

// Restart stops (if running) then starts the service.
func (s *Service) Restart() error {
	if s.State() == StateRunning || s.State() == StateStarting {
		_ = s.Stop()
		// give the OS a moment to release ports
		time.Sleep(2 * time.Second)
	}
	return s.Start()
}
