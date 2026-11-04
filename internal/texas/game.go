package texas

import (
	"math/rand"
	"time"

	"github.com/qbgames/qbdz-game/internal/cards"
	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/player"
)

// Game holds all per-hand state for one Texas Hold'em table.
// Methods return []Event; the Table actor processes them.
// Game is NOT goroutine-safe — it runs exclusively inside the table goroutine.
//
// C++ reference: CSoGameLogic in Source/SoGameLogic/SoGameLogic.h/.cpp
type Game struct {
	cfg    *config.SceneConfig
	status GameStatus
	rng    *rand.Rand

	// Seats: index 0..cfg.SeatCount-1, nil = empty
	players [MaxSeats]*player.Player

	// Per-hand state (reset in resetHand)
	deck       cards.Deck
	holeCards  [MaxSeats][2]cards.Card
	community  [5]cards.Card
	communityN int // how many community cards revealed

	dealerSeat  int // Dealer Button position
	sbSeat      int // Small Blind seat
	bbSeat      int // Big Blind seat
	currentSeat int // whose turn it is to act

	// Chips contributed this hand (across all streets, per seat)
	chipsIn [MaxSeats]uint64
	allIn   [MaxSeats]bool
	active  [MaxSeats]bool // still in hand (not folded)

	// Current street betting state
	streetChips   [MaxSeats]uint64 // chips put in this street
	maxBet        uint64           // highest bet on current street
	lastRaiser    int              // seat of last raiser (-1 if none)
	actionsLeft   int              // players who still need to act
	lastFullRaise uint64           // most recent full raise increment

	blindInfo BlindInfo

	handCount    uint32
	resultReason ResultReason
}

// BlindInfo holds current blind levels for the hand.
type BlindInfo struct {
	SmallBlind uint64
	BigBlind   uint64
	Ante       uint64
	Level      uint32
}

// NewGame creates a Game ready to start hands.
func NewGame(cfg *config.SceneConfig) *Game {
	return &Game{
		cfg:        cfg,
		status:     StatusNoStart,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		dealerSeat: -1,
		blindInfo: BlindInfo{
			SmallBlind: cfg.SmallBlind,
			BigBlind:   cfg.BigBlind,
			Ante:       uint64(cfg.Ante),
		},
	}
}

// SeatPlayer places a player in a seat. Returns empty if seat is taken or invalid.
func (g *Game) SeatPlayer(seat int, p *player.Player) []Event {
	if seat < 0 || seat >= MaxSeats || g.players[seat] != nil {
		return nil
	}
	g.players[seat] = p
	return nil
}

// UnseatPlayer removes a player from a seat (called on exit/stand).
func (g *Game) UnseatPlayer(seat int) {
	if seat >= 0 && seat < MaxSeats {
		g.players[seat] = nil
	}
}

// FindSeat returns the seat index for userID, or -1.
func (g *Game) FindSeat(userID uint32) int {
	for i, p := range g.players {
		if p != nil && p.UserID == userID {
			return i
		}
	}
	return -1
}

// ReadyPlayerCount returns the number of seated, ready players.
func (g *Game) ReadyPlayerCount() int {
	count := 0
	for _, p := range g.players {
		if p != nil && p.IsReady {
			count++
		}
	}
	return count
}

// CanStart returns true when enough players are ready to begin.
func (g *Game) CanStart() bool {
	return g.status == StatusNoStart && g.ReadyPlayerCount() >= 2
}

// Status returns the current game phase.
func (g *Game) Status() GameStatus { return g.status }

// CurrentSeat returns the seat index of the player who must act next.
// Returns -1 when no action is pending.
func (g *Game) CurrentSeat() int {
	if g.status < StatusPreFlop || g.status > StatusRiver {
		return -1
	}
	return g.currentSeat
}

// HoleCards returns the two hole cards dealt to seat.
func (g *Game) HoleCards(seat int) [2]cards.Card {
	if seat < 0 || seat >= MaxSeats {
		return [2]cards.Card{}
	}
	return g.holeCards[seat]
}

// Community returns the revealed community cards (length 0–5).
func (g *Game) Community() []cards.Card {
	return g.community[:g.communityN]
}

