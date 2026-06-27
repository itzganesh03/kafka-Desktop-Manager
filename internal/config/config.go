// Package config handles loading, saving and validating the application's
// local configuration as well as validating a Kafka installation directory.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Config is the persisted application configuration.
type Config struct {
	KafkaPath       string `json:"kafka_path"`
	BootstrapServer string `json:"bootstrap_server"`
	ZookeeperPort   string `json:"zookeeper_port"`
	DefaultTopic    string `json:"default_topic"`
	AutoStartZK     bool   `json:"auto_start_zookeeper"`
	AutoStartKafka  bool   `json:"auto_start_kafka"`
	Theme           string `json:"theme"` // "dark" or "light"

	// DataLogDir is Kafka's log.dirs (where partition data lives, e.g.
	// C:\kafka\kafka-logs). AppLogDir is where server/zk .log files are written
	// (e.g. C:\kafka\logs). Both are cleared by the "Metadata Delete" recovery
	// action. Their names vary per install, so the user picks them during setup.
	DataLogDir string `json:"data_log_dir"`
	AppLogDir  string `json:"app_log_dir"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		KafkaPath:       `C:\kafka`,
		BootstrapServer: "localhost:9092",
		ZookeeperPort:   "2181",
		DefaultTopic:    "",
		AutoStartZK:     false,
		AutoStartKafka:  false,
		Theme:           "dark",
	}
}

// requiredFiles are the files that must exist inside a valid Kafka install.
var requiredFiles = []string{
	filepath.Join("bin", "windows", "zookeeper-server-start.bat"),
	filepath.Join("bin", "windows", "kafka-server-start.bat"),
	filepath.Join("config", "zookeeper.properties"),
	filepath.Join("config", "server.properties"),
}

// ErrInvalidKafkaPath is returned when the install validation fails.
var ErrInvalidKafkaPath = errors.New("invalid kafka installation directory")

// ValidateKafkaPath verifies that the expected Kafka files exist under root.
// It returns the list of missing (relative) paths; an empty slice means valid.
func ValidateKafkaPath(root string) []string {
	var missing []string
	for _, rel := range requiredFiles {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			missing = append(missing, rel)
		}
	}
	return missing
}

// DetectDataLogDir determines Kafka's data-log directory. It first reads
// log.dirs from <kafkaPath>\config\server.properties; if that path exists on
// disk it is used. Otherwise it scans kafkaPath for a likely data-log folder
// (e.g. "kafka-logs", "kafkakafka-logs") so installs with a non-standard name
// are still found. Returns "" if nothing plausible is found.
func DetectDataLogDir(kafkaPath string) string {
	configured := parseLogDirs(kafkaPath)
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
	}
	// Fall back to scanning for a "*log*" directory under kafkaPath.
	if entries, err := os.ReadDir(kafkaPath); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if name == "logs" { // that's the app-log dir, not data
				continue
			}
			if strings.Contains(name, "log") {
				return filepath.Join(kafkaPath, e.Name())
			}
		}
	}
	return configured // may be "" or a non-existent configured path
}

// parseLogDirs returns the first log.dirs entry from server.properties,
// resolved against kafkaPath if relative.
func parseLogDirs(kafkaPath string) string {
	data, err := os.ReadFile(filepath.Join(kafkaPath, "config", "server.properties"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "log.dirs") && !strings.HasPrefix(line, "log.dir") {
			continue
		}
		_, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		if c := strings.Index(val, ","); c >= 0 {
			val = val[:c]
		}
		val = filepath.FromSlash(strings.TrimSpace(val))
		if !filepath.IsAbs(val) {
			val = filepath.Join(kafkaPath, val)
		}
		return filepath.Clean(val)
	}
	return ""
}

// DefaultAppLogDir returns the conventional Kafka application log dir.
func DefaultAppLogDir(kafkaPath string) string {
	return filepath.Join(kafkaPath, "logs")
}

// configDir returns %APPDATA%\KafkaDesktopManager (falling back to the user
// config dir) and ensures it exists.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "KafkaDesktopManager")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Path returns the absolute path to the config file.
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Exists reports whether a saved config file is present.
func Exists() bool {
	p, err := Path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Load reads the config from disk. If no file exists it returns Default().
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the config to disk as indented JSON.
func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
