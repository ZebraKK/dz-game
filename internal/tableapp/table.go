package tableapp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/qbgames/qbdz-game/internal/cards"
	"github.com/qbgames/qbdz-game/internal/player"
	"github.com/qbgames/qbdz-game/internal/protocol"
	"github.com/qbgames/qbdz-game/internal/texas"
)

// ClientSink is implemented by the gateway's per-connection sender.
type ClientSink interface {
	Send(messageType string, stateVersion int64, payload any) bool
}

type participant struct {
	playerID    string
	displayName string
	userID      uint32
	sink        ClientSink
	seat        int
	pendingSeat int
	connected   bool
}

type seatState struct {
	participant *participant
	player      *player.Player
	sittingOut  bool
	lastAction  *protocol.LastActionView
	revealed    []string
}

type cachedAction struct {
	payload protocol.ActionPayload
	result  protocol.ActionResultPayload
}

type commandKind int

const (
	cmdJoin commandKind = iota
	cmdSit
	cmdCancelSit
	cmdSitOut
	cmdLeave
	cmdAction
	cmdStatus
	cmdDisconnect
)

type command struct {
	kind          commandKind
	participantID string
	displayName   string
	sink          ClientSink
	preferredSeat *int
	action        protocol.ActionPayload
	reply         chan commandResult
}

type commandResult struct {
	join protocol.JoinOKPayload
	err  *Error
}

type Table struct {
	cfg   Config
	game  *texas.Game
	inbox chan command
	done  chan struct{}
	stop  context.CancelFunc

	participants map[string]*participant
	seats        [MaxSeats]*seatState
	nextUserID   uint32

	stateVersion int64
	phase        string
	handID       string
	handNumber   uint64
	actionSeq    uint64
	turnID       string
	turnSeat     int
	actionUntil  time.Time
	countdownAt  time.Time
	resultUntil  time.Time

	handParticipants []int
	startingStacks   map[int]uint64
	lastActions      [MaxSeats]*protocol.LastActionView
	revealedCards    [MaxSeats][]string
	actionCache      map[string]cachedAction
	autoSeq          uint64
	recorder         HandRecorder
	handStartedAt    time.Time
	actionLog        []HandActionRecord
}

func NewTable(cfg Config) *Table {
	ctx, cancel := context.WithCancel(context.Background())
	sceneCfg := cfg.sceneConfig()
	t := &Table{
		cfg:            cfg,
		game:           texas.NewGame(&sceneCfg),
		inbox:          make(chan command, 256),
		done:           make(chan struct{}),
		stop:           cancel,
		participants:   make(map[string]*participant),
		turnSeat:       -1,
		startingStacks: make(map[int]uint64),
		actionCache:    make(map[string]cachedAction),
	}
	t.phase = "idle"
	go t.run(ctx)
	return t
}

func (t *Table) Close() {
	t.stop()
	<-t.done
}

func (t *Table) run(ctx context.Context) {
	defer close(t.done)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-t.inbox:
			t.dispatch(cmd)
		case now := <-ticker.C:
			t.onTick(now)
		}
	}
}

func (t *Table) dispatch(cmd command) {
	var result commandResult
	switch cmd.kind {
	case cmdJoin:
		result.join, result.err = t.handleJoin(cmd)
	case cmdSit:
		result.err = t.handleSit(cmd.participantID, cmd.preferredSeat)
	case cmdCancelSit:
		result.err = t.handleCancelSit(cmd.participantID)
	case cmdSitOut:
		result.err = t.handleSitOut(cmd.participantID)
	case cmdLeave:
		result.err = t.handleLeave(cmd.participantID)
	case cmdAction:
		result.err = t.handleAction(cmd.participantID, cmd.action)
	case cmdStatus:
		if p := t.participants[cmd.participantID]; p != nil {
			t.sendSnapshot(p)
		}
	case cmdDisconnect:
		t.handleDisconnect(cmd.participantID, cmd.sink)
	}
	if cmd.reply != nil {
		cmd.reply <- result
	}
}

func (t *Table) call(ctx context.Context, cmd command) (commandResult, error) {
	cmd.reply = make(chan commandResult, 1)
	select {
	case t.inbox <- cmd:
	case <-ctx.Done():
		return commandResult{}, ctx.Err()
	case <-t.done:
		return commandResult{}, context.Canceled
	}
	select {
	case result := <-cmd.reply:
		return result, nil
	case <-ctx.Done():
		return commandResult{}, ctx.Err()
	case <-t.done:
		return commandResult{}, context.Canceled
	}
}

