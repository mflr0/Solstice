package api

import "testing"

// A user can hold a few streams but not more than the cap, even by opening
// them from several accounts at once, and one user's cap is another's.
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

// A stream that ends frees its slot for the next one.
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

// The total cap refuses everyone once reached, and freeing a slot opens the
// server again.
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
