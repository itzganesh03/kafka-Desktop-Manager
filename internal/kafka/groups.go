package kafka

import (
	"sort"
	"strconv"
	"strings"
)

const groupsTool = "kafka-consumer-groups.bat"

// GroupInfo summarises a consumer group.
type GroupInfo struct {
	Name    string
	State   string
	Members int
}

// GroupOffset is one partition assignment row from a group describe.
type GroupOffset struct {
	Topic         string
	Partition     int
	CurrentOffset string
	LogEndOffset  string
	Lag           int64
	ConsumerID    string
	Host          string
}

// ListGroups returns all consumer group names.
func (m *Manager) ListGroups() ([]string, error) {
	out, err := m.runTimeout(groupsTool, "--bootstrap-server", m.bootstrap(), "--list")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	sort.Strings(names)
	return names, nil
}

// DescribeGroup returns offset/lag rows for a group.
func (m *Manager) DescribeGroup(group string) ([]GroupOffset, error) {
	out, err := m.runTimeout(groupsTool, "--bootstrap-server", m.bootstrap(), "--describe", "--group", group)
	if err != nil {
		return nil, err
	}
	return parseGroupDescribe(out), nil
}

// GroupState returns a coarse state + member count for a group, used by the
// groups list. It reuses the --describe --members output.
func (m *Manager) GroupState(group string) (state string, members int) {
	out, err := m.runTimeout(groupsTool, "--bootstrap-server", m.bootstrap(), "--describe", "--group", group, "--state")
	if err != nil {
		return "Unknown", 0
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "GROUP") {
			continue
		}
		fields := splitMultiSpace(line)
		// GROUP COORDINATOR (ID) ASSIGNMENT-STRATEGY STATE #MEMBERS
		if len(fields) >= 2 {
			state = strings.TrimSpace(fields[len(fields)-2])
			members, _ = strconv.Atoi(strings.TrimSpace(fields[len(fields)-1]))
		}
	}
	if state == "" {
		state = "Unknown"
	}
	return state, members
}

// parseGroupDescribe parses the columnar --describe output.
func parseGroupDescribe(out string) []GroupOffset {
	var rows []GroupOffset
	var header []string
	for _, line := range strings.Split(out, "\n") {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}
		fields := splitMultiSpace(raw)
		if strings.HasPrefix(raw, "GROUP") {
			header = fields
			continue
		}
		col := func(name string) string {
			for i, h := range header {
				if strings.EqualFold(h, name) && i < len(fields) {
					return strings.TrimSpace(fields[i])
				}
			}
			return ""
		}
		row := GroupOffset{
			Topic:         col("TOPIC"),
			CurrentOffset: col("CURRENT-OFFSET"),
			LogEndOffset:  col("LOG-END-OFFSET"),
			ConsumerID:    col("CONSUMER-ID"),
			Host:          col("HOST"),
		}
		row.Partition, _ = strconv.Atoi(col("PARTITION"))
		row.Lag, _ = strconv.ParseInt(col("LAG"), 10, 64)
		if row.Topic == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

// ResetOffsetsToEarliest resets a group's offsets for a topic to earliest.
func (m *Manager) ResetOffsetsToEarliest(group, topic string) (string, error) {
	return m.runTimeout(groupsTool,
		"--bootstrap-server", m.bootstrap(),
		"--group", group, "--topic", topic,
		"--reset-offsets", "--to-earliest", "--execute",
	)
}

// ResetOffsetsToLatest resets a group's offsets for a topic to latest.
func (m *Manager) ResetOffsetsToLatest(group, topic string) (string, error) {
	return m.runTimeout(groupsTool,
		"--bootstrap-server", m.bootstrap(),
		"--group", group, "--topic", topic,
		"--reset-offsets", "--to-latest", "--execute",
	)
}

// CountGroups returns the number of consumer groups (used by the dashboard).
func (m *Manager) CountGroups() int {
	names, err := m.ListGroups()
	if err != nil {
		return 0
	}
	return len(names)
}
