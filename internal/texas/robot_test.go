package texas

import (
	"math/rand"
	"testing"

	"github.com/qbgames/qbdz-game/internal/cards"
	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/player"
)

// testCfg returns a minimal scene config for robot tests.
func robotTestCfg() *config.SceneConfig {
	cfg := config.DefaultSceneConfig()
	cfg.SmallBlind = 50
	cfg.BigBlind = 100
	return &cfg
}

func TestPreflopStrength(t *testing.T) {
	cases := []struct {
		name    string
		c0, c1  cards.Card
		minStr  int
	}{
		{
			"AA",
			cards.Card{Number: cards.NumberA, Level: 14, Shape: cards.ShapeSpade},
			cards.Card{Number: cards.NumberA, Level: 14, Shape: cards.ShapeHearts},
			20,
		},
		{
			"72o",
			cards.Card{Number: cards.Number7, Level: 7, Shape: cards.ShapeSpade},
			cards.Card{Number: cards.Number2, Level: 2, Shape: cards.ShapeHearts},
			0,
		},
		{
			"AKs",
			cards.Card{Number: cards.NumberA, Level: 14, Shape: cards.ShapeSpade},
			cards.Card{Number: cards.NumberK, Level: 13, Shape: cards.ShapeSpade},
			15, // suited + connected + high card
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			str := preflopStrength([2]cards.Card{tc.c0, tc.c1})
			if str < tc.minStr {
				t.Errorf("%s: got strength %d, want >= %d", tc.name, str, tc.minStr)
			}
		})
	}
}

func TestRobotDecide_PremiumHandRaises(t *testing.T) {
	g := NewGame(robotTestCfg())
	rng := rand.New(rand.NewSource(42))
	robot := NewRobot(rng)

	// Manually set up a game state: seat0 has AA, no community cards, no bet to call
	g.status = StatusPreFlop
	g.currentSeat = 0
	g.blindInfo = BlindInfo{SmallBlind: 50, BigBlind: 100}
	g.players[0] = player.NewPlayer(1001, "Bot", "", 10000)
	g.players[0].InGame = true
	g.active[0] = true
	g.holeCards[0] = [2]cards.Card{
		{Number: cards.NumberA, Level: 14, Shape: cards.ShapeSpade},
		{Number: cards.NumberA, Level: 14, Shape: cards.ShapeHearts},
	}
	g.maxBet = 0 // nobody has bet

	action, amount := robot.Decide(g, 0)

	// With AA and no bet to call, robot should bet or raise
	if action != player.ActionBet && action != player.ActionRaise && action != player.ActionAllIn {
		t.Errorf("expected aggressive action with AA, got %v (amount %d)", action, amount)
	}
}

func TestRobotDecide_WeakHandFolds(t *testing.T) {
	g := NewGame(robotTestCfg())
	rng := rand.New(rand.NewSource(42))
	robot := NewRobot(rng)

	g.status = StatusPreFlop
	g.currentSeat = 0
	g.blindInfo = BlindInfo{SmallBlind: 50, BigBlind: 100}
	g.players[0] = player.NewPlayer(1001, "Bot", "", 10000)
	g.players[0].InGame = true
	g.active[0] = true
	g.holeCards[0] = [2]cards.Card{
		{Number: cards.Number7, Level: 7, Shape: cards.ShapeSpade},
		{Number: cards.Number2, Level: 2, Shape: cards.ShapeHearts},
	}
	// There's a raise to call (200 chips)
	g.maxBet = 200
	g.streetChips[0] = 0

	action, _ := robot.Decide(g, 0)

	if action != player.ActionFold {
		t.Errorf("expected fold with 72o facing a raise, got %v", action)
	}
}

func TestRobotDecide_StrongMadeHandBets(t *testing.T) {
	g := NewGame(robotTestCfg())
	rng := rand.New(rand.NewSource(42))
	robot := NewRobot(rng)

	g.status = StatusFlop
	g.currentSeat = 0
	g.blindInfo = BlindInfo{SmallBlind: 50, BigBlind: 100}
	g.players[0] = player.NewPlayer(1001, "Bot", "", 10000)
	g.players[0].InGame = true
	g.active[0] = true

	// Hole cards: A♠ A♥ — two aces
	g.holeCards[0] = [2]cards.Card{
		{Number: cards.NumberA, Level: 14, Shape: cards.ShapeSpade},
		{Number: cards.NumberA, Level: 14, Shape: cards.ShapeHearts},
	}
	// Community: A♦ K♣ 2♠ — gives robot three aces
	g.community[0] = cards.Card{Number: cards.NumberA, Level: 14, Shape: cards.ShapeDiamonds}
	g.community[1] = cards.Card{Number: cards.NumberK, Level: 13, Shape: cards.ShapeClubs}
	g.community[2] = cards.Card{Number: cards.Number2, Level: 2, Shape: cards.ShapeSpade}
	g.communityN = 3
	g.maxBet = 0 // check situation
	g.chipsIn[0] = 200 // pot is 200

	action, _ := robot.Decide(g, 0)

	// Three of a kind should lead to a bet or raise
	if action != player.ActionBet && action != player.ActionRaise && action != player.ActionAllIn {
		t.Errorf("expected aggressive action with trip aces, got %v", action)
	}
}

func TestRobotDecide_FreePreflopCheck(t *testing.T) {
	g := NewGame(robotTestCfg())
	rng := rand.New(rand.NewSource(42))
	robot := NewRobot(rng)

	g.status = StatusPreFlop
	g.currentSeat = 0
	g.blindInfo = BlindInfo{SmallBlind: 50, BigBlind: 100}
	g.players[0] = player.NewPlayer(1001, "Bot", "", 10000)
	g.players[0].InGame = true
	g.active[0] = true
	// Weak hand, no bet to call
	g.holeCards[0] = [2]cards.Card{
		{Number: cards.Number3, Level: 3, Shape: cards.ShapeSpade},
		{Number: cards.Number8, Level: 8, Shape: cards.ShapeHearts},
	}
	g.maxBet = 0

	action, _ := robot.Decide(g, 0)

	// With no bet to call even a weak hand should check/bet (not fold)
	if action == player.ActionFold {
		t.Error("robot should not fold when checking is free")
	}
}
