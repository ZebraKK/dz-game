package tableapp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

type sentMessage struct {
	Type         string
	StateVersion int64
	Payload      any
}

type fakeSink struct {
	mu       sync.Mutex
	messages []sentMessage
}

func (f *fakeSink) Send(messageType string, stateVersion int64, payload any) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, sentMessage{Type: messageType, StateVersion: stateVersion, Payload: payload})
	return true
}

func (f *fakeSink) snapshot() []sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMessage(nil), f.messages...)
}

func (f *fakeSink) waitFor(t *testing.T, messageType string) sentMessage {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, msg := range f.snapshot() {
			if msg.Type == messageType {
				return msg
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", messageType)
	return sentMessage{}
}

func (f *fakeSink) count(messageType string) int {
	count := 0
	for _, msg := range f.snapshot() {
		if msg.Type == messageType {
			count++
		}
	}
	return count
}

func testConfig() Config {
	cfg := DefaultConfig()
	cfg.StartCountdown = 10 * time.Millisecond
	cfg.ResultDisplay = 25 * time.Millisecond
	cfg.ActionTimeout = 2 * time.Second
	return cfg
}

func TestHeadsUpHandCompletesAndSpectatorCannotSeePrivateState(t *testing.T) {
	table := NewTable(testConfig())
	defer table.Close()
	recorder := &recordingRecorder{}
	table.SetRecorder(recorder)

	alice := &fakeSink{}
	bob := &fakeSink{}
	spectator := &fakeSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := table.Join(ctx, "alice", "Alice", alice); err != nil {
		t.Fatalf("join alice: %v", err)
	}
	if _, err := table.Join(ctx, "bob", "Bob", bob); err != nil {
		t.Fatalf("join bob: %v", err)
	}
	if _, err := table.Join(ctx, "carol", "Carol", spectator); err != nil {
		t.Fatalf("join spectator: %v", err)
	}
	if err := table.Sit(ctx, "alice", nil); err != nil {
		t.Fatalf("sit alice: %v", err)
	}
	if err := table.Sit(ctx, "bob", nil); err != nil {
		t.Fatalf("sit bob: %v", err)
	}

	alice.waitFor(t, protocol.TypeHandStarted)
	bob.waitFor(t, protocol.TypeHandStarted)
	spectator.waitFor(t, protocol.TypeHandStarted)

	seenAlice := 0
	seenBob := 0
	actions := 0
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if alice.count(protocol.TypeHandSettled) > 0 || bob.count(protocol.TypeHandSettled) > 0 {
			break
		}
		if req, next := nextActionRequest(alice, seenAlice); req != nil {
			seenAlice = next
			sendAction(t, table, "alice", *req, actions)
			actions++
		}
		if req, next := nextActionRequest(bob, seenBob); req != nil {
			seenBob = next
			sendAction(t, table, "bob", *req, actions)
			actions++
		}
		time.Sleep(5 * time.Millisecond)
	}

	if alice.count(protocol.TypeHandSettled) == 0 {
		t.Fatal("alice did not receive hand.settled")
	}
	if bob.count(protocol.TypeHandSettled) == 0 {
		t.Fatal("bob did not receive hand.settled")
	}
	if spectator.count(protocol.TypeHandSettled) == 0 {
		t.Fatal("spectator did not receive hand.settled")
	}
	if alice.count(protocol.TypeHoleCards) != 1 || bob.count(protocol.TypeHoleCards) != 1 {
		t.Fatalf("unexpected hole card counts: alice=%d bob=%d", alice.count(protocol.TypeHoleCards), bob.count(protocol.TypeHoleCards))
	}
	if spectator.count(protocol.TypeHoleCards) != 0 {
		t.Fatal("spectator received private hole cards")
	}
	if spectator.count(protocol.TypeActionRequest) != 0 {
		t.Fatal("spectator received action request")
	}
	for _, msg := range spectator.snapshot() {
		if msg.Type != protocol.TypeSnapshot {
			continue
		}
		snapshot, ok := msg.Payload.(protocol.SnapshotPayload)
		if !ok {
			t.Fatalf("unexpected snapshot payload type %T", msg.Payload)
		}
		if len(snapshot.You.HoleCards) != 0 {
			t.Fatalf("spectator snapshot leaked hole cards: %+v", snapshot.You.HoleCards)
		}
	}
	if actions < 2 {
		t.Fatalf("expected at least two actions, got %d", actions)
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) && recorder.count() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	records := recorder.all()
	if len(records) != 1 {
		t.Fatalf("expected one hand record, got %d", len(records))
	}
	if records[0].HandID == "" || len(records[0].Players) != 2 || len(records[0].Actions) < 2 {
		t.Fatalf("incomplete hand record: %+v", records[0])
	}
	for _, player := range records[0].Players {
		if len(player.HoleCards) != 2 {
			t.Fatalf("hand record missing private cards: %+v", player)
		}
	}
}

type recordingRecorder struct {
	mu      sync.Mutex
	records []HandRecord
}

func (r *recordingRecorder) RecordHand(record HandRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

func (r *recordingRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

func (r *recordingRecorder) all() []HandRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]HandRecord(nil), r.records...)
}

func nextActionRequest(sink *fakeSink, seen int) (*protocol.ActionRequestPayload, int) {
	messages := sink.snapshot()
	for i := seen; i < len(messages); i++ {
		if messages[i].Type != protocol.TypeActionRequest {
			continue
		}
		request, ok := messages[i].Payload.(protocol.ActionRequestPayload)
		if !ok {
			continue
		}
		return &request, i + 1
	}
	return nil, seen
}

func sendAction(t *testing.T, table *Table, playerID string, request protocol.ActionRequestPayload, seq int) {
	t.Helper()
	action := protocol.PlayerAction{Type: "fold"}
	for _, legal := range request.LegalActions {
		if legal.Type == "all_in" {
			action = protocol.PlayerAction{Type: "all_in", Amount: legal.Amount}
			break
		}
		if legal.Type == "check" {
			action = protocol.PlayerAction{Type: "check"}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := table.Action(ctx, playerID, protocol.ActionPayload{
		ActionID: fmt.Sprintf("action-%d", seq),
		HandID:   request.HandID,
		TurnID:   request.TurnID,
		Action:   action,
	})
	if err != nil {
		t.Fatalf("action %s: %v", playerID, err)
	}
}
