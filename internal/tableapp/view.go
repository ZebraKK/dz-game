package tableapp

import (
	"sort"
	"time"

	"github.com/qbgames/qbdz-game/internal/player"
	"github.com/qbgames/qbdz-game/internal/protocol"
	"github.com/qbgames/qbdz-game/internal/texas"
)

func (t *Table) buildSnapshot(p *participant) protocol.SnapshotPayload {
	snapshot := protocol.SnapshotPayload{
		TableID:    t.cfg.TableID,
		Phase:      t.phase,
		Config:     t.cfg.runtimeConfig(),
		Board:      []string{},
		Pot:        protocol.PotView{},
		Seats:      make([]protocol.SeatView, 0, MaxSeats),
		Spectators: make([]protocol.SpectatorView, 0),
		You: protocol.SelfView{
			PlayerID:     p.playerID,
			DisplayName:  p.displayName,
			Role:         participantRole(p),
			HoleCards:    []string{},
			PendingSit:   p.pendingSeat >= 0,
			LegalActions: []protocol.LegalAction{},
		},
	}
	if t.handID != "" {
		snapshot.Board = cardStrings(t.game.Community())
		snapshot.Pot = protocol.PotView{Total: t.game.PotSize()}
		handID := t.handID
		snapshot.HandID = &handID
		button := t.game.DealerSeat() + 1
		sb := t.game.SmallBlindSeat() + 1
		bb := t.game.BigBlindSeat() + 1
		snapshot.ButtonSeat = &button
		snapshot.SmallBlindSeat = &sb
		snapshot.BigBlindSeat = &bb
	}
	if t.phase == "ready" && !t.readyDeadline.IsZero() {
		deadline := t.readyDeadline.UTC().Format(time.RFC3339Nano)
		snapshot.ReadyDeadline = &deadline
	}
	if t.isHandActive() && t.turnID != "" && t.turnSeat >= 0 {
		turnPlayerID := ""
		if s := t.seats[t.turnSeat]; s != nil && s.participant != nil {
			turnPlayerID = s.participant.playerID
		}
		snapshot.CurrentTurn = &protocol.TurnView{
			TurnID:   t.turnID,
			Seat:     t.turnSeat + 1,
			PlayerID: turnPlayerID,
			Deadline: t.actionUntil.UTC().Format(time.RFC3339Nano),
			Thinking: t.thinkingActive,
		}
	}

	for seat := 0; seat < MaxSeats; seat++ {
		snapshot.Seats = append(snapshot.Seats, t.buildSeatView(seat))
	}
	for _, participant := range t.participants {
		if participant.seat >= 0 {
			continue
		}
		connection := "offline"
		if participant.connected {
			connection = "online"
		}
		snapshot.Spectators = append(snapshot.Spectators, protocol.SpectatorView{
			PlayerID:    participant.playerID,
			DisplayName: participant.displayName,
			Connection:  connection,
			PendingSit:  participant.pendingSeat >= 0,
		})
	}
	sort.Slice(snapshot.Spectators, func(i, j int) bool {
		if snapshot.Spectators[i].DisplayName == snapshot.Spectators[j].DisplayName {
			return snapshot.Spectators[i].PlayerID < snapshot.Spectators[j].PlayerID
		}
		return snapshot.Spectators[i].DisplayName < snapshot.Spectators[j].DisplayName
	})

	if p.seat >= 0 {
		seat := p.seat
		snapshot.You.Seat = intPtr(seat + 1)
		if s := t.seats[seat]; s != nil && s.player != nil {
			snapshot.You.Ready = s.player.IsReady
		}
		if t.handID != "" {
			hole := t.game.PlayerCards(seat)
			if hole[0].IsValid() && hole[1].IsValid() {
				snapshot.You.HoleCards = cardStrings(hole[:])
			}
		}
		if t.isHandActive() && t.turnSeat == seat && t.turnID != "" {
			snapshot.You.LegalActions = t.legalActions(seat)
			turnID := t.turnID
			snapshot.You.TurnID = &turnID
		}
	}
	if snapshot.You.HoleCards == nil {
		snapshot.You.HoleCards = []string{}
	}
	if snapshot.You.LegalActions == nil {
		snapshot.You.LegalActions = []protocol.LegalAction{}
	}
	return snapshot
}

func (t *Table) buildSeatView(seat int) protocol.SeatView {
	state := t.game.SeatState(seat)
	if !state.Occupied {
		return protocol.SeatView{
			Seat:          seat + 1,
			Occupancy:     "empty",
			RevealedCards: []string{},
		}
	}

	view := protocol.SeatView{
		Seat:          seat + 1,
		Occupancy:     "occupied",
		Stack:         state.Chips,
		RevealedCards: []string{},
	}
	if t.handID != "" {
		view.StreetBet = state.StreetBet
		view.HandCommitted = state.HandCommitted
		view.IsDealer = t.game.DealerSeat() == seat
		view.IsSmallBlind = t.game.SmallBlindSeat() == seat
		view.IsBigBlind = t.game.BigBlindSeat() == seat
	}
	s := t.seats[seat]
	if s != nil {
		if s.participant != nil {
			view.PlayerID = stringPtr(s.participant.playerID)
			view.DisplayName = stringPtr(s.participant.displayName)
			connection := "offline"
			if s.participant.connected {
				connection = "online"
			}
			view.Connection = &connection
		}
		if s.player != nil {
			view.Ready = s.player.IsReady
		}
		view.LastAction = cloneLastAction(s.lastAction)
		if len(s.revealed) > 0 {
			view.RevealedCards = append([]string(nil), s.revealed...)
		}
		participation := t.participationString(state, s, seat)
		view.Participation = &participation
	}
	return view
}

func (t *Table) participationString(state texas.SeatState, s *seatState, seat int) string {
	inLastHand := false
	if t.phase == "settled" {
		for _, participantSeat := range t.handParticipants {
			if participantSeat == seat {
				inLastHand = true
				break
			}
		}
	}
	if state.InHand || inLastHand {
		if state.Action == player.ActionFold || (t.lastActions[seat] != nil && t.lastActions[seat].Type == "fold") {
			return "folded"
		}
		if state.AllIn {
			return "all_in"
		}
		return "active"
	}
	if s != nil && s.sittingOut {
		return "sitting_out"
	}
	return "waiting"
}

func cloneLastAction(v *protocol.LastActionView) *protocol.LastActionView {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func stringPtr(v string) *string { return &v }
