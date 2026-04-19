package cowboy

import (
	"math/rand"
	"time"

	"github.com/qbgames/qbdz-game/internal/cards"
	"github.com/qbgames/qbdz-game/internal/config"
)

// MaxSeats is the number of physical seats at a Cowboy table.
// Matches C++ MAX_SEAT_COUNT_COWBOY = 8.
const MaxSeats = 8

// Player holds the per-player state for one Cowboy game.
type Player struct {
	UserID   uint32
	NickName string
	Avatar   string
	Money    uint64 // current coin balance
	IsReady  bool

	// Per-hand bet state (reset each hand)
	PositionBets [BetPositionEnd]uint64
	BetTotal     uint64
}

// NewPlayer creates a ready Cowboy player.
func NewPlayer(userID uint32, nick string, money uint64) *Player {
	return &Player{UserID: userID, NickName: nick, Money: money, IsReady: true}
}

// TotalBet sums all position bets.
func (p *Player) TotalBet() uint64 {
	var total uint64
	for _, v := range p.PositionBets {
		total += v
	}
	return total
}

// Game holds all state for one Cowboy table.
// NOT goroutine-safe — runs exclusively inside the table goroutine.
//
// C++ reference: CSoCowboyGameLogic in SoCowboyGameLogic.h/.cpp
type Game struct {
	cfg     *config.CowboySceneConfig
	oddscfg *config.CowboyOddsConfig
	status  GameStatus
	rng     *rand.Rand

	// All players registered at this table (userID → player)
	players map[uint32]*Player

	// Per-hand card state
	deck        cards.Deck
	community   [5]cards.Card
	cowboyCards [2]cards.Card // dealer hand
	cowCards    [2]cards.Card // player-side hand

	// Betting accumulation
	hadBet        bool
	totalBetCoins uint64

	// Rolling 30-hand statistics
	stats     [30]ResultRecord
	statsNext int

	gameCount uint32
}

// NewGame creates a Cowboy Game ready to run.
func NewGame(cfg *config.CowboySceneConfig, oddscfg *config.CowboyOddsConfig) *Game {
	return &Game{
		cfg:     cfg,
		oddscfg: oddscfg,
		status:  StatusNoStart,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		players: make(map[uint32]*Player),
	}
}

// AddPlayer registers a player.
func (g *Game) AddPlayer(p *Player) { g.players[p.UserID] = p }

// RemovePlayer removes a player from the game.
func (g *Game) RemovePlayer(userID uint32) { delete(g.players, userID) }

// PlayerCount returns the number of registered players.
func (g *Game) PlayerCount() int { return len(g.players) }

// Status returns the current game phase.
func (g *Game) Status() GameStatus { return g.status }

// CanStart returns true when a new hand can begin.
// C++ MIN_PLAYERS_START_COWBOY = 1.
func (g *Game) CanStart() bool {
	return (g.status == StatusNoStart || g.status == StatusCountdown || g.status == StatusResult) &&
		len(g.players) >= 1
}

// ── Hand lifecycle ────────────────────────────────────────────────────────────

// StartCountdown arms the pre-game countdown.
// C++ reference: CheckWaitForStart → AddTimer(eumTimerCowboyWaitForStart)
func (g *Game) StartCountdown() []Event {
	if g.status != StatusNoStart {
		return nil
	}
	g.status = StatusCountdown
	waitMs := int64(g.cfg.WaitTimeSec) * 1000
	if waitMs <= 0 {
		waitMs = 3000
	}
	return []Event{
		{Kind: EventSendAll, Cmd: CmdNotifyGameStatus},
		{Kind: EventSetTimer, TimerID: TimerWaitForStart, TimerDuration: waitMs},
	}
}

// StartHand deals cards and transitions through Start → SendCard → BetBegan.
// Called when TimerWaitForStart fires.
// C++ reference: CheckStartNextGame → DealCards → AddTimer(eumTimerCowboyStart)
func (g *Game) StartHand() []Event {
	if !g.CanStart() {
		return nil
	}
	g.gameCount++
	g.resetHand()
	g.status = StatusStart
	g.dealCards()

	startMs := int64(1000) // START_TIME_COWBOY = 1s
	return []Event{
		{Kind: EventGameStart},
		{Kind: EventSendAll, Cmd: CmdNotifyGameStatus},
		{Kind: EventSendAll, Cmd: CmdNotifyPlayerCards},
		{Kind: EventSetTimer, TimerID: TimerStart, TimerDuration: startMs},
	}
}

// OnStartTimer fires after start animation; transitions to BetBegan.
// C++ reference: onStartTimeout → AddTimer(eumTimerCowboySendCard)
func (g *Game) OnStartTimer() []Event {
	g.status = StatusSendCard
	sendMs := int64(1000) // SEND_CARD_TIME_COWBOY = 1s
	return []Event{
		{Kind: EventSendAll, Cmd: CmdNotifyGameStatus},
		{Kind: EventSetTimer, TimerID: TimerSendCard, TimerDuration: sendMs},
	}
}

