package table

import (
	"context"
	"log/slog"
	"math/rand"
	"time"

	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/metrics"
	"github.com/qbgames/qbdz-game/internal/player"
	"github.com/qbgames/qbdz-game/internal/texas"
)

// MaxSeats re-exports the texas constant for callers that only import table.
const MaxSeats = texas.MaxSeats

const inboxSize = 256

// Table is the central actor for one poker table.
// It owns exactly one goroutine (Run), all state is single-threaded.
// External callers send Msg values through Send(); the goroutine processes them.
//
// C++ reference: GameSvrImplTexas + all its Manager members.
type Table struct {
	id            uint32
	cfg           *config.SceneConfig
	blindSchedule []config.BlindInfoConfig // SNG blind escalation table
	host          TableHost
	inbox         chan Msg
	timers        timerSet
	game          *texas.Game
	robot         *texas.Robot

	// Seat → player mapping (mirrors game.players for quick lookup)
	seated [texas.MaxSeats]*player.Player
}

// NewTable creates a Table. Call Run(ctx) in a goroutine to start it.
func NewTable(id uint32, cfg *config.SceneConfig, host TableHost) *Table {
	rng := rand.New(rand.NewSource(int64(id)))
	return &Table{
		id:    id,
		cfg:   cfg,
		host:  host,
		inbox: make(chan Msg, inboxSize),
		game:  texas.NewGame(cfg),
		robot: texas.NewRobot(rng),
	}
}

// Send enqueues a message for the table goroutine.
// Returns false if the inbox is full (caller should treat as back-pressure).
func (t *Table) Send(m Msg) bool {
	select {
	case t.inbox <- m:
		return true
	default:
		return false
	}
}

// SetBlindSchedule sets the SNG blind escalation table.
// Call this after NewTable, before Run, if the scene is a SNG tournament.
func (t *Table) SetBlindSchedule(s []config.BlindInfoConfig) {
	t.blindSchedule = s
}

// Query sends a synchronous query message to the table goroutine and blocks
// until the goroutine replies or the provided timeout elapses.
// Returns nil on timeout.
func (t *Table) Query(kind MsgKind, timeout time.Duration) any {
	reply := make(QueryReply, 1)
	msg := Msg{Kind: kind, Reply: reply}
	if !t.Send(msg) {
		return nil
	}
	select {
	case v := <-reply:
		return v
	case <-time.After(timeout):
		return nil
	}
}

// Run is the table's main loop. It must be called in its own goroutine.
// It exits when ctx is cancelled.
func (t *Table) Run(ctx context.Context) {
	metrics.ActiveTables.Add(1)
	defer metrics.ActiveTables.Add(-1)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-t.inbox:
			t.dispatch(msg)
		case now := <-ticker.C:
			t.onTick(now)
		}
	}
}

// onTick runs every 100ms: fires expired timers and checks for game-start.
func (t *Table) onTick(now time.Time) {
	fired := t.timers.FireExpired(now)
	for _, id := range fired {
		t.onTimer(id)
	}

	// Auto-start if enough players are ready
	if t.game.CanStart() {
		t.processEvents(t.game.StartHand())
	}
}

func (t *Table) onTimer(id texas.TimerID) {
	switch id {
	case texas.TimerAction:
		t.processEvents(t.game.ActionTimeout())
	case texas.TimerCountdown:
		if t.game.CanStart() {
			t.processEvents(t.game.StartHand())
		}
	}
}

// ── Message dispatch ──────────────────────────────────────────────────────────

func (t *Table) dispatch(m Msg) {
	switch m.Kind {
	case MsgKindPlayerEnter:
		t.onPlayerEnter(m)
	case MsgKindPlayerExit:
		t.onPlayerExit(m)
	case MsgKindPlayerOffline:
		t.onPlayerOffline(m)
	case MsgKindPlayerReconnect:
		t.onPlayerReconnect(m)
	case MsgKindPlayerReady:
		t.onPlayerReady(m)
	case MsgKindPlayerSit:
		t.onPlayerSit(m)
	case MsgKindPlayerStand:
		t.onPlayerStand(m)
	case MsgKindGameMessage:
		t.onGameMessage(m)
	case MsgKindForceEnd:
		t.onForceEnd()
	case MsgKindSetBlindLevel:
		t.onSetBlindLevel(m)
	case MsgKindGetRankedPlayers:
		t.onGetRankedPlayers(m)
	case MsgKindGetCapacity:
		t.onGetCapacity(m)
	}
}

