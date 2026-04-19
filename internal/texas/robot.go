package texas

import (
	"math/rand"

	"github.com/qbgames/qbdz-game/internal/cards"
	"github.com/qbgames/qbdz-game/internal/player"
)

// Robot makes betting decisions for an AI player at a Texas Hold'em table.
// Decisions are rule-based and deterministic given the same RNG seed.
//
// C++ reference: CSoRobotGameLogic / RobotAI in Source/SoGameLogic/
type Robot struct {
	rng *rand.Rand
}

// NewRobot creates a Robot with the given random source.
func NewRobot(rng *rand.Rand) *Robot {
	return &Robot{rng: rng}
}

// Decide returns the action and amount for the robot at the given seat.
// It reads the game state through exported accessor methods.
// The returned amount is only meaningful for Bet and Raise actions.
func (r *Robot) Decide(g *Game, seat int) (player.Action, uint64) {
	p := g.players[seat]
	if p == nil {
		return player.ActionFold, 0
	}

	toCall := g.ToCall()
	pot := g.PotSize()
	bb := g.BigBlind()
	hole := g.HoleCards(seat)
	community := g.Community()

	// ── Pre-flop ──────────────────────────────────────────────────────────────
	if len(community) == 0 {
		return r.decidePrefop(hole, toCall, pot, bb, p.ChipDesk)
	}

	// ── Post-flop ─────────────────────────────────────────────────────────────
	return r.decidePostflop(hole, community, toCall, pot, bb, p.ChipDesk)
}

// ── Pre-flop decision ─────────────────────────────────────────────────────────

// preflopStrength returns a hand strength score (0–30) for two hole cards.
// Higher = stronger. Thresholds:
//
//	>= 20  premium (AA, KK, QQ, JJ, AK)
//	>= 12  strong (TT, 99, AQ, AJ, KQ)
//	>=  7  medium (88, 77, AT, KJ, suited connectors)
//	<   7  weak
func preflopStrength(c [2]cards.Card) int {
	a, b := c[0], c[1]
	// Ensure a is the higher card
	if a.Level < b.Level {
		a, b = b, a
	}
	suited := a.Shape == b.Shape
	pair := a.Number == b.Number

	score := int(a.Level) // 2..14

	if pair {
		// Pairs score double their rank value
		score = int(a.Level) * 2
		return score
	}

	// Suited bonus
	if suited {
		score += 2
	}

	// Kicker bonus — connected hands (gap <= 1) get a small bonus
	gap := int(a.Level) - int(b.Level)
	if gap == 1 {
		score += 1
	}

	return score
}

func (r *Robot) decidePrefop(
	hole [2]cards.Card,
	toCall, pot, bb, stack uint64,
) (player.Action, uint64) {
	str := preflopStrength(hole)

	switch {
	case str >= 20:
		// Premium: raise to 3x BB (or push if stack is short)
		raise := 3 * bb
		if raise >= stack || stack <= 6*bb {
			return player.ActionAllIn, 0
		}
		if toCall == 0 {
			return player.ActionBet, raise
		}
		return player.ActionRaise, raise + toCall

	case str >= 12:
		// Strong: call, or bet if no one has bet
		if toCall == 0 {
			return player.ActionBet, bb
		}
		// Call only if pot odds are reasonable (toCall <= 1/3 pot)
		if toCall <= pot/3+1 {
			return player.ActionCall, 0
		}
		return player.ActionFold, 0

	case str >= 7:
		// Medium: check or call small amounts
		if toCall == 0 {
			return player.ActionCheck, 0
		}
		if toCall <= bb {
			return player.ActionCall, 0
		}
		return player.ActionFold, 0

	default:
		// Weak: check if free, otherwise fold
		if toCall == 0 {
			return player.ActionCheck, 0
		}
		return player.ActionFold, 0
	}
}

// ── Post-flop decision ────────────────────────────────────────────────────────

func (r *Robot) decidePostflop(
	hole [2]cards.Card,
	community []cards.Card,
	toCall, pot, bb, stack uint64,
) (player.Action, uint64) {
	// Evaluate the robot's best 5-card hand from all available cards
	all := make([]cards.Card, 0, 7)
	all = append(all, hole[0], hole[1])
	all = append(all, community...)
	hand := cards.BestHand(all)

	// Classify made hand strength
	made := hand.Type

	// Draw potential (count outs) — simplified: we always evaluate made hand only
	switch {
	case made >= cards.HandTwoPair:
		// Strong made hand: value-bet or raise
		bet := pot / 2
		if bet == 0 {
			bet = bb
		}
		if bet >= stack {
			return player.ActionAllIn, 0
		}
		if toCall == 0 {
			return player.ActionBet, bet
		}
		return player.ActionCall, 0

	case made == cards.HandPair:
		// One pair: check or call a small bet
		if toCall == 0 {
			return player.ActionCheck, 0
		}
		// Call if pot odds > 25% (toCall / (pot + toCall))
		equity := toCall * 100 / (pot + toCall + 1)
		if equity <= 30 {
			return player.ActionCall, 0
		}
		return player.ActionFold, 0

	default:
		// High card / drawing hand: check or fold
		if toCall == 0 {
			// Occasionally bluff (1-in-5 chance)
			if r.rng.Intn(5) == 0 {
				bet := bb
				if bet >= stack {
					return player.ActionAllIn, 0
				}
				return player.ActionBet, bet
			}
			return player.ActionCheck, 0
		}
		return player.ActionFold, 0
	}
}
