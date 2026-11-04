package tableapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestThreePlayerBetAndCheckRaiseFlow(t *testing.T) {
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
	}{{"alice", "Alice", alice}, {"bob", "Bob", bob}, {"carol", "Carol", carol}} {
		if _, err := table.Join(ctx, p.id, p.name, p.sink); err != nil {
			t.Fatal(err)
		}
		if err := table.Sit(ctx, p.id, nil); err != nil {
			t.Fatal(err)
		}
	}

	started := alice.waitFor(t, protocol.TypeHandStarted)
	hand, ok := started.Payload.(protocol.HandStartedPayload)
	if !ok {
		t.Fatalf("unexpected hand.started payload: %T", started.Payload)
	}
	if hand.ButtonSeat != 1 || hand.SmallBlindSeat != 2 || hand.BigBlindSeat != 3 {
		t.Fatalf("unexpected three-player positions: button=%d sb=%d bb=%d", hand.ButtonSeat, hand.SmallBlindSeat, hand.BigBlindSeat)
	}

	seen := map[string]int{}
	aliceRequest := waitNextRequest(t, "alice", alice, seen)
	if aliceRequest.Seat != 1 {
		t.Fatalf("button must act first preflop in three-handed play, got seat %d", aliceRequest.Seat)
	}
	act(t, table, "alice", aliceRequest, "call", callAmount(t, aliceRequest), "preflop-alice-call")
	bobRequest := waitNextRequest(t, "bob", bob, seen)
	act(t, table, "bob", bobRequest, "call", callAmount(t, bobRequest), "preflop-bob-call")
	carolRequest := waitNextRequest(t, "carol", carol, seen)
	act(t, table, "carol", carolRequest, "check", 0, "preflop-carol-check")

	bobRequest = waitNextRequest(t, "bob", bob, seen)
	act(t, table, "bob", bobRequest, "check", 0, "flop-bob-check")
	carolRequest = waitNextRequest(t, "carol", carol, seen)
	act(t, table, "carol", carolRequest, "check", 0, "flop-carol-check")
	aliceRequest = waitNextRequest(t, "alice", alice, seen)
	act(t, table, "alice", aliceRequest, "raise", 20, "flop-alice-bet")
	bobRequest = waitNextRequest(t, "bob", bob, seen)
	act(t, table, "bob", bobRequest, "raise", 60, "flop-bob-check-raise")
	carolRequest = waitNextRequest(t, "carol", carol, seen)
	act(t, table, "carol", carolRequest, "fold", 0, "flop-carol-fold")
	aliceRequest = waitNextRequest(t, "alice", alice, seen)
	if aliceRequest.ToCall != 40 {
		t.Fatalf("expected Alice to call 40 after check-raise, got %d", aliceRequest.ToCall)
	}
	act(t, table, "alice", aliceRequest, "call", 40, "flop-alice-call")

	turnRequest := waitNextRequest(t, "bob", bob, seen)
	if turnRequest.Pot != 180 {
		t.Fatalf("expected pot 180 after check-raise and call, got %d", turnRequest.Pot)
	}
	if turnRequest.Seat != 2 {
		t.Fatalf("expected small blind to act first on turn, got seat %d", turnRequest.Seat)
	}
}

func waitNextRequest(t *testing.T, playerID string, sink *fakeSink, seen map[string]int) protocol.ActionRequestPayload {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		request, next := nextActionRequest(sink, seen[playerID])
		if request != nil {
			seen[playerID] = next
			return *request
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s action request", playerID)
	return protocol.ActionRequestPayload{}
}

func act(t *testing.T, table *Table, playerID string, request protocol.ActionRequestPayload, actionType string, amount uint64, actionID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	action := protocol.PlayerAction{Type: actionType, Amount: amount}
	if actionType == "check" || actionType == "fold" {
		action.Amount = 0
	}
	if err := table.Action(ctx, playerID, protocol.ActionPayload{
		ActionID: fmt.Sprintf("%s-%s", actionID, request.TurnID),
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   action,
	}); err != nil {
		t.Fatalf("%s action: %v", playerID, err)
	}
}

func callAmount(t *testing.T, request protocol.ActionRequestPayload) uint64 {
	t.Helper()
	for _, action := range request.LegalActions {
		if action.Type == "call" {
			return action.Amount
		}
	}
	t.Fatal("call action missing")
	return 0
}
