package mail

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"solstice/internal/mail/mailtest"
)

// outboxRow is a row of the fake mail_outbox table.
type outboxRow struct {
	id       int64
	to       string
	status   string
	claim    string
	next     time.Time
	attempts int
}

// fakeOutbox is a mail_outbox table in memory that answers the statements of
// SendDue and Run. It records how many rows each pass claimed, and ended
// receives a value when a pass gives its leftovers back to the queue.
type fakeOutbox struct {
	mu     sync.Mutex
	now    time.Time
	rows   []*outboxRow
	claims []int
	ended  chan struct{}
}

func newFakeOutbox() *fakeOutbox {
	return &fakeOutbox{now: time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC), ended: make(chan struct{}, 16)}
}

// add queues n messages to a recipient, due now.
func (q *fakeOutbox) add(n int, to string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for range n {
		q.rows = append(q.rows, &outboxRow{id: int64(len(q.rows) + 1), to: to, status: "pending", next: q.now})
	}
}

func (q *fakeOutbox) clock() time.Time {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.now
}

func (q *fakeOutbox) advance(d time.Duration) {
	q.mu.Lock()
	q.now = q.now.Add(d)
	q.mu.Unlock()
}

func (q *fakeOutbox) claimed() []int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.claims)
}

// count returns how many rows have each status.
func (q *fakeOutbox) count() map[string]int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := map[string]int{}
	for _, r := range q.rows {
		n[r.status]++
	}
	return n
}

func (q *fakeOutbox) row(id int64) outboxRow {
	q.mu.Lock()
	defer q.mu.Unlock()
	return *q.rows[id-1]
}

func (q *fakeOutbox) Connect(context.Context) (driver.Conn, error) { return outboxConn{q}, nil }
func (q *fakeOutbox) Driver() driver.Driver                        { return nil }

type outboxConn struct{ *fakeOutbox }

func (outboxConn) Prepare(string) (driver.Stmt, error) { return nil, fmt.Errorf("not supported") }
func (outboxConn) Close() error                        { return nil }
func (outboxConn) Begin() (driver.Tx, error)           { return nil, fmt.Errorf("not supported") }

func (c outboxConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	arg := func(i int) any { return args[i].Value }
	n := int64(0)
	switch {
	case strings.Contains(query, "SET claim = ?, next_attempt_at = ?"):
		for _, r := range c.rows {
			if n < 50 && r.status == "pending" && !r.next.After(arg(2).(time.Time)) {
				r.claim, r.next = arg(0).(string), arg(1).(time.Time)
				n++
			}
		}
		c.claims = append(c.claims, int(n))
	case strings.Contains(query, "SET status = ?"):
		r := c.rows[arg(5).(int64)-1]
		r.status, r.attempts, r.next, r.claim = arg(0).(string), int(arg(1).(int64)), arg(4).(time.Time), ""
		n = 1
	case strings.Contains(query, "SET text_body = ''"), strings.Contains(query, "DELETE FROM mail_outbox"):
	case strings.Contains(query, "SET claim = NULL"):
		for _, r := range c.rows {
			if r.claim == arg(1).(string) && r.status == "pending" {
				r.claim, r.next = "", arg(0).(time.Time)
				n++
			}
		}
		c.ended <- struct{}{}
	default:
		return nil, fmt.Errorf("unexpected statement: %s", query)
	}
	return driver.RowsAffected(n), nil
}

func (c outboxConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "WHERE o.claim = ?") {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	rows := &outboxRows{}
	for _, r := range c.rows {
		if r.claim == args[0].Value.(string) {
			rows.rows = append(rows.rows, []driver.Value{r.id, "notification", int64(0), r.to, "Essai", "texte", "",
				fmt.Sprintf("<%d@exemple.fr>", r.id), "", int64(r.attempts), false})
		}
	}
	return rows, nil
}

type outboxRows struct{ rows [][]driver.Value }

