package kafka

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Docker mode runs a single-node KRaft broker (no ZooKeeper) in a container
// created from a generated docker-compose.yml. Admin/producer/consumer tools
// are executed inside the container with `docker exec`.

const (
	// DockerContainer is the container name set in the generated compose file.
	DockerContainer = "kafka"
	// dockerBootstrap is the broker address as seen from inside the container.
	dockerBootstrap = "localhost:9092"
	// dockerKafkaBin is where the apache/kafka image keeps the CLI scripts.
	dockerKafkaBin = "/opt/kafka/bin/"
	// dockerReadyMarker appears in the broker log once KRaft startup completes
	// ("[KafkaRaftServer nodeId=1] Kafka Server started").
	dockerReadyMarker = "kafka server started"
)

// ComposeFileName is the compose file written into the Docker folder.
const ComposeFileName = "docker-compose.yml"

// ComposeYAML returns the docker-compose.yml content for the given image.
func ComposeYAML(image string) string {
	return `services:
  kafka:
    image: ` + image + `
    container_name: ` + DockerContainer + `
    hostname: kafka
    ports:
      - "9092:9092"
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller

      KAFKA_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092

      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT

      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: 0

      KAFKA_LOG_DIRS: /var/lib/kafka/data

    volumes:
      - kafka_data:/var/lib/kafka/data

    restart: unless-stopped

volumes:
  kafka_data:
`
}

// dockerExecArgs maps a Windows .bat tool to its in-container .sh script and
// returns the `docker exec` argument list (without the leading "docker").
func dockerExecArgs(tool string, withStdin bool, args []string) []string {
	out := []string{"exec"}
	if withStdin {
		out = append(out, "-i")
	}
	out = append(out, DockerContainer, dockerScript(tool))
	return append(out, args...)
}

// dockerScript returns /opt/kafka/bin/<name>.sh for a "<name>.bat" tool.
func dockerScript(tool string) string {
	return dockerKafkaBin + strings.TrimSuffix(filepath.Base(tool), ".bat") + ".sh"
}

// shQuote single-quotes s for a POSIX shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// dockerTrackedExec runs a long-lived tool inside the container while writing
// its PID to pidFile, so it can be killed later (killing the `docker exec`
// client alone leaves the process running in the container). The Kafka
// scripts `exec` into java, so the recorded PID is the JVM itself.
func dockerTrackedExec(tool, pidFile string, args []string) *exec.Cmd {
	var b strings.Builder
	b.WriteString("echo $$ > " + shQuote(pidFile) + "; exec " + shQuote(dockerScript(tool)))
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(shQuote(a))
	}
	cmd := exec.Command("docker", "exec", DockerContainer, "sh", "-c", b.String())
	cmd.SysProcAttr = hiddenWindow()
	return cmd
}

// dockerKillTracked kills the in-container process recorded in pidFile.
func dockerKillTracked(pidFile string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	q := shQuote(pidFile)
	cmd := exec.CommandContext(ctx, "docker", "exec", DockerContainer, "sh", "-c",
		"[ -f "+q+" ] && kill $(cat "+q+") 2>/dev/null; rm -f "+q)
	cmd.SysProcAttr = hiddenWindow()
	_ = cmd.Run()
}

// DockerInfo describes the Docker installation on this machine.
type DockerInfo struct {
	Installed      bool   // docker CLI found on PATH
	EngineRunning  bool   // the daemon answered (Docker Desktop is running)
	Version        string // server version
	ComposeVersion string // docker compose plugin version ("" if missing)
	Detail         string // human-readable problem description, if any
}

// Usable reports whether Docker mode can be used right now.
func (d DockerInfo) Usable() bool {
	return d.Installed && d.EngineRunning && d.ComposeVersion != ""
}

// Summary is a one-line description for the UI.
func (d DockerInfo) Summary() string {
	switch {
	case !d.Installed:
		return "Docker not found on this machine"
	case !d.EngineRunning:
		return "Docker is installed but the engine is not running — start Docker Desktop"
	case d.ComposeVersion == "":
		return "Docker " + d.Version + " found, but `docker compose` is missing"
	default:
		return "Docker " + d.Version + " · Compose " + d.ComposeVersion
	}
}