func (t *Table) Join(ctx context.Context, playerID, displayName string, sink ClientSink) (protocol.JoinOKPayload, error) {
	result, err := t.call(ctx, command{kind: cmdJoin, participantID: playerID, displayName: displayName, sink: sink})
	if err != nil {
		return protocol.JoinOKPayload{}, err
	}
	if result.err != nil {
		return protocol.JoinOKPayload{}, result.err
	}
	return result.join, nil
}

func (t *Table) Sit(ctx context.Context, playerID string, preferredSeat *int) error {
	return t.command(ctx, command{kind: cmdSit, participantID: playerID, preferredSeat: preferredSeat})
}

func (t *Table) CancelSit(ctx context.Context, playerID string) error {
	return t.command(ctx, command{kind: cmdCancelSit, participantID: playerID})
}

func (t *Table) SitOut(ctx context.Context, playerID string) error {
	return t.command(ctx, command{kind: cmdSitOut, participantID: playerID})
}

func (t *Table) Leave(ctx context.Context, playerID string) error {
	return t.command(ctx, command{kind: cmdLeave, participantID: playerID})
}

func (t *Table) Action(ctx context.Context, playerID string, action protocol.ActionPayload) error {
	return t.command(ctx, command{kind: cmdAction, participantID: playerID, action: action})
}

func (t *Table) Status(ctx context.Context, playerID string) error {
	return t.command(ctx, command{kind: cmdStatus, participantID: playerID})
}

func (t *Table) command(ctx context.Context, cmd command) error {
	result, err := t.call(ctx, cmd)
	if err != nil {
		return err
	}
	if result.err != nil {
		return result.err
	}
	return nil
}

func (t *Table) Disconnect(playerID string, sink ClientSink) {
	select {
	case t.inbox <- command{kind: cmdDisconnect, participantID: playerID, sink: sink}:
	default:
	}
}

func (t *Table) handleJoin(cmd command) (protocol.JoinOKPayload, *Error) {
	p := t.participants[cmd.participantID]
	if p == nil {
		t.nextUserID++
		p = &participant{
			playerID:    cmd.participantID,
			displayName: cmd.displayName,
			userID:      t.nextUserID,
			seat:        -1,
			pendingSeat: -1,
		}
		t.participants[cmd.participantID] = p
	}
	if cmd.displayName != "" {
		p.displayName = cmd.displayName
	}
	p.sink = cmd.sink
	p.connected = true
	t.stateVersion++

	role := "spectator"
	var seatPtr *int
	if p.seat >= 0 {
		role = "player"
		s := p.seat + 1
		seatPtr = &s
	}
	join := protocol.JoinOKPayload{
		TableID:    t.cfg.TableID,
		Role:       role,
		Seat:       seatPtr,
		PendingSit: p.pendingSeat >= 0,
	}
	p.sink.Send(protocol.TypeJoinOK, t.stateVersion, join)
	t.sendSnapshot(p)
	if role == "player" && t.isCurrentTurn(p.seat) {
		t.sendActionRequest(p)
	}
	return join, nil
}

func (t *Table) handleDisconnect(playerID string, sink ClientSink) {
	p := t.participants[playerID]
	if p == nil || p.sink != sink {
		return
	}
	p.connected = false
	p.sink = nil
	t.stateVersion++
	t.broadcastSnapshot()
}