func (*outboxRows) Columns() []string { return make([]string, 11) }
func (*outboxRows) Close() error      { return nil }
func (r *outboxRows) Next(dest []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(dest, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}

// runOutbox runs an outbox on q until the test ends. Each SendDue waits for
// the test to send it the value of Paused on the returned channel.
func runOutbox(t *testing.T, q *fakeOutbox, srv *mailtest.Server) (*Outbox, chan<- bool) {
	ctx, cancel := context.WithCancel(context.Background())
	o := NewOutbox(sql.OpenDB(q), slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) (Config, error) {
		return Config{Enabled: true, Host: srv.Host, Port: srv.Port, Security: SecurityNone, From: "solstice@exemple.fr"}, nil
	})
	o.Now = q.clock
	gate := make(chan bool)
	o.Paused = func() bool {
		select {
		case p := <-gate:
			return p
		case <-ctx.Done():
			return true
		}
	}
	done := make(chan struct{})
	go func() {
		o.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return o, gate
}

// sendPass lets the next SendDue run and waits until it has given its
// leftovers back to the queue.
func sendPass(t *testing.T, gate chan<- bool, q *fakeOutbox) {
	t.Helper()
	select {
	// Well under the ticker's period: a pass must come from the wake or the drain.
	case gate <- false:
	case <-time.After(time.Second):
		t.Fatal("the worker did not look at the outbox")
	}
	select {
	case <-q.ended:
	case <-time.After(30 * time.Second):
		t.Fatal("the pass did not end")
	}
}

// waits checks that the worker waits for its next tick: no SendDue starts.
func waits(t *testing.T, gate chan<- bool) {
	t.Helper()
	select {
	case gate <- false:
		t.Fatal("the worker looked at the outbox again without being woken")
	case <-time.After(100 * time.Millisecond):
	}
}

// A whole batch sent: the worker goes on with the next batch at once.
func TestRunDrainsWhileBatchesAreFull(t *testing.T) {
	srv := mailtest.Start(t)
	q := newFakeOutbox()
	q.add(120, "paul@exemple.fr")
	o, gate := runOutbox(t, q, srv)
	o.Wake()
	sendPass(t, gate, q)
	sendPass(t, gate, q)
	sendPass(t, gate, q)
	waits(t, gate)
	if got := q.claimed(); !slices.Equal(got, []int{50, 50, 20}) {
		t.Errorf("claimed %v, want [50 50 20]", got)
	}
	if n := q.count(); n["sent"] != 120 || len(srv.Messages()) != 120 {
		t.Errorf("statuses %v, %d messages received, want 120 sent", n, len(srv.Messages()))
	}
}

// A short batch empties the queue: the worker waits for the next tick or
// wake, and then takes what came meanwhile.
func TestRunWaitsAfterAShortBatch(t *testing.T) {
	srv := mailtest.Start(t)
	q := newFakeOutbox()
	q.add(30, "paul@exemple.fr")
	o, gate := runOutbox(t, q, srv)
	o.Wake()
	sendPass(t, gate, q)
	waits(t, gate)
	q.add(5, "paul@exemple.fr")
	o.Wake()
	sendPass(t, gate, q)
	waits(t, gate)
	if got := q.claimed(); !slices.Equal(got, []int{30, 5}) {
		t.Errorf("claimed %v, want [30 5]", got)
	}
	if n := q.count(); n["sent"] != 35 {
		t.Errorf("statuses %v, want 35 sent", n)
	}
}

// A failed message makes a full batch count as short: the rest waits for the
// next tick. A temporary failure comes back once the clock reaches its
// backoff.
func TestRunWaitsAfterAFailingRow(t *testing.T) {
	srv := mailtest.Start(t)
	q := newFakeOutbox()
	q.add(1, "paul@exemple.fr")
	q.add(1, "reject@exemple.fr")
	q.add(58, "paul@exemple.fr")
	o, gate := runOutbox(t, q, srv)
	o.Wake()
	sendPass(t, gate, q)
	waits(t, gate)
	if n := q.count(); n["sent"] != 49 || n["failed"] != 1 || n["pending"] != 10 {
		t.Fatalf("after one batch with a refused recipient: statuses %v, want 49 sent, 1 failed, 10 pending", n)
	}

	srv.FailWith("451 4.3.0 try later")
	o.Wake()
	sendPass(t, gate, q)
	waits(t, gate)
	start := q.clock()
	for id := int64(51); id <= 60; id++ {
		if r := q.row(id); r.status != "pending" || r.attempts != 1 || !r.next.Equal(start.Add(backoff(1))) {
			t.Fatalf("message %d after a temporary failure: %+v, want pending, 1 attempt, next at %v", id, r,
				start.Add(backoff(1)))
		}
	}

	srv.FailWith("")
	q.advance(backoff(1))
	o.Wake()
	sendPass(t, gate, q)
	if got := q.claimed(); !slices.Equal(got, []int{50, 10, 10}) {
		t.Errorf("claimed %v, want [50 10 10]", got)
	}
	if n := q.count(); n["sent"] != 59 || n["failed"] != 1 {
		t.Errorf("statuses %v, want 59 sent, 1 failed", n)
	}
}

// A paused tick sends nothing and the worker waits for the next one.
func TestRunSkipsATickWhilePaused(t *testing.T) {
	srv := mailtest.Start(t)
	q := newFakeOutbox()
	q.add(3, "paul@exemple.fr")
	o, gate := runOutbox(t, q, srv)
	o.Wake()
	gate <- true
	waits(t, gate)
	if got := q.claimed(); len(got) != 0 {
		t.Fatalf("paused: claimed %v", got)
	}
	o.Wake()
	sendPass(t, gate, q)
	if n := q.count(); n["sent"] != 3 {
		t.Errorf("statuses %v, want 3 sent", n)
	}
}
