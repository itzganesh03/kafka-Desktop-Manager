package kafka

import (
	"encoding/json"
	"fmt"
	"time"
)

// RecreateTopic deletes and recreates a topic, preserving its partition count
// and replication factor. Returns combined command output.
func (m *Manager) RecreateTopic(topic string) (string, error) {
	detail, err := m.DescribeTopic(topic)
	if err != nil {
		return "", fmt.Errorf("describe before recreate: %w", err)
	}
	parts := detail.Partitions
	rf := detail.ReplicationFactor
	if parts < 1 {
		parts = 1
	}
	if rf < 1 {
		rf = 1
	}
	delOut, err := m.DeleteTopic(topic)
	if err != nil {
		return delOut, fmt.Errorf("delete during recreate: %w", err)
	}
	// Deletion is asynchronous; wait briefly for it to settle.
	time.Sleep(2 * time.Second)
	createOut, err := m.CreateTopic(topic, parts, rf)
	return delOut + "\n" + createOut, err
}

// PurgeTopic removes all records from a topic by recreating it. This is the
// most reliable purge using only the standard .bat tooling.
func (m *Manager) PurgeTopic(topic string) (string, error) {
	return m.RecreateTopic(topic)
}

// CreateSampleMessages produces n simple JSON sample messages to a topic.
func (m *Manager) CreateSampleMessages(topic string, n int) (string, error) {
	if n < 1 {
		n = 5
	}
	msgs := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		b, _ := json.Marshal(map[string]any{
			"id":        i,
			"message":   fmt.Sprintf("sample message %d", i),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		msgs = append(msgs, string(b))
	}
	return m.SendMessages(topic, msgs)
}
