package tableapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
	"github.com/qbgames/qbdz-game/internal/texas"
)

func TestThreePlayerMultiStreetHand(t *testing.T) {
	table := NewTable(testConfig())
	defer table.Close()
	alice := &fakeSink{}
	bob := &fakeSink{}
	carol := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	for _, player := range []struct {
		id   string
		name string
		sink *fakeSink
	}{
		{"alice", "Alice", alice},
		{"bob", "Bob", bob},
		{"carol", "Carol", carol},
	} {
		if _, err := table.Join(ctx, player.id, player.name, player.sink); err != nil {
			t.Fatal(err)
		}
		if err := table.Sit(ctx, player.id, nil); err != nil {
			t.Fatal(err)
		}
	}

	alice.waitFor(t, protocol.TypeHandStarted)
	seen := map[string]int{"alice": 0, "bob": 0, "carol": 0}
	sinks := map[string]*fakeSink{"alice": alice, "bob": bob, "carol": carol}
	actions := 0
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && alice.count(protocol.TypeHandSettled) == 0 {
		for id, sink := range sinks {
			request, next := nextActionRequest(sink, seen[id])
			if request == nil {
				continue
			}
			seen[id] = next
			action := choosePassiveAction(*request)
			if err := table.Action(ctx, id, protocol.ActionPayload{
				ActionID: fmt.Sprintf("multi-%d", actions),
				HandID:   request.HandID,
				TurnID:   request.TurnID,
				Action:   action,
			}); err != nil {
				t.Fatal(err)
			}
			actions++
		}
		time.Sleep(5 * time.Millisecond)
	}

	if alice.count(protocol.TypeHandSettled) == 0 {
		t.Fatal("hand did not settle")
	}
	if alice.count(protocol.TypeBoardDealt) < 3 {
		t.Fatalf("expected flop, turn and river events, got %d", alice.count(protocol.TypeBoardDealt))
	}
	if actions < 8 {
		t.Fatalf("expected at least 8 passive actions, got %d", actions)
	}
}

func TestBuildPotResultsMapsWinnersPerPool(t *testing.T) {
	table := &Table{participants: map[string]*participant{
		"alice": {playerID: "alice", userID: 1, seat: 0},
		"bob":   {playerID: "bob", userID: 2, seat: 1},
		"carol": {playerID: "carol", userID: 3, seat: 2},
	}}
	var eligibleA, eligibleB [texas.MaxSeats]bool
	eligibleA[0], eligibleA[1], eligibleA[2] = true, true, true
	eligibleB[1], eligibleB[2] = true, true
	pools := []texas.Pool{
		{Amount: 100, Eligible: eligibleA},
		{Amount: 200, Eligible: eligibleB},
	}
	events := []texas.Event{
		{Kind: texas.EventSettleBill, BillUserID: 2, PoolIndex: 0},
		{Kind: texas.EventSettleBill, BillUserID: 3, PoolIndex: 0},
		{Kind: texas.EventSettleBill, BillUserID: 1, PoolIndex: 1},
	}
	results := table.buildPotResults(events, pools)
	if len(results) != 2 {
		t.Fatalf("expected 2 pots, got %d", len(results))
	}
	if fmt.Sprint(results[0].WinnerSeats) != "[2 3]" {
		t.Fatalf("unexpected main pot winners: %v", results[0].WinnerSeats)
	}
	if fmt.Sprint(results[1].WinnerSeats) != "[1]" {
		t.Fatalf("unexpected side pot winners: %v", results[1].WinnerSeats)
	}
}

func choosePassiveAction(request protocol.ActionRequestPayload) protocol.PlayerAction {
	for _, action := range request.LegalActions {
		if action.Type == "check" {
			return protocol.PlayerAction{Type: "check"}
		}
	}
	for _, action := range request.LegalActions {
		if action.Type == "call" {
			return protocol.PlayerAction{Type: "call", Amount: action.Amount}
		}
	}
	return protocol.PlayerAction{Type: "fold"}
}
