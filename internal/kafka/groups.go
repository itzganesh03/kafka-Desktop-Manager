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

// headerCol records a column name and its start offset within the header line.
type headerCol struct {
	name       string
	start, end int
}

// parseHeaderColumns finds each column name and its character span. Kafka pads
// every column (header included) to a fixed width, so a value's characters
// always sit within its header column's [start,end) span — this is robust even
// when a long GROUP name leaves only a single space before the next column.
func parseHeaderColumns(header string) []headerCol {
	var cols []headerCol
	i, n := 0, len(header)
	for i < n {
		for i < n && header[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}
		start := i
		for i < n && header[i] != ' ' {
			i++
		}
		cols = append(cols, headerCol{name: header[start:i], start: start})
	}
	for idx := range cols {
		if idx+1 < len(cols) {
			cols[idx].end = cols[idx+1].start
		} else {
			cols[idx].end = 1 << 30
		}
	}
	return cols
}

// value extracts a column's text from a data line by header offset.
func (c headerCol) value(line string) string {
	if c.start >= len(line) {
		return ""
	}
	end := c.end
	if end > len(line) {
		end = len(line)
	}
	return strings.TrimSpace(line[c.start:end])
}

// parseGroupDescribe parses the columnar --describe output using header offsets.
func parseGroupDescribe(out string) []GroupOffset {
	var rows []GroupOffset
	var cols []headerCol
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if cols == nil {
			if strings.HasPrefix(strings.TrimSpace(line), "GROUP") && strings.Contains(line, "TOPIC") {
				cols = parseHeaderColumns(line)
			}
			continue
		}
		get := func(name string) string {
			for _, c := range cols {
				if strings.EqualFold(c.name, name) {
					return c.value(line)
				}
			}
			return ""
		}
		topic := get("TOPIC")
		if topic == "" || topic == "-" {
			continue
		}
		row := GroupOffset{
			Topic:         topic,
			CurrentOffset: get("CURRENT-OFFSET"),
			LogEndOffset:  get("LOG-END-OFFSET"),
			ConsumerID:    get("CONSUMER-ID"),
			Host:          get("HOST"),
		}
		row.Partition, _ = strconv.Atoi(get("PARTITION"))
		row.Lag, _ = strconv.ParseInt(get("LAG"), 10, 64)
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
