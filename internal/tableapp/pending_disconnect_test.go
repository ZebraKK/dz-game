package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestDisconnectGraceCancelsPendingSit(t *testing.T) {
	cfg := testConfig()
	cfg.ActionTimeout = 5 * time.Second
	cfg.DisconnectGrace = 80 * time.Millisecond
	table := NewTable(cfg)
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	carol := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
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
	for _, id := range []string{"alice", "bob"} {
		if err := table.Sit(ctx, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	alice.waitFor(t, protocol.TypeHandStarted)
	if err := table.Sit(ctx, "carol", nil); err != nil {
		t.Fatal(err)
	}
	carol.waitFor(t, protocol.TypeSitPending)
	table.Disconnect("carol", carol)
	time.Sleep(2 * cfg.DisconnectGrace)

	reconnected := &fakeSink{}
	join, err := table.Join(ctx, "carol", "Carol", reconnected)
	if err != nil {
		t.Fatal(err)
	}
	if join.PendingSit {
		t.Fatal("expired pending sit must be cancelled after disconnect grace")
	}
	if join.Role != "spectator" || join.Seat != nil {
		t.Fatalf("expected spectator after pending sit expiry, got %+v", join)
	}
}