func (t *Table) handleSit(playerID string, preferred *int) *Error {
	p := t.participants[playerID]
	if p == nil {
		return actorError(protocol.ErrNotJoined, "请先加入桌面")
	}
	if p.seat >= 0 {
		s := t.seats[p.seat]
		if s != nil && s.sittingOut {
			s.sittingOut = false
			s.player.IsReady = true
			t.stateVersion++
			t.broadcast(protocol.TypePlayerSitOut, protocol.PlayerSitOutPayload{
				Seat:     p.seat + 1,
				PlayerID: p.playerID,
				SitOut:   false,
			})
			t.broadcastSnapshot()
			t.startCountdownIfReady()
			return nil
		}
		return actorError(protocol.ErrAlreadySeated, "已经在座位上")
	}
	if p.pendingSeat >= 0 {
		return actorError(protocol.ErrSitPending, "已有待处理的入座申请")
	}
	seat, err := t.chooseSeat(preferred)
	if err != nil {
		return err
	}
	if !t.isIdle() {
		p.pendingSeat = seat
		t.stateVersion++
		if p.connected && p.sink != nil {
			p.sink.Send(protocol.TypeSitPending, t.stateVersion, protocol.SitPendingPayload{
				PreferredSeat: intPtr(seat + 1),
				Effective:     "next_hand",
				Message:       "已受理，将在下一手开始时入座",
			})
		}
		t.broadcastSnapshot()
		return nil
	}
	t.assignSeat(p, seat, true)
	return nil
}

func (t *Table) handleCancelSit(playerID string) *Error {
	p := t.participants[playerID]
	if p == nil {
		return actorError(protocol.ErrNotJoined, "请先加入桌面")
	}
	if p.pendingSeat < 0 {
		return nil
	}
	p.pendingSeat = -1
	t.stateVersion++
	if p.connected && p.sink != nil {
		p.sink.Send(protocol.TypeSitCancelled, t.stateVersion, protocol.SitCancelledPayload{Message: "已取消入座申请"})
	}
	t.broadcastSnapshot()
	return nil
}

func (t *Table) handleSitOut(playerID string) *Error {
	p := t.participants[playerID]
	if p == nil || p.seat < 0 || t.seats[p.seat] == nil {
		return actorError(protocol.ErrNotSeated, "当前不在座位上")
	}
	s := t.seats[p.seat]
	s.sittingOut = true
	s.player.IsReady = false
	t.stateVersion++
	t.broadcast(protocol.TypePlayerSitOut, protocol.PlayerSitOutPayload{
		Seat:     p.seat + 1,
		PlayerID: p.playerID,
		SitOut:   true,
	})
	t.broadcastSnapshot()
	return nil
}

func (t *Table) handleLeave(playerID string) *Error {
	p := t.participants[playerID]
	if p == nil {
		return actorError(protocol.ErrNotJoined, "请先加入桌面")
	}
	if p.seat < 0 {
		return actorError(protocol.ErrNotSeated, "当前不在座位上")
	}
	s := t.seats[p.seat]
	if s != nil && s.player.InGame {
		return actorError(protocol.ErrHandInProgress, "当前手进行中，暂不能离桌")
	}
	seat := p.seat
	playerIDForEvent := p.playerID
	t.game.UnseatPlayer(seat)
	t.seats[seat] = nil
	p.seat = -1
	t.stateVersion++
	t.broadcast(protocol.TypePlayerLeft, protocol.PlayerLeftPayload{Seat: seat + 1, PlayerID: playerIDForEvent})
	t.broadcastSnapshot()
	return nil
}

func (t *Table) assignSeat(p *participant, seat int, announce bool) {
	pl := player.NewPlayer(p.userID, p.displayName, "", t.cfg.StartingStack)
	pl.IsReady = true
	t.game.SeatPlayer(seat, pl)
	t.seats[seat] = &seatState{participant: p, player: pl}
	p.seat = seat
	p.pendingSeat = -1
	t.stateVersion++
	if announce {
		t.broadcast(protocol.TypeSeatAssigned, protocol.SeatAssignedPayload{
			Seat:        seat + 1,
			PlayerID:    p.playerID,
			DisplayName: p.displayName,
			Effective:   "immediate",
		})
	}
	t.broadcastSnapshot()
	t.startCountdownIfReady()
}

func (t *Table) chooseSeat(preferred *int) (int, *Error) {
	if preferred != nil {
		seat := *preferred
		if seat < 1 || seat > MaxSeats {
			return 0, actorError(protocol.ErrInvalidSeat, "座位号必须在 1 到 9 之间")
		}
		if t.seats[seat-1] != nil {
			return 0, actorError(protocol.ErrSeatOccupied, "目标座位已被占用")
		}
		return seat - 1, nil
	}
	for i := 0; i < MaxSeats; i++ {
		if t.seats[i] == nil {
			return i, nil
		}
	}
	return 0, actorError(protocol.ErrTableFull, "桌面已满")
}

