package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestStaleTurnAndActionIdempotency(t *testing.T) {
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
	req, _ := nextActionRequest(alice, 0)
	if req == nil {
		t.Fatal("missing alice action request")
	}

	if err := table.Action(ctx, "alice", protocol.ActionPayload{
		ActionID: "stale-action",
		HandID:   req.HandID,
		TurnID:   "wrong-turn",
		Action:   protocol.PlayerAction{Type: "fold"},
	}); err != nil {
		t.Fatal(err)
	}
	errorMessage := alice.waitFor(t, protocol.TypeError)
	errorPayload, ok := errorMessage.Payload.(protocol.ErrorPayload)
	if !ok || errorPayload.Code != protocol.ErrStaleTurn {
		t.Fatalf("expected STALE_TURN, got %#v", errorMessage.Payload)
	}

	call := protocol.LegalAction{}
	for _, action := range req.LegalActions {
		if action.Type == "call" {
			call = action
			break
		}
	}
	if call.Type == "" {
		t.Fatalf("missing call action: %+v", req.LegalActions)
	}
	payload := protocol.ActionPayload{
		ActionID: "same-action",
		HandID:   req.HandID,
		TurnID:   req.TurnID,
		Action:   protocol.PlayerAction{Type: "call", Amount: call.Amount},
	}
	if err := table.Action(ctx, "alice", payload); err != nil {
		t.Fatal(err)
	}
	if err := table.Action(ctx, "alice", payload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if actionResultCount(alice, "same-action") == 2 && actionResultCount(bob, "same-action") == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("unexpected result counts: alice=%d bob=%d", actionResultCount(alice, "same-action"), actionResultCount(bob, "same-action"))
}

func actionResultCount(sink *fakeSink, actionID string) int {
	count := 0
	for _, msg := range sink.snapshot() {
		if msg.Type != protocol.TypeActionResult {
			continue
		}
		payload, ok := msg.Payload.(protocol.ActionResultPayload)
		if ok && payload.ActionID == actionID {
			count++
		}
	}
	return count
}
