package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestIdleSnapshotHasNoHandMarkers(t *testing.T) {
	table := NewTable(testConfig())
	defer table.Close()
	sink := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := table.Join(ctx, "alice", "Alice", sink); err != nil {
		t.Fatal(err)
	}
	if err := table.Sit(ctx, "alice", nil); err != nil {
		t.Fatal(err)
	}

	snapshot := waitForSnapshot(t, sink, func(snapshot protocol.SnapshotPayload) bool {
		return snapshot.Seats[0].Occupancy == "occupied"
	})
	seat := snapshot.Seats[0]
	if seat.IsDealer || seat.IsSmallBlind || seat.IsBigBlind {
		t.Fatalf("idle seat unexpectedly has hand markers: %+v", seat)
	}
}

func waitForSnapshot(t *testing.T, sink *fakeSink, predicate func(protocol.SnapshotPayload) bool) protocol.SnapshotPayload {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, message := range sink.snapshot() {
			if message.Type != protocol.TypeSnapshot {
				continue
			}
			snapshot, ok := message.Payload.(protocol.SnapshotPayload)
			if ok && predicate(snapshot) {
				return snapshot
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for snapshot")
	return protocol.SnapshotPayload{}
}
