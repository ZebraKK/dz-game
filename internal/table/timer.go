package table

import (
	"time"

	"github.com/qbgames/qbdz-game/internal/texas"
)

// timerEntry is one pending timer inside the table goroutine.
type timerEntry struct {
	id       texas.TimerID
	deadline time.Time
}

// timerSet is a lightweight timer collection.
// All access happens from the single table goroutine — no locking needed.
// C++ reference: CSoTimerManager in Manager/SoTimerManager.h
type timerSet struct {
	entries []timerEntry
}

// Set arms a timer with the given ID to fire after d.
// If a timer with the same ID already exists, it is replaced.
func (ts *timerSet) Set(id texas.TimerID, d time.Duration) {
	ts.Remove(id)
	ts.entries = append(ts.entries, timerEntry{id: id, deadline: time.Now().Add(d)})
}

// Remove cancels a pending timer by ID. No-op if not found.
func (ts *timerSet) Remove(id texas.TimerID) {
	for i := range ts.entries {
		if ts.entries[i].id == id {
			ts.entries = append(ts.entries[:i], ts.entries[i+1:]...)
			return
		}
	}
}

// FireExpired returns the IDs of all timers whose deadline has passed,
// removing them from the set. Call this on each 100ms tick.
func (ts *timerSet) FireExpired(now time.Time) []texas.TimerID {
	var fired []texas.TimerID
	remaining := ts.entries[:0]
	for _, e := range ts.entries {
		if !now.Before(e.deadline) {
			fired = append(fired, e.id)
		} else {
			remaining = append(remaining, e)
		}
	}
	ts.entries = remaining
	return fired
}

// Has returns true if a timer with the given ID is currently armed.
func (ts *timerSet) Has(id texas.TimerID) bool {
	for _, e := range ts.entries {
		if e.id == id {
			return true
		}
	}
	return false
}
