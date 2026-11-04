package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestActionTimeoutAutoFoldsWhenCallRequired(t *testing.T) {
	table := NewTable(timeoutConfig())
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
	result := waitForAutoAction(t, alice, "fold")
	if result.PlayerID != "alice" {
		t.Fatalf("unexpected auto-fold player: %+v", result)
	}
	alice.waitFor(t, protocol.TypeHandSettled)
}

func TestActionTimeoutAutoChecksWhenNoCallRequired(t *testing.T) {
	table := NewTable(timeoutConfig())
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
	requestMessage := alice.waitFor(t, protocol.TypeActionRequest)
	request, ok := requestMessage.Payload.(protocol.ActionRequestPayload)
	if !ok {
		t.Fatalf("unexpected action request payload: %T", requestMessage.Payload)
	}
	callAmount, ok := legalAmount(request.LegalActions, "call")
	if !ok {
		t.Fatal("expected a call action in the initial action request")
	}
	if err := table.Action(ctx, "alice", protocol.ActionPayload{
		ActionID: "timeout-check-call",
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   protocol.PlayerAction{Type: "call", Amount: callAmount},
	}); err != nil {
		t.Fatal(err)
	}

	result := waitForAutoAction(t, bob, "check")
	if result.PlayerID != "bob" {
		t.Fatalf("unexpected auto-check player: %+v", result)
	}
	bob.waitFor(t, protocol.TypeBoardDealt)
	bob.waitFor(t, protocol.TypeHandSettled)
}

func timeoutConfig() Config {
	cfg := testConfig()
	cfg.StartCountdown = 10 * time.Millisecond
	cfg.ActionTimeout = 60 * time.Millisecond
	cfg.ResultDisplay = 10 * time.Millisecond
	return cfg
}

func waitForAutoAction(t *testing.T, sink *fakeSink, actionType string) protocol.ActionResultPayload {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, message := range sink.snapshot() {
			if message.Type != protocol.TypeActionResult {
				continue
			}
			result, ok := message.Payload.(protocol.ActionResultPayload)
			if ok && result.Auto && result.Action.Type == actionType {
				return result
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for auto %s", actionType)
	return protocol.ActionResultPayload{}
}

func legalAmount(actions []protocol.LegalAction, actionType string) (uint64, bool) {
	for _, action := range actions {
		if action.Type == actionType {
			return action.Amount, true
		}
	}
	return 0, false
}