func (t *Table) onPlayerEnter(m Msg) {
	seat := t.firstEmptySeat()
	if seat < 0 {
		slog.Warn("table full, player cannot enter", "tableID", t.id, "userID", m.UserID)
		return
	}
	p := player.NewPlayer(m.UserID, m.NickName, m.Avatar, m.ChipDesk)
	p.RoleID = m.RoleID
	t.seated[seat] = p
	t.game.SeatPlayer(seat, p)
	slog.Info("player entered", "tableID", t.id, "userID", m.UserID, "seat", seat, "chips", m.ChipDesk)
}

func (t *Table) onPlayerExit(m Msg) {
	seat := t.game.FindSeat(m.UserID)
	if seat < 0 {
		return
	}
	t.seated[seat] = nil
	t.game.UnseatPlayer(seat)
	slog.Info("player exited", "tableID", t.id, "userID", m.UserID, "seat", seat)
}

func (t *Table) onPlayerOffline(m Msg) {
	seat := t.game.FindSeat(m.UserID)
	if seat < 0 {
		return
	}
	if p := t.seated[seat]; p != nil {
		p.NetStatus = player.NetOffline
	}
}

func (t *Table) onPlayerReconnect(m Msg) {
	seat := t.game.FindSeat(m.UserID)
	if seat < 0 {
		return
	}
	if p := t.seated[seat]; p != nil {
		p.NetStatus = player.NetOnline
	}
}

func (t *Table) onPlayerReady(m Msg) {
	seat := t.game.FindSeat(m.UserID)
	if seat < 0 {
		return
	}
	if p := t.seated[seat]; p != nil {
		p.IsReady = true
	}
	// If enough players ready, arm countdown timer
	if t.game.ReadyPlayerCount() >= 2 && !t.timers.Has(texas.TimerCountdown) {
		t.timers.Set(texas.TimerCountdown, 3*time.Second)
	}
}

func (t *Table) onPlayerSit(m Msg) {
	// Re-use enter logic with specified chair
	if m.Chair >= texas.MaxSeats {
		return
	}
	if t.seated[m.Chair] != nil {
		return
	}
	p := player.NewPlayer(m.UserID, m.NickName, m.Avatar, m.ChipDesk)
	p.IsReady = true
	t.seated[m.Chair] = p
	t.game.SeatPlayer(int(m.Chair), p)

	if t.game.ReadyPlayerCount() >= 2 && !t.timers.Has(texas.TimerCountdown) {
		t.timers.Set(texas.TimerCountdown, 3*time.Second)
	}
}

func (t *Table) onPlayerStand(m Msg) {
	seat := t.game.FindSeat(m.UserID)
	if seat < 0 {
		return
	}
	if p := t.seated[seat]; p != nil {
		p.IsReady = false
		p.LeaveSeat = true
	}
}

func (t *Table) onGameMessage(m Msg) {
	seat := t.game.FindSeat(m.UserID)
	if seat < 0 {
		return
	}

	switch m.Cmd {
	case CmdReqFold:
		t.processEvents(t.game.PlayerAction(seat, player.ActionFold, 0))
	case CmdReqCheck:
		t.processEvents(t.game.PlayerAction(seat, player.ActionCheck, 0))
	case CmdReqCall:
		t.processEvents(t.game.PlayerAction(seat, player.ActionCall, 0))
	case CmdReqBet:
		amount := decodeAmount(m.Payload)
		t.processEvents(t.game.PlayerAction(seat, player.ActionBet, amount))
	case CmdReqRaise:
		amount := decodeAmount(m.Payload)
		t.processEvents(t.game.PlayerAction(seat, player.ActionRaise, amount))
	case CmdReqAllIn:
		t.processEvents(t.game.PlayerAction(seat, player.ActionAllIn, 0))
	}
}

func (t *Table) onForceEnd() {
	// Mark all players as inactive so game ends gracefully on next tick.
	// Full implementation in Phase 5+.
}

