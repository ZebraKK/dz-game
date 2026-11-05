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

func TestIdleSnapshotClearsPreviousHandState(t *testing.T) {
	table := NewTable(testConfig())
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for _, p := range []struct {
		id   string
		name string
		sink *fakeSink
	}{{"alice", "Alice", alice}, {"bob", "Bob", bob}} {
		if _, err := table.Join(ctx, p.id, p.name, p.sink); err != nil {
			t.Fatal(err)
		}
		if err := table.Sit(ctx, p.id, nil); err != nil {
			t.Fatal(err)
		}
	}
	alice.waitFor(t, protocol.TypeHandStarted)
	requestMessage := alice.waitFor(t, protocol.TypeActionRequest)
	request, ok := requestMessage.Payload.(protocol.ActionRequestPayload)
	if !ok {
		t.Fatalf("unexpected action request payload: %T", requestMessage.Payload)
	}
	if err := table.SitOut(ctx, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := table.Action(ctx, "alice", protocol.ActionPayload{
		ActionID: "idle-view-fold",
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   protocol.PlayerAction{Type: "fold"},
	}); err != nil {
		t.Fatal(err)
	}
	alice.waitFor(t, protocol.TypeHandSettled)
	waitForSnapshotPhase(t, alice, "idle")

	snapshot := waitForSnapshot(t, alice, func(snapshot protocol.SnapshotPayload) bool {
		return snapshot.Phase == "idle" && snapshot.Pot.Total == 0
	})
	if snapshot.HandID != nil {
		t.Fatalf("idle snapshot retained handId: %v", *snapshot.HandID)
	}
	if len(snapshot.Board) != 0 {
		t.Fatalf("idle snapshot retained board: %v", snapshot.Board)
	}
	for _, seat := range snapshot.Seats {
		if seat.Occupancy != "occupied" {
			continue
		}
		if seat.StreetBet != 0 || seat.HandCommitted != 0 {
			t.Fatalf("idle snapshot retained per-hand seat state: %+v", seat)
		}
	}
}