func (t *Table) onTick(now time.Time) {
	switch {
	case t.phase == "countdown" && !now.Before(t.countdownAt):
		t.startHand(now)
	case t.isHandActive() && !t.actionUntil.IsZero() && !now.Before(t.actionUntil):
		t.handleAutoAction(now)
	case t.phase == "settled" && !now.Before(t.resultUntil):
		t.finishResultDisplay(now)
	}
}

func (t *Table) startCountdownIfReady() {
	if t.phase != "idle" {
		return
	}
	if t.readyCount() < t.cfg.MinPlayers {
		return
	}
	t.phase = "countdown"
	t.countdownAt = time.Now().Add(t.cfg.StartCountdown)
	t.stateVersion++
	t.broadcastSnapshot()
}

func (t *Table) readyCount() int {
	count := 0
	for _, s := range t.seats {
		if s != nil && !s.sittingOut && s.player != nil && s.player.IsReady && s.player.ChipDesk > 0 {
			count++
		}
	}
	return count
}

func (t *Table) startHand(now time.Time) {
	if !t.game.CanStart() {
		t.phase = "idle"
		t.broadcastSnapshot()
		return
	}
	t.handNumber++
	t.handID = fmt.Sprintf("hand-%06d", t.handNumber)
	t.actionSeq = 0
	t.turnID = ""
	t.turnSeat = -1
	t.actionUntil = time.Time{}
	t.actionCache = make(map[string]cachedAction)
	t.handStartedAt = now
	t.actionLog = nil
	t.lastActions = [MaxSeats]*protocol.LastActionView{}
	t.revealedCards = [MaxSeats][]string{}
	t.startingStacks = make(map[int]uint64)
	for i, s := range t.seats {
		if s != nil && s.player != nil && s.player.IsReady && s.player.ChipDesk > 0 {
			t.startingStacks[i] = s.player.ChipDesk
		}
	}

	t.game.StartHand()
	t.phase = phaseFromStatus(t.game.Status())
	t.handParticipants = t.participantsInHand()
	t.stateVersion++

	hand := protocol.HandStartedPayload{
		HandID:             t.handID,
		ButtonSeat:         t.game.DealerSeat() + 1,
		SmallBlindSeat:     t.game.SmallBlindSeat() + 1,
		BigBlindSeat:       t.game.BigBlindSeat() + 1,
		ParticipatingSeats: oneBased(t.handParticipants),
		StartingStacks:     seatStackMap(t.startingStacks),
	}
	t.broadcast(protocol.TypeHandStarted, hand)
	t.sendBlindPosted("small", t.game.SmallBlindSeat())
	t.sendBlindPosted("big", t.game.BigBlindSeat())

	for _, seat := range t.handParticipants {
		s := t.seats[seat]
		if s == nil || s.participant == nil || !s.participant.connected || s.participant.sink == nil {
			continue
		}
		cards := t.game.PlayerCards(seat)
		s.participant.sink.Send(protocol.TypeHoleCards, t.stateVersion, protocol.HoleCardsPayload{
			HandID: t.handID,
			Cards:  cardStrings(cards[:]),
		})
	}
	t.sendActionRequestToCurrent(now)
	t.broadcastSnapshot()
}

func (t *Table) sendBlindPosted(kind string, seat int) {
	if seat < 0 || seat >= MaxSeats || t.seats[seat] == nil || t.seats[seat].participant == nil {
		return
	}
	start := t.startingStacks[seat]
	state := t.game.SeatState(seat)
	amount := uint64(0)
	if start > state.Chips {
		amount = start - state.Chips
	}
	t.broadcast(protocol.TypeBlindPosted, protocol.BlindPostedPayload{
		HandID:     t.handID,
		Seat:       seat + 1,
		PlayerID:   t.seats[seat].participant.playerID,
		Blind:      kind,
		Amount:     amount,
		StackAfter: state.Chips,
		Pot:        t.game.PotSize(),
	})
}

func (t *Table) sendActionRequestToCurrent(now time.Time) {
	seat := t.game.CurrentSeat()
	if seat < 0 || !t.isHandActive() {
		t.turnSeat = -1
		t.turnID = ""
		t.actionUntil = time.Time{}
		return
	}
	t.turnSeat = seat
	t.actionSeq++
	t.turnID = fmt.Sprintf("turn-%s-%d", t.handID, t.actionSeq)
	t.actionUntil = now.Add(t.cfg.ActionTimeout)
	if s := t.seats[seat]; s != nil {
		t.sendActionRequest(s.participant)
	}
}

