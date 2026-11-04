package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestSitDuringCountdownJoinsStartingHand(t *testing.T) {
	cfg := testConfig()
	cfg.StartCountdown = time.Second
	table := NewTable(cfg)
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	carol := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for _, p := range []struct {
		id   string
		name string
		sink *fakeSink
	}{{"alice", "Alice", alice}, {"bob", "Bob", bob}, {"carol", "Carol", carol}} {
		if _, err := table.Join(ctx, p.id, p.name, p.sink); err != nil {
			t.Fatal(err)
		}
	}
	if err := table.Sit(ctx, "alice", nil); err != nil {
		t.Fatal(err)
	}
	if err := table.Sit(ctx, "bob", nil); err != nil {
		t.Fatal(err)
	}
	waitForSnapshotPhase(t, carol, "countdown")

	if err := table.Sit(ctx, "carol", nil); err != nil {
		t.Fatal(err)
	}
	carol.waitFor(t, protocol.TypeSeatAssigned)
	if carol.count(protocol.TypeSitPending) != 0 {
		t.Fatal("player joining during countdown must not be pending for the next hand")
	}

	message := alice.waitFor(t, protocol.TypeHandStarted)
	payload, ok := message.Payload.(protocol.HandStartedPayload)
	if !ok {
		t.Fatalf("unexpected hand.started payload: %T", message.Payload)
	}
	if len(payload.ParticipatingSeats) != 3 {
		t.Fatalf("expected three players in starting hand, got %v", payload.ParticipatingSeats)
	}
}

func waitForSnapshotPhase(t *testing.T, sink *fakeSink, phase string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, message := range sink.snapshot() {
			if message.Type != protocol.TypeSnapshot {
				continue
			}
			payload, ok := message.Payload.(protocol.SnapshotPayload)
			if ok && payload.Phase == phase {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for snapshot phase %q", phase)
}
