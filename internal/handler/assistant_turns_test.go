package handler

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestTurnRegistryAdmitsOneTurnPerSession(t *testing.T) {
	var reg turnRegistry
	session := uuid.New()

	first, ok := reg.admit(session, func() {})
	if !ok {
		t.Fatal("the first turn of an idle session was refused")
	}
	if _, ok := reg.admit(session, func() {}); ok {
		t.Fatal("a second turn was admitted beside the first; two turns would write one CV")
	}

	// Another session is another slot: the bound is per conversation, not global.
	if _, ok := reg.admit(uuid.New(), func() {}); !ok {
		t.Fatal("a different session was refused while this one ran")
	}

	reg.release(session, first)
	if _, ok := reg.admit(session, func() {}); !ok {
		t.Fatal("the session stayed busy after its turn ended")
	}
}

// A registry that keeps entries for turns that have ended is a leak that grows for as long as
// the process lives, and it would also refuse every later turn of the session.
func TestTurnRegistryForgetsAFinishedTurn(t *testing.T) {
	var reg turnRegistry
	session := uuid.New()

	slot, _ := reg.admit(session, func() {})
	reg.release(session, slot)

	if n := reg.len(); n != 0 {
		t.Fatalf("registry holds %d entries after the turn ended, want 0", n)
	}
}

func TestTurnRegistryCancelsTheTurnItHolds(t *testing.T) {
	var reg turnRegistry
	session := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())

	slot, _ := reg.admit(session, cancel)
	if !reg.cancel(session) {
		t.Fatal("cancelling a running turn reported nothing to cancel")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("the turn's context was not cancelled")
	}
	reg.release(session, slot)
}

// Cancelling a session with nothing running is how a client stops a turn it cannot see the end
// of. It reports that there was nothing to do rather than failing.
func TestTurnRegistryCancelIsHarmlessWhenIdle(t *testing.T) {
	var reg turnRegistry

	if reg.cancel(uuid.New()) {
		t.Fatal("cancelling an idle session claimed to have cancelled a turn")
	}
}

// The registry is reached from the stream writer's goroutine and from cancel requests at the
// same time, so its map must never be touched unguarded.
func TestTurnRegistryIsSafeUnderConcurrentUse(t *testing.T) {
	var reg turnRegistry
	sessions := make([]uuid.UUID, 8)
	for i := range sessions {
		sessions[i] = uuid.New()
	}

	var wg sync.WaitGroup
	for _, session := range sessions {
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if slot, ok := reg.admit(session, func() {}); ok {
					reg.cancel(session)
					reg.release(session, slot)
				}
			}()
		}
	}
	wg.Wait()

	if n := reg.len(); n != 0 {
		t.Fatalf("registry holds %d entries after every turn ended, want 0", n)
	}
}
