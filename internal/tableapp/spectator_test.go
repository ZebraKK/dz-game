package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestSnapshotIncludesSpectators(t *testing.T) {
	table := NewTable(testConfig())
	defer table.Close()
	alice := &fakeSink{}
	carol := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := table.Join(ctx, "alice", "Alice", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Join(ctx, "carol", "Carol", carol); err != nil {
		t.Fatal(err)
	}
	message := carol.waitFor(t, protocol.TypeSnapshot)
	snapshot, ok := message.Payload.(protocol.SnapshotPayload)
	if !ok {
		t.Fatalf("unexpected snapshot payload: %T", message.Payload)
	}
	found := false
	for _, spectator := range snapshot.Spectators {
		if spectator.PlayerID == "carol" && spectator.DisplayName == "Carol" && spectator.Connection == "online" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Carol missing from spectators: %+v", snapshot.Spectators)
	}
}