// onSetBlindLevel looks up the requested level in the blind schedule and
// updates the game's blind info. Takes effect at the start of the next hand.
// C++ reference: OnGameBlindLevel → CSoGameLogic::SetBlindLevel
func (t *Table) onSetBlindLevel(m Msg) {
	for i := range t.blindSchedule {
		b := &t.blindSchedule[i]
		if b.Level == m.BlindLevel && b.Valid {
			t.game.SetBlindLevel(uint64(b.SmallBlind), uint64(b.BigBlind), uint64(b.Ante), b.Level)
			return
		}
	}
}

// onGetRankedPlayers returns player user IDs ordered by chip count (SNG ranking).
// Sends a []uint32 on m.Reply.
func (t *Table) onGetRankedPlayers(m Msg) {
	if m.Reply == nil {
		return
	}
	seats := t.game.RankedSeats()
	ids := make([]uint32, 0, len(seats))
	for _, seat := range seats {
		if p := t.seated[seat]; p != nil {
			ids = append(ids, p.UserID)
		}
	}
	m.Reply <- ids
}

// onGetCapacity returns the configured max seat count.
// Sends an int32 on m.Reply.
func (t *Table) onGetCapacity(m Msg) {
	if m.Reply == nil {
		return
	}
	m.Reply <- int32(t.cfg.SeatCount)
}

// ── Event processing ──────────────────────────────────────────────────────────

// processEvents executes each Event returned by game logic methods.
// After processing, it checks whether the current player is a robot and
// triggers an immediate robot action if so.
func (t *Table) processEvents(events []texas.Event) {
	for _, e := range events {
		switch e.Kind {
		case texas.EventSendAll:
			_ = t.host.SendToAll(e.Cmd, e.Payload)

		case texas.EventSendTo:
			_ = t.host.SendToPlayer(e.TargetUserID, e.Cmd, e.Payload)

		case texas.EventSendExcept:
			_ = t.host.SendExcept(e.ExcludeUserID, e.Cmd, e.Payload)

		case texas.EventSetTimer:
			d := time.Duration(e.TimerDuration) * time.Millisecond
			t.timers.Set(e.TimerID, d)

		case texas.EventCancelTimer:
			t.timers.Remove(e.TimerID)

		case texas.EventSettleBill:
			bill := BuildBillResult(e)
			_ = t.host.SetGameBill(bill)
			metrics.BillsTotal.Add(1)
			slog.Debug("bill settled",
				"tableID", t.id,
				"userID", bill.UserID,
				"win", bill.WinChips,
				"fee", bill.ServiceFee,
			)

		case texas.EventGameStart:
			_ = t.host.SetGameStart()
			slog.Info("hand started", "tableID", t.id)

		case texas.EventGameEnd:
			_ = t.host.SetGameEnd()
			_ = t.host.SendToAll(CmdNotifyGameResult, nil)
			metrics.HandsTotal.Add(1)
			slog.Info("hand ended",
				"tableID", t.id,
				"hands_total", metrics.HandsTotal.Value(),
			)

		case texas.EventKickPlayer:
			_ = t.host.KickPlayer(e.TargetUserID, e.KickReasonID)
		}
	}

	// If the game is waiting for a robot to act, decide immediately.
	t.maybeRobotAct()
}

// maybeRobotAct checks whether the current actor is a robot player and,
// if so, calls Game.PlayerAction with the robot's chosen action.
// This keeps robot turns instantaneous rather than waiting for a timeout.
func (t *Table) maybeRobotAct() {
	seat := t.game.CurrentSeat()
	if seat < 0 {
		return
	}
	p := t.seated[seat]
	if p == nil || !p.IsRobot {
		return
	}
	action, amount := t.robot.Decide(t.game, seat)
	metrics.ActionsTotal.Add(1)
	metrics.RobotActionsTotal.Add(1)
	t.processEvents(t.game.PlayerAction(seat, action, amount))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (t *Table) firstEmptySeat() int {
	for i := 0; i < texas.MaxSeats; i++ {
		if t.seated[i] == nil {
			return i
		}
	}
	return -1
}

// decodeAmount extracts a uint64 chip amount from a raw payload.
// Until Protobuf is wired, payload[0..7] is treated as big-endian uint64.
func decodeAmount(payload []byte) uint64 {
	if len(payload) < 8 {
		return 0
	}
	var v uint64
	for i := 0; i < 8; i++ {
		v = (v << 8) | uint64(payload[i])
	}
	return v
}
