package cowboy

import (
	"github.com/qbgames/qbdz-game/internal/cards"
	"github.com/qbgames/qbdz-game/internal/config"
)

// HandResult holds the evaluated outcome of one Cowboy hand.
type HandResult struct {
	WinStatus      WinStatus
	CowboyHand     cards.Hand // dealer's best 5-card hand
	CowHand        cards.Hand // player-side best 5-card hand
	WinnerCardType cards.HandType

	// Which standard bet positions were hit
	HitPositions [BetPositionEnd]bool

	// Computed payout multipliers per position (from config)
	PositionOdds [BetPositionEnd]uint32
}

// EvaluateResult determines the winner and which bet positions pay out.
//
// Parameters:
//   - cowboyCards: 2 hole cards for the Cowboy (dealer)
//   - cowCards:    2 hole cards for the Cow (player side)
//   - community:   5 community cards
//   - cfg:         odds configuration
//
// C++ reference: CSoCowboyGameLogic::ParsePositionOdds
func EvaluateResult(
	cowboyCards [2]cards.Card,
	cowCards [2]cards.Card,
	community [5]cards.Card,
	cfg *config.CowboyOddsConfig,
) HandResult {
	var r HandResult

	// Build 7-card hands for each side
	cowboyAll := append(community[:], cowboyCards[0], cowboyCards[1])
	cowAll := append(community[:], cowCards[0], cowCards[1])

	r.CowboyHand = cards.BestHand(cowboyAll)
	r.CowHand = cards.BestHand(cowAll)

	// Compare hands to determine main winner
	cmp := cards.CompareHands(r.CowboyHand, r.CowHand)
	switch {
	case cmp > 0:
		r.WinStatus = WinCowboy
		r.WinnerCardType = r.CowboyHand.Type
	case cmp < 0:
		r.WinStatus = WinCow
		r.WinnerCardType = r.CowHand.Type
	default:
		r.WinStatus = WinDraw
		r.WinnerCardType = r.CowboyHand.Type
	}

	// Determine which positions hit
	r.HitPositions[BetDraw] = r.WinStatus == WinDraw
	r.HitPositions[BetPersonWin] = r.WinStatus == WinCow
	r.HitPositions[BetCowWin] = r.WinStatus == WinCowboy

	// Side bets evaluate the Cow (player-side) 2 hole cards
	r.HitPositions[BetFlush] = isTwoCardFlush(cowCards)
	r.HitPositions[BetStraight] = isTwoCardStraight(cowCards)
	r.HitPositions[BetPair] = isTwoCardPair(cowCards)
	r.HitPositions[BetStraightFlush] = isTwoCardFlush(cowCards) && isTwoCardStraight(cowCards)
	r.HitPositions[BetAA] = isAA(cowCards)

	// Hand-type bets evaluate the winner's best 5-card hand
	winnerHand := r.CowboyHand
	if r.WinStatus == WinCow {
		winnerHand = r.CowHand
	}
	r.HitPositions[BetHighPair] = winnerHand.Type <= cards.HandPair
	r.HitPositions[BetTwoPair] = winnerHand.Type == cards.HandTwoPair
	r.HitPositions[BetThreeFlush] = winnerHand.Type >= cards.HandThreeOfAKind &&
		winnerHand.Type <= cards.HandFlush
	r.HitPositions[BetFullHouse] = winnerHand.Type == cards.HandFullHouse
	r.HitPositions[BetQuadPlus] = winnerHand.Type >= cards.HandFourOfAKind

	// Look up odds multipliers from config
	for i := 0; i < BetPositionEnd; i++ {
		pos := BetPosition(i)
		if cfg != nil {
			r.PositionOdds[i] = cfg.GetOddsByPosition(uint32(pos))
		}
	}

	return r
}

// PayoutForBet calculates the net payout for a single player bet.
// Returns a positive value for a win (including returned stake),
// or the negated bet amount for a loss.
//
// C++ reference: CSoCowboyGameBillManager settlement logic
func PayoutForBet(pos BetPosition, betAmount uint64, result *HandResult) int64 {
	if !result.HitPositions[pos] {
		return -int64(betAmount) // lost
	}
	odds := int64(result.PositionOdds[pos])
	if odds == 0 {
		odds = 1 // fallback: return stake
	}
	// Win = betAmount * odds (many Cowboy games return stake + winnings)
	return int64(betAmount) * odds
}

// ── Two-card hand helpers (for side bets on hole cards only) ─────────────────

func isTwoCardFlush(cards [2]cards.Card) bool {
	return cards[0].Shape == cards[1].Shape
}

func isTwoCardStraight(c [2]cards.Card) bool {
	diff := int(c[0].Level) - int(c[1].Level)
	if diff < 0 {
		diff = -diff
	}
	// A-K is also a straight (diff = 14-13 = 1)
	return diff == 1 || diff == 12 // 12 = K(13) - A(1) in non-high-A numbering
}

func isTwoCardPair(c [2]cards.Card) bool {
	return c[0].Number == c[1].Number
}

func isAA(c [2]cards.Card) bool {
	return c[0].Number == cards.NumberA && c[1].Number == cards.NumberA
}
