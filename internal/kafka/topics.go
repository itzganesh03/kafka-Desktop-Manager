package kafka

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const topicsTool = "kafka-topics.bat"

// TopicInfo summarises a topic for the list table.
type TopicInfo struct {
	Name              string
	Partitions        int
	ReplicationFactor int
}

// PartitionInfo is one partition row from a describe.
type PartitionInfo struct {
	Partition int
	Leader    string
	Replicas  string
	ISR       string
}

// TopicDetail is the full describe output for a single topic.
type TopicDetail struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	Configs           string
	PartitionRows     []PartitionInfo
}

// ListTopics returns just the topic names.
func (m *Manager) ListTopics() ([]string, error) {
	out, err := m.runTimeout(topicsTool, "--bootstrap-server", m.bootstrap(), "--list")
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

// ListTopicInfos returns name + partition + replication-factor for every topic
// by parsing a single describe-all call.
func (m *Manager) ListTopicInfos() ([]TopicInfo, error) {
	out, err := m.runTimeout(topicsTool, "--bootstrap-server", m.bootstrap(), "--describe")
	if err != nil {
		return nil, err
	}
	byName := map[string]*TopicInfo{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Topic:") || strings.Contains(line, "Partition:") {
			continue
		}
		fields := parseFields(line)
		name := fields["Topic"]
		if name == "" {
			continue
		}
		ti := &TopicInfo{Name: name}
		ti.Partitions, _ = strconv.Atoi(fields["PartitionCount"])
		ti.ReplicationFactor, _ = strconv.Atoi(fields["ReplicationFactor"])
		byName[name] = ti
	}
	infos := make([]TopicInfo, 0, len(byName))
	for _, ti := range byName {
		infos = append(infos, *ti)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// DescribeTopic returns full detail for a single topic.
func (m *Manager) DescribeTopic(topic string) (*TopicDetail, error) {
	out, err := m.runTimeout(topicsTool, "--bootstrap-server", m.bootstrap(), "--describe", "--topic", topic)
	if err != nil {
		return nil, err
	}
	d := &TopicDetail{Name: topic}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := parseFields(line)
		if _, ok := fields["PartitionCount"]; ok {
			d.Partitions, _ = strconv.Atoi(fields["PartitionCount"])
			d.ReplicationFactor, _ = strconv.Atoi(fields["ReplicationFactor"])
			d.Configs = fields["Configs"]
			continue
		}
		if p, ok := fields["Partition"]; ok {
			pi := PartitionInfo{Leader: fields["Leader"], Replicas: fields["Replicas"], ISR: fields["Isr"]}
			pi.Partition, _ = strconv.Atoi(p)
			d.PartitionRows = append(d.PartitionRows, pi)
		}
	}
	return d, nil
}

// CreateTopic creates a new topic.
func (m *Manager) CreateTopic(name string, partitions, replication int) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("topic name is required")
	}
	if partitions < 1 {
		partitions = 1
	}
	if replication < 1 {
		replication = 1
	}
	out, err := m.runTimeout(topicsTool,
		"--bootstrap-server", m.bootstrap(),
		"--create", "--topic", name,
		"--partitions", strconv.Itoa(partitions),
		"--replication-factor", strconv.Itoa(replication),
	)
	if err == nil {
		m.InvalidateTopicCache()
	}
	return out, err
}

// DeleteTopic deletes a topic.
func (m *Manager) DeleteTopic(name string) (string, error) {
	out, err := m.runTimeout(topicsTool, "--bootstrap-server", m.bootstrap(), "--delete", "--topic", name)
	if err == nil {
		m.InvalidateTopicCache()
	}
	return out, err
}

// parseFields parses Kafka's tab/space separated "Key: value" describe lines
// into a map. Kafka separates pairs by tabs and uses "Key: Value".
func parseFields(line string) map[string]string {
	out := map[string]string{}
	// Split on tabs first; fall back to runs of spaces.
	parts := strings.Split(line, "\t")
	if len(parts) == 1 {
		parts = splitMultiSpace(line)
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		idx := strings.Index(p, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(p[:idx])
		val := strings.TrimSpace(p[idx+1:])
		out[key] = val
	}
	return out
}

// splitMultiSpace splits on runs of 2+ spaces, preserving "Key: value" pairs
// that contain single spaces.
func splitMultiSpace(s string) []string {
	var parts []string
	var cur strings.Builder
	spaces := 0
	for _, r := range s {
		if r == ' ' {
			spaces++
			if spaces >= 2 {
				if cur.Len() > 0 {
					parts = append(parts, cur.String())
					cur.Reset()
				}
				continue
			}
			cur.WriteRune(r)
		} else {
			spaces = 0
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}
