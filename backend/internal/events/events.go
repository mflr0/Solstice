// Package events carries what happens in Solstice (an issue created, a
// comment edited, a sprint started…) from the request that did it to what
// reacts to it: open pages refreshing, e-mail notifications, webhooks.
//
// Requests publish on a Bus. Live updates are handed over at once; the other
// consumers run on the bus's own goroutine, so an HTTP request never waits
// for an e-mail or a webhook.
package events

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"solstice/internal/issue"
	"solstice/internal/perm"
)

// Kind is what happened. Kinds follow Jira's webhook events.
type Kind string

const (
	IssueCreated   Kind = "issue_created"
	IssueUpdated   Kind = "issue_updated"
	IssueDeleted   Kind = "issue_deleted"
	CommentCreated Kind = "comment_created"
	CommentUpdated Kind = "comment_updated"
	CommentDeleted Kind = "comment_deleted"
	WorklogCreated Kind = "worklog_created"
	WorklogUpdated Kind = "worklog_updated"
	WorklogDeleted Kind = "worklog_deleted"
	SprintCreated  Kind = "sprint_created"
	SprintStarted  Kind = "sprint_started"
	SprintUpdated  Kind = "sprint_updated"
	SprintClosed   Kind = "sprint_closed"
	SprintDeleted  Kind = "sprint_deleted"
	BoardCreated   Kind = "board_created"
	BoardUpdated   Kind = "board_updated"
	BoardDeleted   Kind = "board_deleted"
)

// Change is one line of an issue's history, as Jira's changelog items: raw
// values (ids) and their display strings.
type Change struct {
	Field      string
	From, To   *string
	FromString *string
	ToString   *string
}

// Comment is a comment as it was when the event happened.
type Comment struct {
	ID        int64
	AuthorID  int64
	UpdatedBy int64
	Body      string
	OldBody   string // before an edit
	Group     string // restricted to this group (lower-case name)…
	RoleID    int64  // …and/or this project role
	Created   time.Time
	Updated   time.Time
}

// Worklog is a work log as it was when the event happened.
type Worklog struct {
	ID        int64
	AuthorID  int64
	UpdatedBy int64 // who last changed it, 0 when never
	Body      string
	Started   time.Time
	Seconds   int64
	Created   time.Time
	Updated   time.Time // zero when never changed
}

// Sprint is a sprint as it was when the event happened.
type Sprint struct {
	ID        int64
	BoardID   int64
	Name      string
	State     issue.SprintState
	Goal      *string
	Start     *time.Time
	End       *time.Time
	Completed *time.Time
}

// Board is a board as it was when the event happened.
type Board struct {
	ID   int64
	Name string
	Kind string // kanban, scrum
}

// Event is something that happened, with what consumers need to know.
type Event struct {
	Kind    Kind // empty: only open pages need to know (a card moved up)
	At      time.Time
	ActorID int64

	// The issue concerned, if any, and who may hear about it.
	IssueID int64
	Key     string
	Access  perm.Issue
	Before  *perm.Issue // visibility before the change, when it changed

	// Live is what open pages refresh (issue, comment, attachment, link,
	// worklog, deleted); empty for none.
	Live string

	Changes []Change
	Comment *Comment
	Worklog *Worklog
	Sprint  *Sprint
	Board   *Board

	// Issue is the issue's state. The publisher sets it for a deleted issue
	// (it is gone by the time consumers run); otherwise the bus loads it
	// when a consumer needs it.
	Issue *Issue

	// HookMatches, for a deleted issue, are the webhooks whose JQL filter
	// matched it just before it was deleted.
	HookMatches map[int64]bool
}

// Str is a helper for optional strings in changes.
func Str(s string) *string { return &s }

// Int is an optional string holding an id.
func Int(id int64) *string { s := strconv.FormatInt(id, 10); return &s }

// Consumer reacts to events on the bus's goroutine.
type Consumer interface {
	// Wants tells, cheaply, whether the consumer has anything to do with e.
	Wants(e *Event) bool
	// Handle processes e. Errors are logged; the event is not retried.
	Handle(ctx context.Context, e *Event) error
}

// Bus delivers events to the live hub immediately and to consumers in
// order, on its own goroutine.
type Bus struct {
	db  *sql.DB
	log *slog.Logger

	mu        sync.Mutex
	cond      *sync.Cond
	queue     []Event
	busy      bool
	closed    bool
	done      chan struct{}
	live      []func(Event)
	consumers []Consumer
	custom    func(context.Context, int64) ([]CustomValue, error)
}

