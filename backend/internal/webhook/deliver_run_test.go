package webhook

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// deliveryRow is a row of the fake webhook_deliveries table.
type deliveryRow struct {
	id       int64
	url      string
	status   string
	claim    string
	next     time.Time
	attempts int
}

// fakeDeliveries is a webhook_deliveries table in memory that answers the
// statements of DeliverDue. It records how many rows each pass claimed, and
// ended receives a value when a pass gives its leftovers back to the queue.
type fakeDeliveries struct {
	mu     sync.Mutex
	now    time.Time
	rows   []*deliveryRow
	claims []int
	ended  chan struct{}
}

func newFakeDeliveries() *fakeDeliveries {
	return &fakeDeliveries{now: time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC), ended: make(chan struct{}, 16)}
}

// add queues n deliveries to a URL, due now.
func (q *fakeDeliveries) add(n int, url string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for range n {
		q.rows = append(q.rows, &deliveryRow{id: int64(len(q.rows) + 1), url: url, status: "pending", next: q.now})
	}
}

func (q *fakeDeliveries) clock() time.Time {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.now
}

func (q *fakeDeliveries) advance(d time.Duration) {
	q.mu.Lock()
	q.now = q.now.Add(d)
	q.mu.Unlock()
}

func (q *fakeDeliveries) claimed() []int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.claims)
}

// count returns how many rows have each status.
func (q *fakeDeliveries) count() map[string]int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := map[string]int{}
	for _, r := range q.rows {
		n[r.status]++
	}
	return n
}

func (q *fakeDeliveries) row(id int64) deliveryRow {
	q.mu.Lock()
	defer q.mu.Unlock()
	return *q.rows[id-1]
}

func (q *fakeDeliveries) Connect(context.Context) (driver.Conn, error) { return deliveriesConn{q}, nil }
func (q *fakeDeliveries) Driver() driver.Driver                        { return nil }

type deliveriesConn struct{ *fakeDeliveries }

func (deliveriesConn) Prepare(string) (driver.Stmt, error) { return nil, fmt.Errorf("not supported") }
func (deliveriesConn) Close() error                        { return nil }
func (deliveriesConn) Begin() (driver.Tx, error)           { return nil, fmt.Errorf("not supported") }