func (t *Table) sendActionRequest(p *participant) {
	if p == nil || p.seat < 0 || p.seat != t.turnSeat || t.turnID == "" || !p.connected || p.sink == nil {
		return
	}
	actions := t.legalActions(p.seat)
	if len(actions) == 0 {
		return
	}
	p.sink.Send(protocol.TypeActionRequest, t.stateVersion, protocol.ActionRequestPayload{
		HandID:       t.handID,
		TurnID:       t.turnID,
		PlayerID:     p.playerID,
		Seat:         p.seat + 1,
		Deadline:     t.actionUntil.UTC().Format(time.RFC3339Nano),
		ToCall:       t.game.ToCall(),
		Pot:          t.game.PotSize(),
		LegalActions: actions,
	})
}

func (t *Table) handleAutoAction(now time.Time) {
	seat := t.game.CurrentSeat()
	if seat < 0 || !t.isHandActive() {
		return
	}
	action := protocol.PlayerAction{Type: "fold"}
	for _, legal := range t.legalActions(seat) {
		if legal.Type == "check" {
			action = protocol.PlayerAction{Type: "check"}
			break
		}
	}
	t.autoSeq++
	id := fmt.Sprintf("auto-%s-%d", t.handID, t.autoSeq)
	t.applyAction(seat, protocol.ActionPayload{
		ActionID: id,
		HandID:   t.handID,
		TurnID:   t.turnID,
		Action:   action,
	}, true, now)
}

func (t *Table) handleAction(playerID string, action protocol.ActionPayload) *Error {
	p := t.participants[playerID]
	if p == nil || p.seat < 0 {
		return actorError(protocol.ErrNotSeated, "当前不在座位上")
	}
	if action.ActionID == "" {
		t.sendError(p, actorError(protocol.ErrInvalidAction, "缺少 actionId"))
		return nil
	}
	if cached, ok := t.actionCache[action.ActionID]; ok {
		if cached.payload.HandID == action.HandID && cached.payload.TurnID == action.TurnID &&
			cached.payload.Action.Type == action.Action.Type && cached.payload.Action.Amount == action.Action.Amount {
			if p.connected && p.sink != nil {
				p.sink.Send(protocol.TypeActionResult, t.stateVersion, cached.result)
			}
			return nil
		}
		err := actorError(protocol.ErrIDConflict, "动作编号已被其他动作使用")
		t.sendError(p, err)
		return nil
	}
	if action.HandID != t.handID || action.TurnID != t.turnID {
		t.sendError(p, actorErrorWithDetails(protocol.ErrStaleTurn, "动作已过期", map[string]any{"reason": "TURN_CHANGED"}))
		return nil
	}
	if err := t.applyAction(p.seat, action, false, time.Now()); err != nil {
		t.sendError(p, err)
	}
	return nil
}

func (t *Table) applyAction(seat int, action protocol.ActionPayload, auto bool, now time.Time) *Error {
	gameAction, amount, err := mapAction(action.Action)
	if err != nil {
		return err
	}
	oldBoard := len(t.game.Community())
	events, applyErr := t.game.ApplyAction(seat, gameAction, amount)
	if applyErr != nil {
		if rej, ok := applyErr.(*texas.ActionRejection); ok {
			return actorErrorWithDetails(rej.Code, "动作不合法", map[string]any{"reason": rej.Reason})
		}
		return actorError(protocol.ErrInvalidAction, "动作不合法")
	}

	state := t.game.SeatState(seat)
	last := &protocol.LastActionView{Type: action.Action.Type, Amount: action.Action.Amount, Auto: auto}
	t.lastActions[seat] = last
	if s := t.seats[seat]; s != nil {
		s.lastAction = last
	}
	t.stateVersion++
	result := protocol.ActionResultPayload{
		HandID:         t.handID,
		ActionID:       action.ActionID,
		PlayerID:       t.seats[seat].participant.playerID,
		Seat:           seat + 1,
		Action:         action.Action,
		Auto:           auto,
		Pot:            t.game.PotSize(),
		StackAfter:     state.Chips,
		StreetBetAfter: state.StreetBet,
	}
	if !auto {
		t.actionCache[action.ActionID] = cachedAction{payload: action, result: result}
	}
	t.broadcast(protocol.TypeActionResult, result)
	if s := t.seats[seat]; s != nil && s.participant != nil {
		t.actionLog = append(t.actionLog, HandActionRecord{
			Seat:     seat + 1,
			PlayerID: s.participant.playerID,
			Action:   action.Action.Type,
			Amount:   action.Action.Amount,
			Auto:     auto,
			At:       now,
		})
	}

	newBoard := len(t.game.Community())
	if newBoard > oldBoard {
		board := cardStrings(t.game.Community())
		t.broadcast(protocol.TypeBoardDealt, protocol.BoardDealtPayload{
			HandID: t.handID,
			Street: streetForBoardCount(newBoard),
			Cards:  append([]string(nil), board[oldBoard:]...),
			Board:  board,
			Pot:    t.game.PotSize(),
		})
	}

	if handEnded(events) {
		t.finishHand(events, now)
		return nil
	}
	t.phase = phaseFromStatus(t.game.Status())
	t.sendActionRequestToCurrent(now)
	t.broadcastSnapshot()
	return nil
}