// ToCall returns how many chips the current player needs to call.
func (g *Game) ToCall() uint64 {
	if g.currentSeat < 0 || g.currentSeat >= MaxSeats {
		return 0
	}
	owed := g.maxBet - g.streetChips[g.currentSeat]
	return owed
}

// PotSize returns the total chips in the pot (all streets, all players).
func (g *Game) PotSize() uint64 {
	var total uint64
	for _, v := range g.chipsIn {
		total += v
	}
	return total
}

// BigBlind returns the current big blind amount.
func (g *Game) BigBlind() uint64 { return g.blindInfo.BigBlind }

// SetBlindLevel updates the blind schedule for future hands.
// Called by the table when the host sends a SetBlindLevel event (SNG escalation).
// Takes effect at the start of the next hand.
func (g *Game) SetBlindLevel(sb, bb, ante uint64, level uint32) {
	g.blindInfo.SmallBlind = sb
	g.blindInfo.BigBlind = bb
	g.blindInfo.Ante = ante
	g.blindInfo.Level = level
}

// RankedSeats returns seat indices ordered by chip count, highest first.
// Used by GetRankedPlayers to rank SNG survivors.
func (g *Game) RankedSeats() []int {
	type seatChips struct {
		seat  int
		chips uint64
	}
	var sc []seatChips
	for i, p := range g.players {
		if p != nil {
			sc = append(sc, seatChips{i, p.ChipDesk})
		}
	}
	// Simple insertion sort (MaxSeats ≤ 9, tiny)
	for i := 1; i < len(sc); i++ {
		for j := i; j > 0 && sc[j].chips > sc[j-1].chips; j-- {
			sc[j], sc[j-1] = sc[j-1], sc[j]
		}
	}
	seats := make([]int, len(sc))
	for i, v := range sc {
		seats[i] = v.seat
	}
	return seats
}

// ── Hand lifecycle ────────────────────────────────────────────────────────────

// StartHand begins a new hand: shuffle, assign dealer/blinds, deal hole cards,
// post blinds. Returns events for the table to send.
func (g *Game) StartHand() []Event {
	g.handCount++
	g.resetHand()

	var events []Event

	// Move dealer button. In heads-up play the dealer posts the small blind
	// and acts first preflop.
	g.dealerSeat = g.nextActiveSeat(g.dealerSeat)
	if g.ReadyPlayerCount() == 2 {
		g.sbSeat = g.dealerSeat
		g.bbSeat = g.nextActiveSeat(g.dealerSeat)
	} else {
		g.sbSeat = g.nextActiveSeat(g.dealerSeat)
		g.bbSeat = g.nextActiveSeat(g.sbSeat)
	}

	// Mark all seated+ready players as in-game
	for i, p := range g.players {
		if p != nil && p.IsReady {
			p.InGame = true
			p.Action = player.ActionNull
			g.active[i] = true
		}
	}

	// Shuffle deck
	g.deck = cards.NewDeck()
	g.deck.Shuffle(g.rng)

	// Post antes
	if g.blindInfo.Ante > 0 {
		events = append(events, g.postAnte()...)
	}

	// Post blinds
	events = append(events, g.postBlinds()...)
	g.lastFullRaise = g.blindInfo.BigBlind

	// Deal 2 hole cards to each active player
	events = append(events, g.dealHoleCards()...)

	// Start PreFlop action (player after BB acts first)
	g.status = StatusPreFlop
	g.currentSeat = g.nextActiveSeat(g.bbSeat)
	g.lastRaiser = g.bbSeat
	g.actionsLeft = g.countActive()

	// Notify game start
	events = append(events, Event{Kind: EventGameStart})

	// Notify whose turn it is
	events = append(events, g.notifyTurn()...)

	return events
}