func (c deliveriesConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
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
		r := c.rows[arg(7).(int64)-1]
		r.status, r.attempts, r.next, r.claim = arg(0).(string), int(arg(1).(int64)), arg(6).(time.Time), ""
		n = 1
	case strings.Contains(query, "DELETE FROM webhook_deliveries"):
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

func (c deliveriesConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "WHERE d.claim = ?") {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	rows := &deliveryRows{}
	for _, r := range c.rows {
		if r.claim == args[0].Value.(string) {
			rows.rows = append(rows.rows, []driver.Value{r.id, int64(1), IssueUpdated, r.url, "{}", int64(r.attempts), ""})
		}
	}
	return rows, nil
}

type deliveryRows struct{ rows [][]driver.Value }

func (*deliveryRows) Columns() []string { return make([]string, 7) }
func (*deliveryRows) Close() error      { return nil }
func (r *deliveryRows) Next(dest []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(dest, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}

// receiver accepts every delivery, but answers 500 on /fail while failing
// is set.
func receiver(t *testing.T, failing *atomic.Bool) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" && failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runDeliveries runs the service's worker on q until the test ends. Each
// DeliverDue waits for the test to send it the value of Paused on the
// returned channel.
func runDeliveries(t *testing.T, q *fakeDeliveries) (*Service, chan<- bool) {
	ctx, cancel := context.WithCancel(context.Background())
	s := New(sql.OpenDB(q), slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	s.Now = q.clock
	gate := make(chan bool)
	s.Paused = func() bool {
		select {
		case p := <-gate:
			return p
		case <-ctx.Done():
			return true
		}
	}
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return s, gate
}

// deliverPass lets the next DeliverDue run and waits until it has given its
// leftovers back to the queue.
func deliverPass(t *testing.T, gate chan<- bool, q *fakeDeliveries) {
	t.Helper()
	select {
	// Well under the ticker's period: a pass must come from the wake or the drain.
	case gate <- false:
	case <-time.After(time.Second):
		t.Fatal("the worker did not look at the deliveries")
	}
	select {
	case <-q.ended:
	case <-time.After(30 * time.Second):
		t.Fatal("the pass did not end")
	}
}

// waits checks that the worker waits for its next tick: no DeliverDue starts.
func waits(t *testing.T, gate chan<- bool) {
	t.Helper()
	select {
	case gate <- false:
		t.Fatal("the worker looked at the deliveries again without being woken")
	case <-time.After(100 * time.Millisecond):
	}
}

// A whole batch delivered: the worker goes on with the next batch at once.
func TestRunDrainsWhileBatchesAreFull(t *testing.T) {
	srv := receiver(t, new(atomic.Bool))
	q := newFakeDeliveries()
	q.add(120, srv.URL+"/hook")
	s, gate := runDeliveries(t, q)
	s.Wake()
	deliverPass(t, gate, q)
	deliverPass(t, gate, q)
	deliverPass(t, gate, q)
	waits(t, gate)
	if got := q.claimed(); !slices.Equal(got, []int{50, 50, 20}) {
		t.Errorf("claimed %v, want [50 50 20]", got)
	}
	if n := q.count(); n["sent"] != 120 {
		t.Errorf("statuses %v, want 120 sent", n)
	}
}

// A short batch empties the queue: the worker waits for the next tick or
// wake, and then takes what came meanwhile.
func TestRunWaitsAfterAShortBatch(t *testing.T) {
	srv := receiver(t, new(atomic.Bool))
	q := newFakeDeliveries()
	q.add(30, srv.URL+"/hook")
	s, gate := runDeliveries(t, q)
	s.Wake()
	deliverPass(t, gate, q)
	waits(t, gate)
	q.add(5, srv.URL+"/hook")
	s.Wake()
	deliverPass(t, gate, q)
	waits(t, gate)
	if got := q.claimed(); !slices.Equal(got, []int{30, 5}) {
		t.Errorf("claimed %v, want [30 5]", got)
	}
	if n := q.count(); n["sent"] != 35 {
		t.Errorf("statuses %v, want 35 sent", n)
	}
}

// A failed delivery makes a full batch count as short: the rest waits for the
// next tick. The failed one comes back once the clock reaches its backoff.
func TestRunWaitsAfterAFailingRow(t *testing.T) {
	failing := new(atomic.Bool)
	failing.Store(true)
	srv := receiver(t, failing)
	q := newFakeDeliveries()
	q.add(1, srv.URL+"/hook")
	q.add(1, srv.URL+"/fail")
	q.add(58, srv.URL+"/hook")
	s, gate := runDeliveries(t, q)
	s.Wake()
	deliverPass(t, gate, q)
	waits(t, gate)
	start := q.clock()
	if r := q.row(2); r.status != "pending" || r.attempts != 1 || !r.next.Equal(start.Add(backoff(1))) {
		t.Fatalf("failed delivery: %+v, want pending, 1 attempt, next at %v", r, start.Add(backoff(1)))
	}
	if n := q.count(); n["sent"] != 49 || n["pending"] != 11 {
		t.Fatalf("after one batch with a failure: statuses %v, want 49 sent, 11 pending", n)
	}

	s.Wake()
	deliverPass(t, gate, q)
	waits(t, gate)
	if r := q.row(2); r.attempts != 1 {
		t.Fatalf("failed delivery retried before its backoff: %+v", r)
	}

	failing.Store(false)
	q.advance(backoff(1))
	s.Wake()
	deliverPass(t, gate, q)
	if got := q.claimed(); !slices.Equal(got, []int{50, 10, 1}) {
		t.Errorf("claimed %v, want [50 10 1]", got)
	}
	if n := q.count(); n["sent"] != 60 {
		t.Errorf("statuses %v, want 60 sent", n)
	}
}

// A paused tick delivers nothing and the worker waits for the next one.
func TestRunSkipsATickWhilePaused(t *testing.T) {
	srv := receiver(t, new(atomic.Bool))
	q := newFakeDeliveries()
	q.add(3, srv.URL+"/hook")
	s, gate := runDeliveries(t, q)
	s.Wake()
	gate <- true
	waits(t, gate)
	if got := q.claimed(); len(got) != 0 {
		t.Fatalf("paused: claimed %v", got)
	}
	s.Wake()
	deliverPass(t, gate, q)
	if n := q.count(); n["sent"] != 3 {
		t.Errorf("statuses %v, want 3 sent", n)
	}
}