func (t *Table) finishHand(events []texas.Event, now time.Time) {
	t.phase = "settled"
	t.resultUntil = now.Add(t.cfg.ResultDisplay)
	t.actionUntil = time.Time{}
	t.turnID = ""
	t.turnSeat = -1

	winners := make(map[uint32]uint64)
	for _, ev := range events {
		if ev.Kind == texas.EventSettleBill {
			winners[ev.BillUserID] += uint64(maxInt64(ev.WinChips))
		}
	}
	board := cardStrings(t.game.Community())
	reveals := make([]protocol.ShowdownReveal, 0, len(t.handParticipants))
	if len(board) == 5 && t.nonFoldedCount() > 1 {
		for _, seat := range t.handParticipants {
			if t.lastActions[seat] != nil && t.lastActions[seat].Type == "fold" {
				continue
			}
			cards := t.game.PlayerCards(seat)
			if !cards[0].IsValid() || !cards[1].IsValid() {
				continue
			}
			revealCards := cardStrings(cards[:])
			t.revealedCards[seat] = append([]string(nil), revealCards...)
			if s := t.seats[seat]; s != nil {
				s.revealed = append([]string(nil), revealCards...)
			}
			reveals = append(reveals, protocol.ShowdownReveal{
				Seat:     seat + 1,
				PlayerID: t.seats[seat].participant.playerID,
				Cards:    revealCards,
				HandRank: bestHandName(cards[:], t.game.Community()),
			})
		}
		t.broadcast(protocol.TypeShowdown, protocol.ShowdownRevealedPayload{
			HandID:  t.handID,
			Board:   board,
			Reveals: reveals,
		})
	}

	winnersBySeat := make(map[int]uint64)
	for userID, amount := range winners {
		if p := t.participantForUserID(userID); p != nil && p.seat >= 0 {
			winnersBySeat[p.seat] += amount
		}
	}
	winnerSeats := make([]int, 0, len(winnersBySeat))
	winnerResults := make([]protocol.WinnerResult, 0, len(winnersBySeat))
	for seat, amount := range winnersBySeat {
		winnerSeats = append(winnerSeats, seat)
		hole := t.game.PlayerCards(seat)
		winnerResults = append(winnerResults, protocol.WinnerResult{
			Seat:     seat + 1,
			PlayerID: t.seats[seat].participant.playerID,
			Amount:   amount,
			HandRank: bestHandName(hole[:], t.game.Community()),
		})
	}
	sort.Ints(winnerSeats)
	sort.Slice(winnerResults, func(i, j int) bool { return winnerResults[i].Seat < winnerResults[j].Seat })

	pots := t.buildPotResults(events, t.game.Pots())

	finalStacks := make(map[string]uint64)
	deltas := make(map[string]int64)
	for _, seat := range t.handParticipants {
		state := t.game.SeatState(seat)
		key := fmt.Sprintf("%d", seat+1)
		finalStacks[key] = state.Chips
		deltas[key] = int64(state.Chips) - int64(t.startingStacks[seat])
	}

	t.recordHand(board, pots, winnerResults)
	t.broadcast(protocol.TypeHandSettled, protocol.HandSettledPayload{
		HandID:      t.handID,
		Board:       board,
		Pots:        pots,
		Winners:     winnerResults,
		FinalStacks: finalStacks,
		Deltas:      deltas,
	})

	for _, seat := range t.handParticipants {
		s := t.seats[seat]
		if s == nil {
			continue
		}
		if s.player.ChipDesk == 0 {
			s.sittingOut = true
			s.player.IsReady = false
		}
	}
	t.broadcastSnapshot()
}