// PlayerAction processes a player's betting action.
// seat is the seat index; action and amount come from the client message.
func (g *Game) PlayerAction(seat int, action player.Action, amount uint64) []Event {
	if g.status < StatusPreFlop || g.status > StatusRiver {
		return nil
	}
	if seat != g.currentSeat {
		return nil
	}
	p := g.players[seat]
	if p == nil || !p.InGame || p.Action == player.ActionFold {
		return nil
	}

	var events []Event
	events = append(events, Event{Kind: EventCancelTimer, TimerID: TimerAction})

	switch action {
	case player.ActionFold:
		events = append(events, g.doFold(seat)...)
	case player.ActionCheck:
		events = append(events, g.doCheck(seat)...)
	case player.ActionCall:
		events = append(events, g.doCall(seat)...)
	case player.ActionBet:
		events = append(events, g.doBet(seat, amount)...)
	case player.ActionRaise:
		events = append(events, g.doRaise(seat, amount)...)
	case player.ActionAllIn:
		events = append(events, g.doAllIn(seat)...)
	}

	// Check if street or hand is over
	events = append(events, g.advanceIfReady()...)
	return events
}

// ActionTimeout auto-acts for the current seat (fold or check).
func (g *Game) ActionTimeout() []Event {
	p := g.players[g.currentSeat]
	if p == nil {
		return nil
	}
	p.ActionTimeoutCount++
	p.ActionTimedOut = true

	// If player can check (no bet to call), check; otherwise fold.
	if g.maxBet == 0 || g.streetChips[g.currentSeat] == g.maxBet {
		return g.PlayerAction(g.currentSeat, player.ActionCheck, 0)
	}
	return g.PlayerAction(g.currentSeat, player.ActionFold, 0)
}

// ── Blind posting ─────────────────────────────────────────────────────────────

func (g *Game) postBlinds() []Event {
	var events []Event

	sb := g.blindInfo.SmallBlind
	bb := g.blindInfo.BigBlind

	if sbP := g.players[g.sbSeat]; sbP != nil {
		paid := g.forceBet(g.sbSeat, sb)
		_ = paid
	}
	if bbP := g.players[g.bbSeat]; bbP != nil {
		paid := g.forceBet(g.bbSeat, bb)
		_ = paid
		if g.streetChips[g.bbSeat] > g.maxBet {
			g.maxBet = g.streetChips[g.bbSeat]
		}
	}

	events = append(events, Event{
		Kind:    EventSendAll,
		Cmd:     0, // CmdNotifyBlindInfo — set when proto is wired
		Payload: nil,
	})
	return events
}

func (g *Game) postAnte() []Event {
	for i, p := range g.players {
		if p != nil && p.InGame {
			g.forceBet(i, g.blindInfo.Ante)
		}
	}
	return nil
}

// forceBet deducts chips from a seat's stack (for blinds/antes).
// Returns the actual amount paid (may be less if stack is small).
func (g *Game) forceBet(seat int, amount uint64) uint64 {
	p := g.players[seat]
	if p == nil {
		return 0
	}
	paid := amount
	if paid > p.ChipDesk {
		paid = p.ChipDesk
		g.allIn[seat] = true
		p.Action = player.ActionAllIn
	}
	p.ChipDesk -= paid
	g.chipsIn[seat] += paid
	g.streetChips[seat] += paid
	return paid
}

// ── Card dealing ──────────────────────────────────────────────────────────────

func (g *Game) dealHoleCards() []Event {
	// Deal 2 cards to each active player (one round at a time, like a real deal)
	for round := 0; round < 2; round++ {
		for i := 0; i < MaxSeats; i++ {
			if g.players[i] != nil && g.active[i] {
				c, ok := g.deck.Deal()
				if !ok {
					break
				}
				g.holeCards[i][round] = c
			}
		}
	}
	// Each player gets a private SendToPlayer with their hole cards
	// (payload would be Protobuf; for now just mark the event)
	var events []Event
	for i, p := range g.players {
		if p != nil && g.active[i] {
			events = append(events, Event{
				Kind:         EventSendTo,
				TargetUserID: p.UserID,
				Cmd:          CmdNotifyDealCards,
				Payload:      nil, // filled when Protobuf is wired
			})
		}
	}
	return events
}

// CmdNotifyDealCards is the command ID for hole card notification.
// Will be replaced by the actual proto cmd value once DZGame.proto is integrated.
const CmdNotifyDealCards = 2002

