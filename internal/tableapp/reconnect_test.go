package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestReconnectRestoresSeatAndHoleCards(t *testing.T) {
	table := NewTable(testConfig())
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := table.Join(ctx, "alice", "Alice", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Join(ctx, "bob", "Bob", bob); err != nil {
		t.Fatal(err)
	}
	if err := table.Sit(ctx, "alice", nil); err != nil {
		t.Fatal(err)
	}
	if err := table.Sit(ctx, "bob", nil); err != nil {
		t.Fatal(err)
	}
	alice.waitFor(t, protocol.TypeHandStarted)

	table.Disconnect("alice", alice)
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
	if len(snapshot.You.HoleCards) != 2 {
		t.Fatalf("expected two private cards after reconnect, got %+v", snapshot.You.HoleCards)
	}

	table.Disconnect("alice", reconnected)
}
