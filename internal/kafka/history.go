package kafka

import "sync"

// History is a bounded, thread-safe store of executed commands (most recent
// first).
type History struct {
	mu      sync.RWMutex
	records []CommandRecord
	max     int
	subs    []func()
}

// NewHistory creates a history store keeping up to max records.
func NewHistory(max int) *History {
	if max <= 0 {
		max = 100
	}
	return &History{max: max}
}

// Add prepends a record, trimming to the max size, and notifies subscribers.
func (h *History) Add(r CommandRecord) {
	h.mu.Lock()
	h.records = append([]CommandRecord{r}, h.records...)
	if len(h.records) > h.max {
		h.records = h.records[:h.max]
	}
	subs := append([]func(){}, h.subs...)
	h.mu.Unlock()
	for _, s := range subs {
		s()
	}
}

// List returns a copy of the records (most recent first).
func (h *History) List() []CommandRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]CommandRecord, len(h.records))
	copy(out, h.records)
	return out
}

// Subscribe registers a callback fired whenever a record is added.
func (h *History) Subscribe(fn func()) {
	h.mu.Lock()
	h.subs = append(h.subs, fn)
	h.mu.Unlock()
}