func (t *Table) buildPotResults(events []texas.Event, pools []texas.Pool) []protocol.PotResult {
	winnerUsers := make(map[int]map[uint32]bool)
	for _, ev := range events {
		if ev.Kind != texas.EventSettleBill {
			continue
		}
		if ev.PoolIndex < 0 || ev.PoolIndex >= len(pools) {
			continue
		}
		if winnerUsers[ev.PoolIndex] == nil {
			winnerUsers[ev.PoolIndex] = make(map[uint32]bool)
		}
		winnerUsers[ev.PoolIndex][ev.BillUserID] = true
	}

	out := make([]protocol.PotResult, 0, len(pools))
	for i, pool := range pools {
		winnerSeats := make([]int, 0, len(winnerUsers[i]))
		for userID := range winnerUsers[i] {
			if p := t.participantForUserID(userID); p != nil && p.seat >= 0 {
				winnerSeats = append(winnerSeats, p.seat+1)
			}
		}
		sort.Ints(winnerSeats)
		out = append(out, protocol.PotResult{
			Index:         i + 1,
			Amount:        pool.Amount,
			EligibleSeats: eligibleSeats(pool),
			WinnerSeats:   winnerSeats,
		})
	}
	return out
}

func (t *Table) finishResultDisplay(now time.Time) {
	t.assignPendingSits()
	t.phase = "idle"
	t.handID = ""
	t.handParticipants = nil
	t.startingStacks = make(map[int]uint64)
	t.lastActions = [MaxSeats]*protocol.LastActionView{}
	t.revealedCards = [MaxSeats][]string{}
	for _, s := range t.seats {
		if s != nil {
			s.lastAction = nil
			s.revealed = nil
		}
	}
	t.stateVersion++
	t.broadcastSnapshot()
	t.startCountdownIfReady()
}

func (t *Table) assignPendingSits() {
	ids := make([]string, 0)
	for id, p := range t.participants {
		if p.pendingSeat >= 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := t.participants[id]
		seat := p.pendingSeat
		if seat < 0 || seat >= MaxSeats || t.seats[seat] != nil {
			seat = -1
			for i := 0; i < MaxSeats; i++ {
				if t.seats[i] == nil {
					seat = i
					break
				}
			}
		}
		if seat >= 0 {
			t.assignSeat(p, seat, true)
		}
	}
}

func (t *Table) participantsInHand() []int {
	out := make([]int, 0)
	for i := 0; i < MaxSeats; i++ {
		s := t.seats[i]
		if s != nil && s.player != nil && s.player.InGame {
			out = append(out, i)
		}
	}
	return out
}

func (t *Table) nonFoldedCount() int {
	count := 0
	for _, seat := range t.handParticipants {
		if t.lastActions[seat] == nil || t.lastActions[seat].Type != "fold" {
			count++
		}
	}
	return count
}

func (t *Table) isHandActive() bool {
	return t.phase == "preflop" || t.phase == "flop" || t.phase == "turn" || t.phase == "river"
}

func (t *Table) isIdle() bool { return t.phase == "idle" }

func (t *Table) isCurrentTurn(seat int) bool {
	return t.isHandActive() && seat == t.turnSeat && t.turnID != ""
}

func (t *Table) legalActions(seat int) []protocol.LegalAction {
	domain := t.game.LegalActions(seat)
	out := make([]protocol.LegalAction, 0, len(domain))
	for _, a := range domain {
		action := protocol.LegalAction{Amount: a.Amount, MinAmount: a.MinAmount, MaxAmount: a.MaxAmount}
		switch a.Action {
		case player.ActionFold:
			action.Type = "fold"
		case player.ActionCheck:
			action.Type = "check"
		case player.ActionCall:
			action.Type = "call"
		case player.ActionRaise:
			action.Type = "raise"
		case player.ActionAllIn:
			action.Type = "all_in"
		default:
			continue
		}
		out = append(out, action)
	}
	return out
}