func (g *Game) dealCommunity(n int) []Event {
	for i := 0; i < n; i++ {
		c, ok := g.deck.Deal()
		if !ok {
			break
		}
		g.community[g.communityN] = c
		g.communityN++
	}
	return []Event{{Kind: EventSendAll, Cmd: CmdNotifyDealCards}}
}

// ── Action handlers ───────────────────────────────────────────────────────────

func (g *Game) doFold(seat int) []Event {
	p := g.players[seat]
	p.Action = player.ActionFold
	p.InGame = false
	g.active[seat] = false
	g.actionsLeft--
	return []Event{{Kind: EventSendAll, Cmd: CmdNotifyAction}}
}

func (g *Game) doCheck(seat int) []Event {
	p := g.players[seat]
	p.Action = player.ActionCheck
	g.actionsLeft--
	return []Event{{Kind: EventSendAll, Cmd: CmdNotifyAction}}
}

func (g *Game) doCall(seat int) []Event {
	p := g.players[seat]
	owed := g.maxBet - g.streetChips[seat]
	if owed >= p.ChipDesk {
		return g.doAllIn(seat)
	}
	g.streetChips[seat] += owed
	g.chipsIn[seat] += owed
	p.ChipDesk -= owed
	p.Action = player.ActionCall
	g.actionsLeft--
	return []Event{{Kind: EventSendAll, Cmd: CmdNotifyAction}}
}

func (g *Game) doBet(seat int, amount uint64) []Event {
	p := g.players[seat]
	if amount >= p.ChipDesk {
		return g.doAllIn(seat)
	}
	actual := amount
	g.streetChips[seat] += actual
	g.chipsIn[seat] += actual
	p.ChipDesk -= actual
	p.Action = player.ActionBet
	if g.streetChips[seat] > g.maxBet {
		oldMaxBet := g.maxBet
		g.maxBet = g.streetChips[seat]
		g.lastRaiser = seat
		raiseIncrement := g.maxBet - oldMaxBet
		if raiseIncrement >= g.blindInfo.BigBlind {
			g.lastFullRaise = raiseIncrement
			g.actionsLeft = g.countActive() - 1
		} else {
			g.actionsLeft--
		}
	} else {
		g.actionsLeft--
	}
	return []Event{{Kind: EventSendAll, Cmd: CmdNotifyAction}}
}

func (g *Game) doRaise(seat int, totalAmount uint64) []Event {
	p := g.players[seat]
	owed := totalAmount - g.streetChips[seat]
	if owed >= p.ChipDesk {
		return g.doAllIn(seat)
	}
	g.streetChips[seat] += owed
	g.chipsIn[seat] += owed
	p.ChipDesk -= owed
	p.Action = player.ActionRaise
	if g.streetChips[seat] > g.maxBet {
		oldMaxBet := g.maxBet
		g.maxBet = g.streetChips[seat]
		g.lastRaiser = seat
		raiseIncrement := g.maxBet - oldMaxBet
		if raiseIncrement >= g.lastFullRaise {
			g.lastFullRaise = raiseIncrement
			g.actionsLeft = g.countActive() - 1
		} else {
			g.actionsLeft--
		}
	} else {
		g.actionsLeft--
	}
	return []Event{{Kind: EventSendAll, Cmd: CmdNotifyAction}}
}

func (g *Game) doAllIn(seat int) []Event {
	p := g.players[seat]
	amount := p.ChipDesk
	g.streetChips[seat] += amount
	g.chipsIn[seat] += amount
	p.ChipDesk = 0
	p.Action = player.ActionAllIn
	g.allIn[seat] = true
	if g.streetChips[seat] > g.maxBet {
		oldMaxBet := g.maxBet
		g.maxBet = g.streetChips[seat]
		g.lastRaiser = seat
		raiseIncrement := g.maxBet - oldMaxBet
		if raiseIncrement >= g.lastFullRaise {
			g.lastFullRaise = raiseIncrement
			g.actionsLeft = g.countActive() - 1
		} else {
			g.actionsLeft--
		}
	} else {
		g.actionsLeft--
	}
	return []Event{{Kind: EventSendAll, Cmd: CmdNotifyAction}}
}

