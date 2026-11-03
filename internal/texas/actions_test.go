package texas

import (
	"testing"

	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/player"
)

func newActionTestGame(stacks ...uint64) *Game {
	cfg := config.DefaultSceneConfig()
	cfg.SeatCount = 9
	cfg.SmallBlind = 10
	cfg.BigBlind = 20
	g := NewGame(&cfg)
	for i, stack := range stacks {
		p := player.NewPlayer(uint32(i+1), string(rune('A'+i)), "", stack)
		p.IsReady = true
		g.SeatPlayer(i, p)
	}
	g.StartHand()
	return g
}

func TestLegalActionsPreflop(t *testing.T) {
	g := newActionTestGame(2000, 2000, 2000)
	actions := g.LegalActions(0)
	if len(actions) != 4 {
		t.Fatalf("expected 4 legal actions, got %d: %+v", len(actions), actions)
	}
	if actions[0].Action != player.ActionFold {
		t.Fatalf("expected fold first, got %v", actions[0].Action)
	}
	if actions[1].Action != player.ActionCall || actions[1].Amount != 20 {
		t.Fatalf("unexpected call: %+v", actions[1])
	}
	if actions[2].Action != player.ActionRaise || actions[2].MinAmount != 40 || actions[2].MaxAmount != 2000 {
		t.Fatalf("unexpected raise: %+v", actions[2])
	}
	if actions[3].Action != player.ActionAllIn || actions[3].Amount != 2000 {
		t.Fatalf("unexpected all-in: %+v", actions[3])
	}
}

func TestApplyActionRejectsInvalidCheck(t *testing.T) {
	g := newActionTestGame(2000, 2000)
	_, err := g.ApplyAction(0, player.ActionCheck, 0)
	rej, ok := err.(*ActionRejection)
	if !ok || rej.Code != RejectInvalidAction || rej.Reason != "CHECK_NOT_ALLOWED" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestApplyActionRaiseUpdatesMinimum(t *testing.T) {
	g := newActionTestGame(2000, 2000, 2000)
	if _, err := g.ApplyAction(0, player.ActionRaise, 60); err != nil {
		t.Fatalf("raise failed: %v", err)
	}
	if g.CurrentBet() != 60 || g.LastFullRaise() != 40 {
		t.Fatalf("unexpected bet state: max=%d fullRaise=%d", g.CurrentBet(), g.LastFullRaise())
	}
	actions := g.LegalActions(g.CurrentSeat())
	var raise LegalAction
	for _, a := range actions {
		if a.Action == player.ActionRaise {
			raise = a
		}
	}
	if raise.MinAmount != 100 {
		t.Fatalf("expected min raise to 100, got %+v", raise)
	}
}

func TestApplyActionAllInRequiresExactStack(t *testing.T) {
	g := newActionTestGame(500, 2000)
	stack := g.SeatState(0).Chips
	_, err := g.ApplyAction(0, player.ActionAllIn, stack-1)
	rej, ok := err.(*ActionRejection)
	if !ok || rej.Reason != "ALL_IN_AMOUNT_MISMATCH" {
		t.Fatalf("unexpected error: %#v", err)
	}
	if _, err := g.ApplyAction(0, player.ActionAllIn, stack); err != nil {
		t.Fatalf("all-in failed: %v", err)
	}
}