func mapAction(action protocol.PlayerAction) (player.Action, uint64, *Error) {
	switch action.Type {
	case "fold":
		return player.ActionFold, 0, nil
	case "check":
		return player.ActionCheck, 0, nil
	case "call":
		return player.ActionCall, action.Amount, nil
	case "raise":
		return player.ActionRaise, action.Amount, nil
	case "all_in":
		return player.ActionAllIn, action.Amount, nil
	default:
		return player.ActionNull, 0, actorError(protocol.ErrInvalidAction, "动作不合法")
	}
}

func (t *Table) broadcast(messageType string, payload any) {
	for _, p := range t.participants {
		if p.connected && p.sink != nil {
			p.sink.Send(messageType, t.stateVersion, payload)
		}
	}
}

func (t *Table) broadcastSnapshot() {
	for _, p := range t.participants {
		t.sendSnapshot(p)
	}
}

func (t *Table) sendSnapshot(p *participant) {
	if p == nil || !p.connected || p.sink == nil {
		return
	}
	p.sink.Send(protocol.TypeSnapshot, t.stateVersion, t.buildSnapshot(p))
}

func (t *Table) sendError(p *participant, err *Error) {
	if p == nil || !p.connected || p.sink == nil || err == nil {
		return
	}
	p.sink.Send(protocol.TypeError, t.stateVersion, protocol.ErrorPayload{
		Code:      err.Code,
		Message:   err.Message,
		Retryable: err.Retryable,
		Details:   err.Details,
	})
}

func phaseFromStatus(status texas.GameStatus) string {
	switch status {
	case texas.StatusPreFlop:
		return "preflop"
	case texas.StatusFlop:
		return "flop"
	case texas.StatusTurn:
		return "turn"
	case texas.StatusRiver:
		return "river"
	case texas.StatusResult:
		return "settled"
	case texas.StatusCountdown:
		return "countdown"
	default:
		return "idle"
	}
}

func handEnded(events []texas.Event) bool {
	for _, ev := range events {
		if ev.Kind == texas.EventGameEnd || ev.Kind == texas.EventSettleBill {
			return true
		}
	}
	return false
}

func streetForBoardCount(n int) string {
	switch n {
	case 3:
		return "flop"
	case 4:
		return "turn"
	case 5:
		return "river"
	default:
		return "flop"
	}
}

func oneBased(seats []int) []int {
	out := make([]int, 0, len(seats))
	for _, seat := range seats {
		out = append(out, seat+1)
	}
	return out
}

func seatStackMap(stacks map[int]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(stacks))
	for seat, chips := range stacks {
		out[fmt.Sprintf("%d", seat+1)] = chips
	}
	return out
}

func eligibleSeats(pool texas.Pool) []int {
	out := make([]int, 0)
	for i, ok := range pool.Eligible {
		if ok {
			out = append(out, i+1)
		}
	}
	return out
}

func bestHandName(hole []cards.Card, board []cards.Card) string {
	all := make([]cards.Card, 0, len(hole)+len(board))
	all = append(all, hole...)
	all = append(all, board...)
	if len(all) < 5 {
		return cards.HandNull.String()
	}
	return cards.BestHand(all).Type.String()
}

func maxInt64(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func intPtr(v int) *int { return &v }

func cardStrings(in []cards.Card) []string {
	out := make([]string, 0, len(in))
	for _, c := range in {
		if !c.IsValid() {
			continue
		}
		out = append(out, rankString(c.Number)+suitString(c.Shape))
	}
	return out
}

func rankString(n cards.Number) string {
	if n == cards.Number10 {
		return "T"
	}
	return n.String()
}

func suitString(s cards.Shape) string {
	switch s {
	case cards.ShapeSpade:
		return "s"
	case cards.ShapeHearts:
		return "h"
	case cards.ShapeClubs:
		return "c"
	case cards.ShapeDiamonds:
		return "d"
	default:
		return "?"
	}
}

func (t *Table) participantForUserID(userID uint32) *participant {
	for _, p := range t.participants {
		if p.userID == userID {
			return p
		}
	}
	return nil
}

func participantRole(p *participant) string {
	if p != nil && p.seat >= 0 {
		return "player"
	}
	return "spectator"
}