// ── Street advancement ────────────────────────────────────────────────────────

// advanceIfReady checks if the current street is over and advances to the next.
func (g *Game) advanceIfReady() []Event {
	active := g.countActive()

	// Only one player left — they win by default
	if active <= 1 {
		return g.settle()
	}

	// All remaining players are all-in — run out the board
	nonAllIn := 0
	for i, p := range g.players {
		if p != nil && g.active[i] && !g.allIn[i] {
			nonAllIn++
		}
	}
	if nonAllIn <= 1 && g.actionsLeft <= 0 {
		return g.runOutBoard()
	}

	// Still waiting for players to act
	if g.actionsLeft > 0 {
		g.currentSeat = g.nextActiveSeatFrom(g.currentSeat)
		return g.notifyTurn()
	}

	// Street complete — advance
	switch g.status {
	case StatusPreFlop:
		return g.startFlop()
	case StatusFlop:
		return g.startTurn()
	case StatusTurn:
		return g.startRiver()
	case StatusRiver:
		return g.settle()
	}
	return nil
}

func (g *Game) startFlop() []Event {
	g.status = StatusFlop
	g.resetStreet()
	events := g.dealCommunity(3)
	events = append(events, g.startStreetAction()...)
	return events
}

func (g *Game) startTurn() []Event {
	g.status = StatusTurn
	g.resetStreet()
	events := g.dealCommunity(1)
	events = append(events, g.startStreetAction()...)
	return events
}

func (g *Game) startRiver() []Event {
	g.status = StatusRiver
	g.resetStreet()
	events := g.dealCommunity(1)
	events = append(events, g.startStreetAction()...)
	return events
}

func (g *Game) startStreetAction() []Event {
	g.currentSeat = g.nextActiveSeatFrom(g.dealerSeat)
	g.actionsLeft = g.countActive()
	return g.notifyTurn()
}

// runOutBoard deals remaining community cards when all players are all-in.
func (g *Game) runOutBoard() []Event {
	var events []Event
	for g.communityN < 5 {
		if g.communityN == 3 {
			g.status = StatusTurn
		} else if g.communityN == 4 {
			g.status = StatusRiver
		}
		events = append(events, g.dealCommunity(1)...)
	}
	events = append(events, g.settle()...)
	return events
}

// ── Settlement ────────────────────────────────────────────────────────────────

// settle computes winners, distributes pots, and emits bill events.
func (g *Game) settle() []Event {
	g.status = StatusResult
	g.resultReason = ReasonNormal

	active := g.countActive()
	if active == 1 {
		g.resultReason = ReasonFold
	} else if g.communityN < 5 {
		g.resultReason = ReasonAllIn
	}

	var events []Event
	pools := ComputePots(g.chipsIn, g.allIn, g.active)

	// Evaluate each active player's best hand
	bestHands := make([]cards.Hand, MaxSeats)
	for i, p := range g.players {
		if p != nil && g.active[i] {
			allCards := make([]cards.Card, 0, 7)
			allCards = append(allCards, g.holeCards[i][0], g.holeCards[i][1])
			for j := 0; j < g.communityN; j++ {
				allCards = append(allCards, g.community[j])
			}
			bestHands[i] = cards.BestHand(allCards)
		}
	}

	// Distribute each pool to its winner(s)
	for poolIndex, pool := range pools {
		// Find best hand among eligible seats
		winSeats := g.findWinners(pool, bestHands)
		share := pool.Amount / uint64(len(winSeats))
		remainder := pool.Amount % uint64(len(winSeats))

		for idx, seat := range winSeats {
			won := share
			if idx == 0 {
				won += remainder // odd chips go to first winner (closest to dealer)
			}
			p := g.players[seat]

			// Deduct service fee (rake) from winnings
			fee := won * uint64(g.cfg.ServiceCharge) / 100
			actualWon := won - fee

			p.ChipDesk += actualWon
			p.WinLoseChipTotal += int64(actualWon)

			events = append(events, Event{
				Kind:       EventSettleBill,
				BillUserID: p.UserID,
				WinChips:   int64(actualWon),
				BetChips:   g.chipsIn[seat],
				ServiceFee: fee,
				HandCount:  g.handCount,
				BillReason: g.resultReason,
				PoolIndex:  poolIndex,
			})
		}
	}

	// Net loss for each player = chips put in minus chips won back
	// (WinLoseChipTotal already credited wins above; subtract buy-in here)
	for i, p := range g.players {
		if p != nil && g.chipsIn[i] > 0 {
			p.WinLoseChipTotal -= int64(g.chipsIn[i])
		}
	}

	events = append(events, Event{Kind: EventSendAll, Cmd: CmdNotifyGameResult})
	events = append(events, Event{Kind: EventGameEnd})

	// Reset for next hand
	g.status = StatusNoStart
	for _, p := range g.players {
		if p != nil {
			p.InGame = false
			p.Action = player.ActionNull
			p.ActionTimedOut = false
		}
	}

	return events
}