// OnSendCardTimer fires after card animation; opens betting.
// C++ reference: onSendCardTimeout
func (g *Game) OnSendCardTimer() []Event {
	g.status = StatusBetBegan
	betMs := int64(g.cfg.BetTimeSec) * 1000
	if betMs <= 0 {
		betMs = 15000
	}
	return []Event{
		{Kind: EventSendAll, Cmd: CmdNotifyGameStatus},
		{Kind: EventSetTimer, TimerID: TimerBet, TimerDuration: betMs},
	}
}

// PlaceBet records a player's bet on a position.
// Returns events to notify the bettor (and broadcast to seated players).
// C++ reference: onRequestCowboyBet
func (g *Game) PlaceBet(userID uint32, pos BetPosition, amount uint64) []Event {
	if g.status != StatusBetBegan {
		return nil
	}
	p, ok := g.players[userID]
	if !ok {
		return nil
	}
	if !g.isValidPosition(pos) || p.Money < amount ||
		g.totalBetCoins+amount > g.cfg.TableBetLimit ||
		p.TotalBet()+amount > g.cfg.PlayerBetLimit {
		return []Event{{Kind: EventSendTo, TargetUserID: userID, Cmd: CmdNotifyBets}}
	}

	p.PositionBets[pos] += amount
	p.BetTotal += amount
	p.Money -= amount
	g.hadBet = true
	g.totalBetCoins += amount

	return []Event{
		{Kind: EventCowboyBet, CowboyBetUserID: userID, CowboyBetAmount: amount},
		{Kind: EventSendTo, TargetUserID: userID, Cmd: CmdNotifyBets},
		{Kind: EventSendExcept, ExcludeUserID: userID, Cmd: CmdNotifyBets},
	}
}

// OnBetTimer fires when the betting window closes; settles all bets.
// C++ reference: onBetTimeout → AfterGameBetEnd → CheckNotifyGameResult
func (g *Game) OnBetTimer() []Event {
	g.status = StatusBetEnd
	events := []Event{{Kind: EventSendAll, Cmd: CmdNotifyGameStatus}}

	result := EvaluateResult(g.cowboyCards, g.cowCards, g.community, g.oddscfg)
	g.recordStat(result)

	events = append(events, Event{Kind: EventSendAll, Cmd: CmdNotifyGameResult})
	for _, p := range g.players {
		events = append(events, g.settlePlayer(p, &result)...)
	}

	g.status = StatusResult
	nextMs := int64(g.cfg.ResultToStartSec) * 1000
	if nextMs <= 0 {
		nextMs = 10000
	}
	events = append(events,
		Event{Kind: EventGameEnd},
		Event{Kind: EventSetTimer, TimerID: TimerResultToStart, TimerDuration: nextMs},
	)
	return events
}

// OnResultToStartTimer starts the next hand, or returns to waiting if no players.
// C++ reference: onResultToStartTimeout → CheckStartNextGame
func (g *Game) OnResultToStartTimer() []Event {
	if len(g.players) == 0 {
		g.status = StatusNoStart
		return nil
	}
	return g.StartHand()
}

// ── Settlement ────────────────────────────────────────────────────────────────

func (g *Game) settlePlayer(p *Player, result *HandResult) []Event {
	var events []Event
	for i, betAmount := range p.PositionBets {
		if betAmount == 0 {
			continue
		}
		pos := BetPosition(i)
		payout := PayoutForBet(pos, betAmount, result)
		// Return stake; add winnings if positive
		p.Money += betAmount
		if payout > 0 {
			p.Money += uint64(payout)
		}
		events = append(events, Event{
			Kind:      EventSettleBet,
			BetUserID: p.UserID,
			BetPos:    pos,
			BetAmount: betAmount,
			PayAmount: payout,
		})
	}
	return events
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (g *Game) resetHand() {
	g.hadBet = false
	g.totalBetCoins = 0
	for _, p := range g.players {
		for i := range p.PositionBets {
			p.PositionBets[i] = 0
		}
		p.BetTotal = 0
	}
}

func (g *Game) dealCards() {
	g.deck = cards.NewDeck()
	g.deck.Shuffle(g.rng)
	g.cowboyCards[0], _ = g.deck.Deal()
	g.cowboyCards[1], _ = g.deck.Deal()
	g.cowCards[0], _ = g.deck.Deal()
	g.cowCards[1], _ = g.deck.Deal()
	for i := 0; i < 5; i++ {
		g.community[i], _ = g.deck.Deal()
	}
}

func (g *Game) isValidPosition(pos BetPosition) bool {
	if g.oddscfg == nil {
		return pos == BetDraw || pos == BetPersonWin || pos == BetCowWin
	}
	for _, o := range g.oddscfg.OddInfos {
		if BetPosition(o.Position) == pos {
			return true
		}
	}
	return false
}

func (g *Game) recordStat(result HandResult) {
	g.stats[g.statsNext] = ResultRecord{
		IsValid:        true,
		WinStatus:      result.WinStatus,
		WinnerCardType: int(result.WinnerCardType),
	}
	g.statsNext = (g.statsNext + 1) % len(g.stats)
}
