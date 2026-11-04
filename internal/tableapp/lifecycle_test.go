package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestPendingSitWaitsForNextHand(t *testing.T) {
	table := NewTable(testConfig())
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
	}{{"alice", "Alice", alice}, {"bob", "Bob", bob}} {
		if _, err := table.Join(ctx, p.id, p.name, p.sink); err != nil {
			t.Fatal(err)
		}
		if err := table.Sit(ctx, p.id, nil); err != nil {
			t.Fatal(err)
		}
	}
	alice.waitFor(t, protocol.TypeHandStarted)

	if _, err := table.Join(ctx, "carol", "Carol", carol); err != nil {
		t.Fatal(err)
	}
	if err := table.Sit(ctx, "carol", nil); err != nil {
		t.Fatal(err)
	}
	carol.waitFor(t, protocol.TypeSitPending)

	playerID, request := firstActionRequest(map[string]*fakeSink{"alice": alice, "bob": bob})
	if request == nil {
		t.Fatal("missing current action request")
	}
	if err := table.Action(ctx, playerID, protocol.ActionPayload{
		ActionID: "pending-sit-fold",
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   protocol.PlayerAction{Type: "fold"},
	}); err != nil {
		t.Fatal(err)
	}
	alice.waitFor(t, protocol.TypeHandSettled)
	carol.waitFor(t, protocol.TypeSeatAssigned)
}

func TestSitOutSkipsNextHand(t *testing.T) {
	table := NewTable(testConfig())
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
	if err := table.SitOut(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	alice.waitFor(t, protocol.TypePlayerSitOut)

	playerID, request := firstActionRequest(map[string]*fakeSink{"alice": alice, "bob": bob})
	if request == nil {
		t.Fatal("missing current action request")
	}
	if err := table.Action(ctx, playerID, protocol.ActionPayload{
		ActionID: "sitout-fold",
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   protocol.PlayerAction{Type: "fold"},
	}); err != nil {
		t.Fatal(err)
	}
	alice.waitFor(t, protocol.TypeHandSettled)
	time.Sleep(150 * time.Millisecond)
	if bob.count(protocol.TypeHandStarted) != 1 {
		t.Fatalf("sitout player triggered another hand; hand.started count=%d", bob.count(protocol.TypeHandStarted))
	}
}

func firstActionRequest(sinks map[string]*fakeSink) (string, *protocol.ActionRequestPayload) {
	for id, sink := range sinks {
		if request, _ := nextActionRequest(sink, 0); request != nil {
			return id, request
		}
	}
	return "", nil
}
