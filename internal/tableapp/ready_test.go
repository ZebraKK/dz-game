package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestReadyCheckRequiredBeforeNextHand(t *testing.T) {
	cfg := testConfig()
	cfg.ResultDisplay = 10 * time.Millisecond
	cfg.ReadyTimeout = 2 * time.Second
	table := NewTable(cfg)
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	joinAndSitReadyPlayers(t, ctx, table, alice, bob)
	alice.waitFor(t, protocol.TypeHandStarted)
	requestMessage := alice.waitFor(t, protocol.TypeActionRequest)
	request := requestMessage.Payload.(protocol.ActionRequestPayload)
	if err := table.Action(ctx, "alice", protocol.ActionPayload{
		ActionID: "ready-check-fold",
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   protocol.PlayerAction{Type: "fold"},
	}); err != nil {
		t.Fatal(err)
	}
	alice.waitFor(t, protocol.TypeHandSettled)

	snapshot := waitForSnapshot(t, alice, func(snapshot protocol.SnapshotPayload) bool {
		return snapshot.Phase == "ready" && snapshot.ReadyDeadline != nil
	})
	if pointerBool(snapshot.Seats[0].Ready) || pointerBool(snapshot.Seats[1].Ready) {
		t.Fatalf("players should not be ready automatically after a hand: %+v", snapshot.Seats[:2])
	}
	time.Sleep(60 * time.Millisecond)
	if alice.count(protocol.TypeHandStarted) != 1 {
		t.Fatal("next hand started before ready confirmation")
	}

	if err := table.Ready(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if alice.count(protocol.TypeHandStarted) != 1 {
		t.Fatal("next hand started while Bob was still unready")
	}
	if err := table.Ready(ctx, "bob"); err != nil {
		t.Fatal(err)
	}
	waitForMessageCount(t, alice, protocol.TypeHandStarted, 2)
}

func TestReadyTimeoutStandsUnreadyAndPreservesChips(t *testing.T) {
	cfg := testConfig()
	cfg.ResultDisplay = 10 * time.Millisecond
	cfg.ReadyTimeout = 100 * time.Millisecond
	table := NewTable(cfg)
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	joinAndSitReadyPlayers(t, ctx, table, alice, bob)
	alice.waitFor(t, protocol.TypeHandStarted)
	requestMessage := alice.waitFor(t, protocol.TypeActionRequest)
	request := requestMessage.Payload.(protocol.ActionRequestPayload)
	if err := table.Action(ctx, "alice", protocol.ActionPayload{
		ActionID: "ready-timeout-fold",
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   protocol.PlayerAction{Type: "fold"},
	}); err != nil {
		t.Fatal(err)
	}
	alice.waitFor(t, protocol.TypeHandSettled)
	waitForSnapshot(t, alice, func(snapshot protocol.SnapshotPayload) bool {
		return snapshot.Phase == "ready"
	})

	if err := table.Ready(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	stoodMessage := alice.waitFor(t, protocol.TypePlayerStood)
	stood, ok := stoodMessage.Payload.(protocol.PlayerStoodPayload)
	if !ok || stood.PlayerID != "bob" || stood.Reason != "ready_timeout" {
		t.Fatalf("unexpected player.stood payload: %+v", stoodMessage.Payload)
	}
	waitForSnapshotPhase(t, alice, "idle")

	if err := table.Sit(ctx, "bob", nil); err != nil {
		t.Fatal(err)
	}
	snapshot := waitForSnapshot(t, alice, func(snapshot protocol.SnapshotPayload) bool {
		return snapshot.Seats[1].Occupancy == "occupied" && snapshot.Seats[1].Stack == 2010
	})
	if snapshot.Seats[1].Stack != 2010 {
		t.Fatalf("Bob's stack was not preserved: %+v", snapshot.Seats[1])
	}
}

func joinAndSitReadyPlayers(t *testing.T, ctx context.Context, table *Table, alice, bob *fakeSink) {
	t.Helper()
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
}

func waitForMessageCount(t *testing.T, sink *fakeSink, messageType string, expected int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sink.count(messageType) >= expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s count %d, got %d", messageType, expected, sink.count(messageType))
}

func pointerBool(value bool) bool {
	return value
}
