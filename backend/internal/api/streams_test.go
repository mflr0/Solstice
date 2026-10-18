package api

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"solstice/internal/perm"
)

func TestStreamSlotsPerUser(t *testing.T) {
	var c streamSlots
	for i := range maxStreamsPerUser {
		if !c.acquire(1) {
			t.Fatalf("stream %d of the user refused", i+1)
		}
	}
	if c.acquire(1) {
		t.Fatal("a user opened more streams than the cap")
	}
	if !c.acquire(2) {
		t.Fatal("another user was refused by the first one's streams")
	}
	if c.total != maxStreamsPerUser+1 {
		t.Errorf("%d streams counted, want %d", c.total, maxStreamsPerUser+1)
	}
}

func TestStreamSlotsReleased(t *testing.T) {
	var c streamSlots
	for range maxStreamsPerUser {
		if !c.acquire(1) {
			t.Fatal("a stream was refused")
		}
	}
	c.release(1)
	if !c.acquire(1) {
		t.Fatal("the slot of a closed stream was not freed")
	}
	if c.perUser[1] != maxStreamsPerUser {
		t.Errorf("%d streams for the user, want %d", c.perUser[1], maxStreamsPerUser)
	}
}

func TestStreamSlotsTotal(t *testing.T) {
	var c streamSlots
	for i := range maxStreamsTotal {
		if !c.acquire(int64(i + 1)) {
			t.Fatalf("stream %d refused before the total cap", i+1)
		}
	}
	if c.acquire(1) || c.acquire(maxStreamsTotal+1) {
		t.Fatal("the total cap refused nothing")
	}
	c.release(1)
	if !c.acquire(maxStreamsTotal + 1) {
		t.Fatal("a slot freed at the total cap was not counted again")
	}
}

func TestStreamSlotReleasedWhenClientCancels(t *testing.T) {
	srv := quietServer()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	slots := func() (total, perUser int) {
		srv.streams.mu.Lock()
		defer srv.streams.mu.Unlock()
		return srv.streams.total, srv.streams.perUser[42]
	}

	done := make(chan error, 1)
	go func() {
		done <- srv.streamEvents(w, req, w, &perm.Subject{UserID: 42})
	}()

	for total, _ := slots(); total == 0; total, _ = slots() {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("streamEvents: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the stream did not end after the client cancelled")
	}
	if total, perUser := slots(); total != 0 || perUser != 0 {
		t.Errorf("slots not released: total=%d perUser[42]=%d", total, perUser)
	}
}