// DetectDocker checks for the docker CLI, a running engine and Compose v2.
// It mirrors `docker version` / `docker compose version`.
func DetectDocker() DockerInfo {
	var info DockerInfo
	if _, err := exec.LookPath("docker"); err != nil {
		info.Detail = err.Error()
		return info
	}
	info.Installed = true

	out, err := dockerOutput(15*time.Second, "version", "--format", "{{.Server.Version}}")
	if err != nil || out == "" {
		info.Detail = strings.TrimSpace(out)
		return info
	}
	info.EngineRunning = true
	info.Version = out

	if cv, err := dockerOutput(15*time.Second, "compose", "version", "--short"); err == nil {
		info.ComposeVersion = cv
	}
	return info
}

// dockerOutput runs a quick docker command and returns trimmed combined output.
func dockerOutput(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.SysProcAttr = hiddenWindow()
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// dockerStream runs a docker command in dir, sending every output line (stdout
// and stderr) to onLine. It returns when the command exits.
func dockerStream(ctx context.Context, dir string, onLine func(string), args ...string) error {
	if onLine == nil {
		onLine = func(string) {}
	}
	onLine("> docker " + strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.SysProcAttr = hiddenWindow()
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if line := strings.TrimRight(sc.Text(), "\r"); strings.TrimSpace(line) != "" {
				onLine(line)
			}
		}
		_, _ = io.Copy(io.Discard, pr) // never block the writer if Scan stopped early
		close(done)
	}()
	err := cmd.Wait()
	_ = pw.Close()
	<-done
	if err != nil {
		return fmt.Errorf("docker %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

// ContainerRunning reports whether the Kafka container is up.
func ContainerRunning() bool {
	out, err := dockerOutput(10*time.Second, "inspect", "-f", "{{.State.Running}}", DockerContainer)
	return err == nil && out == "true"
}

// composeArgs prefixes compose sub-commands with plain (non-ANSI) output.
func composeArgs(args ...string) []string {
	return append([]string{"compose", "--ansi", "never"}, args...)
}

// ComposeFileExists reports whether dir contains docker-compose.yml.
func ComposeFileExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ComposeFileName))
	return err == nil
}

// hasComposeFile reports whether the configured Docker folder has the file.
func (m *Manager) hasComposeFile() bool {
	return ComposeFileExists(m.Config().DockerDirOrDefault())
}

