package tableapp

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestThinkingExtendsCurrentTurnOnceAndBroadcasts(t *testing.T) {
	cfg := testConfig()
	cfg.ActionTimeout = 500 * time.Millisecond
	cfg.ThinkingExtension = 300 * time.Millisecond
	cfg.ActionWarning = 100 * time.Millisecond
	table := NewTable(cfg)
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

	requestMessage := alice.waitFor(t, protocol.TypeActionRequest)
	request, ok := requestMessage.Payload.(protocol.ActionRequestPayload)
	if !ok {
		t.Fatalf("unexpected action request payload: %T", requestMessage.Payload)
	}
	if !request.ThinkingAvailable || request.ThinkingExtensionMs != 300 {
		t.Fatalf("unexpected thinking request fields: %+v", request)
	}
	oldDeadline, err := time.Parse(time.RFC3339Nano, request.Deadline)
	if err != nil {
		t.Fatal(err)
	}

	if err := table.Think(ctx, "alice"); err != nil {
		t.Fatalf("think: %v", err)
	}
	thinkingMessage := bob.waitFor(t, protocol.TypeTurnThinking)
	thinking, ok := thinkingMessage.Payload.(protocol.TurnThinkingPayload)
	if !ok {
		t.Fatalf("unexpected thinking payload: %T", thinkingMessage.Payload)
	}
	if thinking.PlayerID != "alice" || thinking.ExtensionMs != 300 {
		t.Fatalf("unexpected thinking payload: %+v", thinking)
	}
	newDeadline, err := time.Parse(time.RFC3339Nano, thinking.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	if got := newDeadline.Sub(oldDeadline); got != 300*time.Millisecond {
		t.Fatalf("unexpected extension: %s", got)
	}
	if !table.thinkingActive || !table.thinkingUsed {
		t.Fatal("expected thinking state to be active")
	}

	err = table.Think(ctx, "alice")
	appErr, ok := err.(*Error)
	if !ok || appErr.Code != protocol.ErrThinkingAlreadyUsed {
		t.Fatalf("expected THINKING_ALREADY_USED, got %v", err)
	}
}

func TestActionWarningIsPrivateAndOffersThinking(t *testing.T) {
	cfg := testConfig()
	cfg.ActionTimeout = 250 * time.Millisecond
	cfg.ThinkingExtension = 200 * time.Millisecond
	cfg.ActionWarning = 80 * time.Millisecond
	table := NewTable(cfg)
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
	alice.waitFor(t, protocol.TypeActionRequest)

	warningMessage := alice.waitFor(t, protocol.TypeActionWarning)
	warning, ok := warningMessage.Payload.(protocol.ActionWarningPayload)
	if !ok {
		t.Fatalf("unexpected warning payload: %T", warningMessage.Payload)
	}
	if warning.RemainingMs <= 0 || warning.RemainingMs > 80 {
		t.Fatalf("unexpected remaining time: %d", warning.RemainingMs)
	}
	if !warning.ThinkingAvailable || warning.ThinkingExtensionMs != 200 {
		t.Fatalf("unexpected warning thinking fields: %+v", warning)
	}
	if bob.count(protocol.TypeActionWarning) != 0 {
		t.Fatal("action warning should be private to the current actor")
	}
	if err := table.Think(ctx, "alice"); err != nil {
		t.Fatalf("think after warning: %v", err)
	}
}
