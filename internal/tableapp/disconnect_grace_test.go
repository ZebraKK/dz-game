package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestDisconnectGraceExpiresToSitOut(t *testing.T) {
	cfg := testConfig()
	cfg.ActionTimeout = 5 * time.Second
	cfg.DisconnectGrace = 80 * time.Millisecond
	cfg.ResultDisplay = 20 * time.Millisecond
	table := NewTable(cfg)
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
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
	table.Disconnect("alice", alice)

	event := bob.waitFor(t, protocol.TypePlayerSitOut)
	payload, ok := event.Payload.(protocol.PlayerSitOutPayload)
	if !ok || payload.PlayerID != "alice" || !payload.SitOut {
		t.Fatalf("unexpected sitout event: %+v", event.Payload)
	}
	bob.waitFor(t, protocol.TypeHandSettled)
	waitForSnapshotPhase(t, bob, "idle")

	reconnected := &fakeSink{}
	join, err := table.Join(ctx, "alice", "Alice", reconnected)
	if err != nil {
		t.Fatal(err)
	}
	if join.Role != "player" || join.Seat == nil || *join.Seat != 1 {
		t.Fatalf("unexpected reconnect join: %+v", join)
	}
	snapshotMessage := reconnected.waitFor(t, protocol.TypeSnapshot)
	snapshot, ok := snapshotMessage.Payload.(protocol.SnapshotPayload)
	if !ok {
		t.Fatalf("unexpected snapshot payload: %T", snapshotMessage.Payload)
	}
	seat := snapshotSeat(snapshot, 1)
	if seat == nil || seat.Participation == nil || *seat.Participation != "sitting_out" {
		t.Fatalf("expired disconnect should remain seated but sitting out: %+v", seat)
	}
	if seat.Connection == nil || *seat.Connection != "online" {
		t.Fatalf("reconnected player should be online: %+v", seat)
	}

	if err := table.Sit(ctx, "alice", nil); err != nil {
		t.Fatal(err)
	}
	if !hasSitOutEvent(bob, "alice", false) {
		t.Fatal("explicit sit should resume participation after disconnect expiry")
	}
}

func TestReconnectWithinGraceCancelsSitOutExpiry(t *testing.T) {
	cfg := testConfig()
	cfg.ActionTimeout = 5 * time.Second
	cfg.DisconnectGrace = 150 * time.Millisecond
	table := NewTable(cfg)
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
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
	table.Disconnect("alice", alice)
	reconnected := &fakeSink{}
	if _, err := table.Join(ctx, "alice", "Alice", reconnected); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * cfg.DisconnectGrace)
	if hasSitOutEvent(bob, "alice", true) {
		t.Fatal("reconnect within grace must not force sitout")
	}
}

func snapshotSeat(snapshot protocol.SnapshotPayload, seatNumber int) *protocol.SeatView {
	for i := range snapshot.Seats {
		if snapshot.Seats[i].Seat == seatNumber {
			return &snapshot.Seats[i]
		}
	}
	return nil
}

func hasSitOutEvent(sink *fakeSink, playerID string, sitOut bool) bool {
	for _, message := range sink.snapshot() {
		if message.Type != protocol.TypePlayerSitOut {
			continue
		}
		payload, ok := message.Payload.(protocol.PlayerSitOutPayload)
		if ok && payload.PlayerID == playerID && payload.SitOut == sitOut {
			return true
		}
	}
	return false
}
