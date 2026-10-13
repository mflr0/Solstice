// Package live tells connected browsers that an issue changed, so open pages
// refresh without polling. Events only carry the issue key: browsers fetch
// the new state through the API, where permissions apply as usual.
package live

import (
	"sync"

	"solstice/internal/perm"
)

// Event announces a change to an issue.
type Event struct {
	Key   string     `json:"key"`
	What  string     `json:"what"` // issue, comment, attachment, link, worklog
	By    int64      `json:"by"`   // user who made the change
	Issue perm.Issue `json:"-"`    // who may hear about it
	// A change to a restricted comment is only announced to those who may
	// read it: its group (lower-case name) and/or project role.
	Group string `json:"-"`
	Role  int64  `json:"-"`
}

type subscriber struct {
	ch  chan Event
	can func(Event) bool
}

// Hub fans events out to subscribers.
type Hub struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[*subscriber]struct{}{}} }

// Subscribe registers a listener that only receives the events for which can
// returns true. The returned function unsubscribes.
func (h *Hub) Subscribe(can func(Event) bool) (<-chan Event, func()) {
	s := &subscriber{ch: make(chan Event, 64), can: can}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s.ch, func() {
		h.mu.Lock()
		delete(h.subs, s)
		h.mu.Unlock()
	}
}

// Publish delivers an event without blocking: a subscriber too slow to keep
// up misses events and catches up on its next fetch.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if !s.can(e) {
			continue
		}
		select {
		case s.ch <- e:
		default:
		}
	}
}