// CmdNotifyGameResult is the command ID for game result notification.
const CmdNotifyGameResult = 2006

// CmdNotifyAction is the command ID for player action notification.
const CmdNotifyAction = 2003

func (g *Game) findWinners(pool Pool, hands []cards.Hand) []int {
	var best cards.Hand
	initialized := false
	var winners []int

	for i := 0; i < MaxSeats; i++ {
		if !pool.Eligible[i] {
			continue
		}
		if !initialized {
			best = hands[i]
			winners = []int{i}
			initialized = true
			continue
		}
		cmp := cards.CompareHands(hands[i], best)
		if cmp > 0 {
			best = hands[i]
			winners = []int{i}
		} else if cmp == 0 {
			winners = append(winners, i)
		}
	}
	return winners
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (g *Game) resetHand() {
	g.communityN = 0
	g.maxBet = 0
	g.lastRaiser = -1
	g.actionsLeft = 0
	g.lastFullRaise = 0
	g.resultReason = ReasonNull
	for i := range g.chipsIn {
		g.chipsIn[i] = 0
		g.streetChips[i] = 0
		g.allIn[i] = false
		g.active[i] = false
		g.holeCards[i] = [2]cards.Card{}
	}
	for i := range g.community {
		g.community[i] = cards.Zero
	}
}

func (g *Game) resetStreet() {
	g.maxBet = 0
	g.lastRaiser = -1
	g.lastFullRaise = 0
	for i := range g.streetChips {
		g.streetChips[i] = 0
	}
	for _, p := range g.players {
		if p != nil && p.InGame && p.Action != player.ActionAllIn {
			p.Action = player.ActionNull
		}
	}
}

func (g *Game) countActive() int {
	n := 0
	for i, p := range g.players {
		if p != nil && g.active[i] && p.Action != player.ActionFold {
			n++
		}
	}
	return n
}

// nextActiveSeat returns the next seat index (wrapping) that has an in-game player,
// starting after `from`.
func (g *Game) nextActiveSeat(from int) int {
	for i := 1; i <= MaxSeats; i++ {
		seat := (from + i) % MaxSeats
		if p := g.players[seat]; p != nil && (p.IsReady || p.InGame) {
			return seat
		}
	}
	return from
}

// nextActiveSeatFrom returns the next seat that is still active (not folded, not all-in).
func (g *Game) nextActiveSeatFrom(from int) int {
	for i := 1; i <= MaxSeats; i++ {
		seat := (from + i) % MaxSeats
		if p := g.players[seat]; p != nil && g.active[seat] &&
			p.Action != player.ActionFold && !g.allIn[seat] {
			return seat
		}
	}
	return from
}

func (g *Game) notifyTurn() []Event {
	if g.currentSeat < 0 || g.currentSeat >= MaxSeats {
		return nil
	}
	return []Event{
		{Kind: EventSendAll, Cmd: CmdNotifyAction},
		{
			Kind:          EventSetTimer,
			TimerID:       TimerAction,
			TimerDuration: int64(g.cfg.ActionTimeSec) * 1000,
		},
	}
}
