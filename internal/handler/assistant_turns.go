package handler

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// turnSlot is one session's running turn, held only for as long as it runs. It carries the
// turn's cancellation so a later request can reach a turn that is already streaming on another
// goroutine — the only handle on a turn that no HTTP request owns any more.
type turnSlot struct {
	cancel context.CancelFunc
}

// turnRegistry is the set of turns running right now, keyed by session.
//
// It exists because a turn outlives the request that started it: the stream writer runs after
// its handler returned, so by the time a cancel request arrives there is nothing left to
// address it to. It also keeps a session to one turn at a time, which matters more than it
// sounds — two turns of one tailoring session would edit one CV from two conversations that
// cannot see each other.
//
// The registry is per-process and deliberately not persisted: a CancelFunc cannot be written to
// a database, and the goroutine that must act on it lives here. A turn that outlives a
// blue/green flip is therefore uncancellable and ends at its step cap instead.
// Its zero value is ready to use: the assistant handlers are assembled as a struct literal in
// several places, and a registry that had to be constructed would be nil in whichever one
// forgot — a nil map read is fine, but the first turn to write one would panic.
type turnRegistry struct {
	mu    sync.Mutex
	turns map[uuid.UUID]*turnSlot
}

// admit takes the session's slot for a turn about to start, reporting false if the session is
// already running one.
func (r *turnRegistry) admit(session uuid.UUID, cancel context.CancelFunc) (*turnSlot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, busy := r.turns[session]; busy {
		return nil, false
	}
	if r.turns == nil {
		r.turns = make(map[uuid.UUID]*turnSlot)
	}
	slot := &turnSlot{cancel: cancel}
	r.turns[session] = slot
	return slot, true
}

// release gives the slot back when the turn ends, however it ended.
//
// It clears the entry only if the session still holds this very slot: a turn that was already
// replaced must not delete its successor's entry and leave the session looking idle while a
// turn runs.
func (r *turnRegistry) release(session uuid.UUID, slot *turnSlot) {
	if slot == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if current, ok := r.turns[session]; ok && current == slot {
		delete(r.turns, session)
	}
}

// cancel stops the session's running turn, reporting whether there was one. A session with
// nothing running is not an error: a client cancels what it can no longer see the end of, and
// asking it to first prove the turn is alive would just make it guess.
func (r *turnRegistry) cancel(session uuid.UUID) bool {
	r.mu.Lock()
	slot, running := r.turns[session]
	r.mu.Unlock()

	if !running {
		return false
	}
	slot.cancel()
	return true
}

// len is the number of turns running, for the tests that guard against the registry keeping
// entries for turns that have ended.
func (r *turnRegistry) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.turns)
}
