package tableapp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

func TestNineSeatsFillAndTenthPlayerIsRejected(t *testing.T) {
	cfg := testConfig()
	cfg.StartCountdown = time.Second
	table := NewTable(cfg)
	defer table.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sinks := make([]*fakeSink, 0, 10)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("player-%d", i+1)
		sink := &fakeSink{}
		sinks = append(sinks, sink)
		if _, err := table.Join(ctx, id, id, sink); err != nil {
			t.Fatal(err)
		}
		if i < 9 {
			if err := table.Sit(ctx, id, nil); err != nil {
				t.Fatalf("sit %s: %v", id, err)
			}
		}
	}

	err := table.Sit(ctx, "player-10", nil)
	var appErr *Error
	if !errors.As(err, &appErr) || appErr.Code != protocol.ErrTableFull {
		t.Fatalf("expected TABLE_FULL for tenth player, got %v", err)
	}
	waitForOccupiedSeats(t, sinks[0], MaxSeats)
}

func waitForOccupiedSeats(t *testing.T, sink *fakeSink, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		messages := sink.snapshot()
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Type != protocol.TypeSnapshot {
				continue
			}
			snapshot, ok := messages[i].Payload.(protocol.SnapshotPayload)
			if !ok {
				t.Fatalf("unexpected snapshot payload: %T", messages[i].Payload)
			}
			occupied := 0
			for _, seat := range snapshot.Seats {
				if seat.Occupancy == "occupied" {
					occupied++
				}
			}
			if occupied == expected {
				return
			}
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected %d occupied seats", expected)
}
