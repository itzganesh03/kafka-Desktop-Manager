package kafka

import (
	"sync"
	"time"
)

// topicCacheTTL is how long a fetched topic list is considered fresh. Dropdowns
// in the UI use the cache so they populate instantly instead of spawning a JVM
// (kafka-topics.bat) on every page open.
const topicCacheTTL = 30 * time.Second

type topicCache struct {
	mu    sync.Mutex
	names []string
	at    time.Time
}

var tCache topicCache

// CachedTopics returns the topic list, using a short-lived cache. If the cache
// is fresh it returns immediately; otherwise it fetches and refreshes it. If a
// fetch fails but a previous (stale) list exists, the stale list is returned.
func (m *Manager) CachedTopics() ([]string, error) {
	tCache.mu.Lock()
	if tCache.names != nil && time.Since(tCache.at) < topicCacheTTL {
		names := append([]string(nil), tCache.names...)
		tCache.mu.Unlock()
		return names, nil
	}
	tCache.mu.Unlock()

	names, err := m.ListTopics()
	if err != nil {
		tCache.mu.Lock()
		stale := append([]string(nil), tCache.names...)
		tCache.mu.Unlock()
		if len(stale) > 0 {
			return stale, nil
		}
		return nil, err
	}

	tCache.mu.Lock()
	tCache.names = names
	tCache.at = time.Now()
	tCache.mu.Unlock()
	return names, nil
}

// InvalidateTopicCache forces the next CachedTopics call to refetch (call after
// creating or deleting a topic).
func (m *Manager) InvalidateTopicCache() {
	tCache.mu.Lock()
	tCache.at = time.Time{}
	tCache.mu.Unlock()
}