// InstallDocker performs the Docker setup end-to-end, streaming progress to
// onLine:
//  1. verify `docker version` and `docker compose version`
//  2. create dir (e.g. C:\kafka-docker)
//  3. write docker-compose.yml (an existing, different file is kept as .bak)
//  4. `docker compose pull`
//  5. `docker compose up -d`
//
// ctx cancels a long-running pull.
func (m *Manager) InstallDocker(ctx context.Context, dir, image string, onLine func(string)) error {
	if onLine == nil {
		onLine = func(string) {}
	}
	if !m.IsDocker() && (m.Broker.State() != StateStopped || m.ZooKeeper.State() != StateStopped) {
		return fmt.Errorf("stop the local ZooKeeper and Kafka broker first — the Docker broker also needs port 9092")
	}

	onLine("Step 1/5 — Verifying Docker")
	if err := dockerStream(ctx, "", onLine, "version"); err != nil {
		return fmt.Errorf("docker is not working (is Docker Desktop running?): %w", err)
	}
	if err := dockerStream(ctx, "", onLine, "compose", "version"); err != nil {
		return fmt.Errorf("docker compose is not available: %w", err)
	}

	onLine("")
	onLine("Step 2/5 — Creating folder " + dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	onLine("")
	onLine("Step 3/5 — Writing " + filepath.Join(dir, ComposeFileName))
	path := filepath.Join(dir, ComposeFileName)
	content := ComposeYAML(image)
	if old, err := os.ReadFile(path); err == nil && string(old) != content {
		bak := path + ".bak"
		if err := os.WriteFile(bak, old, 0o644); err != nil {
			return err
		}
		onLine("Existing compose file backed up to " + bak)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}

	onLine("")
	onLine("Step 4/5 — Downloading the Kafka image (" + image + ")")
	if err := dockerStream(ctx, dir, onLine, composeArgs("pull")...); err != nil {
		return err
	}

	onLine("")
	onLine("Step 5/5 — Starting Kafka")
	if err := dockerStream(ctx, dir, onLine, composeArgs("up", "-d")...); err != nil {
		return err
	}
	onLine("")
	onLine("✓ Kafka is running in Docker (container \"" + DockerContainer + "\", localhost:9092)")
	return nil
}

// ResetDockerData stops the container and deletes its data volume
// (`docker compose down -v`) — the Docker equivalent of Metadata Delete.
func (m *Manager) ResetDockerData(onLine func(string)) error {
	if !m.hasComposeFile() {
		return fmt.Errorf("no %s in %s — run \"Install Kafka via Docker\" from Settings", ComposeFileName, m.Config().DockerDirOrDefault())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := dockerStream(ctx, m.Config().DockerDirOrDefault(), onLine, composeArgs("down", "-v")...)
	m.Broker.stopLogFollower()
	return err
}

// ---- Broker service in Docker mode ----

// startDocker runs `docker compose up -d` and then follows the container log.
func (s *Service) startDocker() error {
	if s.Name != "Kafka Broker" {
		return fmt.Errorf("%s is not used in Docker mode (the container runs Kafka in KRaft mode)", s.Name)
	}
	if st := s.State(); st == StateRunning || st == StateStarting {
		return fmt.Errorf("%s is already %s", s.Name, st)
	}
	if !s.mgr.hasComposeFile() {
		return fmt.Errorf("no %s in %s — run \"Install Kafka via Docker\" from Settings", ComposeFileName, s.mgr.Config().DockerDirOrDefault())
	}

	s.mu.Lock()
	s.readyHit = false
	s.readyMarker = dockerReadyMarker
	s.mu.Unlock()
	s.setState(StateStarting)

	dir := s.mgr.Config().DockerDirOrDefault()
	args := composeArgs("up", "-d")
	s.mgr.history.Add(CommandRecord{Display: "docker " + strings.Join(args, " "), Tool: "docker", Args: args, When: time.Now()})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := dockerStream(ctx, dir, s.appendLog, args...); err != nil {
		s.setState(StateStopped)
		return err
	}
	return s.followDockerLogs()
}

// AttachDocker attaches to an already-running Kafka container (e.g. on app
// launch) so its state and logs show up. It is a no-op outside Docker mode.
func (s *Service) AttachDocker() {
	if !s.mgr.IsDocker() || s.Name != "Kafka Broker" || s.State() != StateStopped {
		return
	}
	if !ContainerRunning() {
		return
	}
	s.mu.Lock()
	s.readyHit = false
	s.readyMarker = dockerReadyMarker
	s.mu.Unlock()
	s.setState(StateStarting)
	if err := s.followDockerLogs(); err != nil {
		s.appendLog("failed to follow container logs: " + err.Error())
		s.setState(StateStopped)
	}
}

// followDockerLogs streams `docker logs -f` from the current container start.
// The follower exits when the container stops, which moves the state to
// Stopped (mirroring how the local .bat process exit works).
func (s *Service) followDockerLogs() error {
	startedAt, err := dockerOutput(10*time.Second, "inspect", "-f", "{{.State.StartedAt}}", DockerContainer)
	if err != nil {
		return fmt.Errorf("inspect %s: %v %s", DockerContainer, err, startedAt)
	}
	cmd := exec.Command("docker", "logs", "-f", "--since", startedAt, DockerContainer)
	cmd.SysProcAttr = hiddenWindow()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	s.cmd = cmd
	s.mu.Unlock()

	go s.pump(stdout)
	go s.pump(stderr)
	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		same := s.cmd == cmd
		s.mu.Unlock()
		if same {
			s.setState(StateStopped)
		}
	}()
	return nil
}

// stopLogFollower kills the `docker logs -f` process (not the container).
func (s *Service) stopLogFollower() {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	s.setState(StateStopped)
}

// stopDocker stops the Kafka container (data volume is kept).
func (s *Service) stopDocker() error {
	if s.Name != "Kafka Broker" {
		return fmt.Errorf("%s is not used in Docker mode", s.Name)
	}
	s.setState(StateStopping)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var err error
	if s.mgr.hasComposeFile() {
		args := composeArgs("stop")
		s.mgr.history.Add(CommandRecord{Display: "docker " + strings.Join(args, " "), Tool: "docker", Args: args, When: time.Now()})
		err = dockerStream(ctx, s.mgr.Config().DockerDirOrDefault(), s.appendLog, args...)
	} else {
		err = dockerStream(ctx, "", s.appendLog, "stop", DockerContainer)
	}
	s.stopLogFollower()
	return err
}