// NewBus starts a bus. db is used to load issues for consumers.
func NewBus(db *sql.DB, log *slog.Logger) *Bus {
	b := &Bus{db: db, log: log, done: make(chan struct{})}
	b.cond = sync.NewCond(&b.mu)
	go b.loop()
	return b
}

// OnPublish registers a listener called synchronously by Publish. It must
// not block.
func (b *Bus) OnPublish(f func(Event)) {
	b.mu.Lock()
	b.live = append(b.live, f)
	b.mu.Unlock()
}

// LoadCustom sets how the bus reads an issue's custom fields for consumers:
// only the API knows how to render them.
func (b *Bus) LoadCustom(f func(ctx context.Context, issueID int64) ([]CustomValue, error)) {
	b.mu.Lock()
	b.custom = f
	b.mu.Unlock()
}

// Subscribe registers a consumer.
func (b *Bus) Subscribe(c Consumer) {
	b.mu.Lock()
	b.consumers = append(b.consumers, c)
	b.mu.Unlock()
}

// Publish hands an event over. It never blocks on consumers.
func (b *Bus) Publish(e Event) {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	b.mu.Lock()
	live := b.live
	closed := b.closed
	if !closed && e.Kind != "" {
		b.queue = append(b.queue, e)
		b.cond.Broadcast()
	}
	b.mu.Unlock()
	if closed && e.Kind != "" {
		b.log.Warn("event published after shutdown, dropped", "kind", e.Kind, "key", e.Key)
	}
	for _, f := range live {
		f(e)
	}
}

// Wait returns once every event published so far has been handled.
func (b *Bus) Wait() {
	b.mu.Lock()
	for len(b.queue) > 0 || b.busy {
		b.cond.Wait()
	}
	b.mu.Unlock()
}

// Close stops taking events and waits for the queued ones to be handled, or
// for ctx to end.
func (b *Bus) Close(ctx context.Context) error {
	b.mu.Lock()
	b.closed = true
	b.cond.Broadcast()
	b.mu.Unlock()
	select {
	case <-b.done:
		return nil
	case <-ctx.Done():
		b.mu.Lock()
		lost := len(b.queue)
		b.mu.Unlock()
		b.log.Warn("shutdown: events not handled", "count", lost)
		return ctx.Err()
	}
}

func (b *Bus) loop() {
	defer close(b.done)
	for {
		b.mu.Lock()
		for len(b.queue) == 0 && !b.closed {
			b.cond.Wait()
		}
		if len(b.queue) == 0 {
			b.mu.Unlock()
			return
		}
		e := b.queue[0]
		b.queue = b.queue[1:]
		b.busy = true
		consumers, custom := b.consumers, b.custom
		b.mu.Unlock()

		b.dispatch(&e, consumers, custom)

		b.mu.Lock()
		b.busy = false
		b.cond.Broadcast()
		b.mu.Unlock()
	}
}

func (b *Bus) dispatch(e *Event, consumers []Consumer, custom func(context.Context, int64) ([]CustomValue, error)) {
	defer func() {
		if r := recover(); r != nil {
			b.log.Error("event consumer panicked", "kind", e.Kind, "key", e.Key, "panic", r)
		}
	}()
	var wanted []Consumer
	for _, c := range consumers {
		if c.Wants(e) {
			wanted = append(wanted, c)
		}
	}
	if len(wanted) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if e.Issue == nil && e.IssueID != 0 {
		iss, err := LoadIssue(ctx, b.db, e.IssueID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// Deleted meanwhile: consumers see no issue.
		case err != nil:
			b.log.Error("event: loading the issue failed", "key", e.Key, "err", err)
			return
		default:
			e.Issue = iss
		}
		if e.Issue != nil && custom != nil {
			if e.Issue.Custom, err = custom(ctx, e.IssueID); err != nil {
				b.log.Error("event: loading the custom fields failed", "key", e.Key, "err", err)
				return
			}
		}
	}
	for _, c := range wanted {
		if err := c.Handle(ctx, e); err != nil {
			b.log.Error("event consumer failed", "kind", e.Kind, "key", e.Key, "err", err)
		}
	}
}
