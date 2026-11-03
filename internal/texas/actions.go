package texas

import (
	"fmt"

	"github.com/qbgames/qbdz-game/internal/cards"
	"github.com/qbgames/qbdz-game/internal/player"
)

// LegalAction describes one action available to the player whose turn it is.
// Amounts are integer chips. Raise amounts are total street commitments.
type LegalAction struct {
	Action    player.Action
	Amount    uint64
	MinAmount uint64
	MaxAmount uint64
}

// ActionRejection is a typed rejection from the game rules layer.
type ActionRejection struct {
	Code   string
	Reason string
}

func (e *ActionRejection) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Reason)
}

const (
	RejectNotYourTurn   = "NOT_YOUR_TURN"
	RejectInvalidAction = "INVALID_ACTION"
)

// SeatState is a read-only snapshot of one seat.
type SeatState struct {
	Occupied      bool
	UserID        uint32
	NickName      string
	Chips         uint64
	StreetBet     uint64
	HandCommitted uint64
	InHand        bool
	Active        bool
	AllIn         bool
	Ready         bool
	Action        player.Action
	LastAction    player.Action
	ActionIsAuto  bool
}

func (g *Game) SeatState(seat int) SeatState {
	if seat < 0 || seat >= MaxSeats {
		return SeatState{}
	}
	p := g.players[seat]
	if p == nil {
		return SeatState{}
	}
	return SeatState{
		Occupied:      true,
		UserID:        p.UserID,
		NickName:      p.NickName,
		Chips:         p.ChipDesk,
		StreetBet:     g.streetChips[seat],
		HandCommitted: g.chipsIn[seat],
		InHand:        p.InGame,
		Active:        g.active[seat] && p.Action != player.ActionFold,
		AllIn:         g.allIn[seat] || p.Action == player.ActionAllIn,
		Ready:         p.IsReady,
		Action:        p.Action,
		LastAction:    p.LastAction,
		ActionIsAuto:  p.ActionIsAuto,
	}
}

func (g *Game) DealerSeat() int       { return g.dealerSeat }
func (g *Game) SmallBlindSeat() int   { return g.sbSeat }
func (g *Game) BigBlindSeat() int     { return g.bbSeat }
func (g *Game) CurrentBet() uint64    { return g.maxBet }
func (g *Game) LastFullRaise() uint64 { return g.lastFullRaise }

func (g *Game) Pots() []Pool {
	return ComputePots(g.chipsIn, g.allIn, g.active)
}

func (g *Game) PlayerCards(seat int) [2]cards.Card { return g.HoleCards(seat) }

// LegalActions returns the actions currently allowed for seat.
// An empty result means the seat is not the current actor.
func (g *Game) LegalActions(seat int) []LegalAction {
	if g.status < StatusPreFlop || g.status > StatusRiver {
		return nil
	}
	if seat < 0 || seat >= MaxSeats || seat != g.currentSeat {
		return nil
	}
	p := g.players[seat]
	if p == nil || !p.InGame || p.Action == player.ActionFold || g.allIn[seat] {
		return nil
	}

	toCall := g.maxBet - g.streetChips[seat]
	actions := []LegalAction{{Action: player.ActionFold}}
	if toCall == 0 {
		actions = append(actions, LegalAction{Action: player.ActionCheck})
	}
	if p.ChipDesk > 0 {
		callAmount := minUint64(toCall, p.ChipDesk)
		if callAmount > 0 {
			actions = append(actions, LegalAction{Action: player.ActionCall, Amount: callAmount})
		}

		maxTotal := g.streetChips[seat] + p.ChipDesk
		if maxTotal > g.maxBet {
			minTotal := g.minRaiseTo()
			if maxTotal >= minTotal {
				actions = append(actions, LegalAction{
					Action:    player.ActionRaise,
					MinAmount: minTotal,
					MaxAmount: maxTotal,
				})
			}
		}
		actions = append(actions, LegalAction{Action: player.ActionAllIn, Amount: p.ChipDesk})
	}
	return actions
}

func (g *Game) minRaiseTo() uint64 {
	if g.maxBet == 0 {
		return g.blindInfo.BigBlind
	}
	increment := g.lastFullRaise
	if increment < g.blindInfo.BigBlind {
		increment = g.blindInfo.BigBlind
	}
	return g.maxBet + increment
}

// ApplyAction validates an action against the current legal action set and
// applies it to the game. The legacy PlayerAction method remains available for
// older callers, while the new gateway uses this strict path.
func (g *Game) ApplyAction(seat int, action player.Action, amount uint64) ([]Event, error) {
	if g.status < StatusPreFlop || g.status > StatusRiver {
		return nil, &ActionRejection{Code: RejectNotYourTurn, Reason: "HAND_NOT_ACTIVE"}
	}
	if seat < 0 || seat >= MaxSeats || seat != g.currentSeat {
		return nil, &ActionRejection{Code: RejectNotYourTurn, Reason: "NOT_CURRENT_ACTOR"}
	}
	p := g.players[seat]
	if p == nil || !p.InGame || p.Action == player.ActionFold || g.allIn[seat] {
		return nil, &ActionRejection{Code: RejectInvalidAction, Reason: "PLAYER_NOT_ACTIVE"}
	}

	toCall := g.maxBet - g.streetChips[seat]
	switch action {
	case player.ActionFold:
		if amount != 0 {
			return nil, invalid("FOLD_AMOUNT_MUST_BE_ZERO")
		}
		return g.PlayerAction(seat, player.ActionFold, 0), nil
	case player.ActionCheck:
		if toCall != 0 {
			return nil, invalid("CHECK_NOT_ALLOWED")
		}
		if amount != 0 {
			return nil, invalid("CHECK_AMOUNT_MUST_BE_ZERO")
		}
		return g.PlayerAction(seat, player.ActionCheck, 0), nil
	case player.ActionCall:
		expected := minUint64(toCall, p.ChipDesk)
		if expected == 0 || amount != expected {
			return nil, invalid("CALL_AMOUNT_MISMATCH")
		}
		return g.PlayerAction(seat, player.ActionCall, 0), nil
	case player.ActionRaise:
		maxTotal := g.streetChips[seat] + p.ChipDesk
		minTotal := g.minRaiseTo()
		if g.maxBet == 0 {
			minTotal = g.blindInfo.BigBlind
		}
		if amount < minTotal {
			return nil, invalid("RAISE_BELOW_MINIMUM")
		}
		if amount > maxTotal {
			return nil, invalid("RAISE_ABOVE_STACK")
		}
		return g.PlayerAction(seat, player.ActionRaise, amount), nil
	case player.ActionAllIn:
		if p.ChipDesk == 0 || amount != p.ChipDesk {
			return nil, invalid("ALL_IN_AMOUNT_MISMATCH")
		}
		return g.PlayerAction(seat, player.ActionAllIn, 0), nil
	default:
		return nil, invalid("ACTION_NOT_LEGAL")
	}
}

func invalid(reason string) *ActionRejection {
	return &ActionRejection{Code: RejectInvalidAction, Reason: reason}
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
