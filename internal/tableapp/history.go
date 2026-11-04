package tableapp

import (
	"log/slog"
	"time"

	"github.com/qbgames/qbdz-game/internal/protocol"
)

type HandRecorder interface {
	RecordHand(record HandRecord) error
}

type HandActionRecord struct {
	Seat     int       `json:"seat"`
	PlayerID string    `json:"playerId"`
	Action   string    `json:"action"`
	Amount   uint64    `json:"amount,omitempty"`
	Auto     bool      `json:"auto"`
	At       time.Time `json:"at"`
}

type HandPlayerRecord struct {
	Seat          int      `json:"seat"`
	PlayerID      string   `json:"playerId"`
	DisplayName   string   `json:"displayName"`
	StartingStack uint64   `json:"startingStack"`
	FinalStack    uint64   `json:"finalStack"`
	Delta         int64    `json:"delta"`
	HoleCards     []string `json:"holeCards"`
	Folded        bool     `json:"folded"`
}

type HandRecord struct {
	TableID   string                  `json:"tableId"`
	HandID    string                  `json:"handId"`
	StartedAt time.Time               `json:"startedAt"`
	EndedAt   time.Time               `json:"endedAt"`
	Board     []string                `json:"board"`
	Players   []HandPlayerRecord      `json:"players"`
	Actions   []HandActionRecord      `json:"actions"`
	Pots      []protocol.PotResult    `json:"pots"`
	Winners   []protocol.WinnerResult `json:"winners"`
}

func (t *Table) SetRecorder(recorder HandRecorder) {
	t.recorder = recorder
}

func (t *Table) recordHand(board []string, pots []protocol.PotResult, winners []protocol.WinnerResult) {
	if t.recorder == nil {
		return
	}
	players := make([]HandPlayerRecord, 0, len(t.handParticipants))
	for _, seat := range t.handParticipants {
		s := t.seats[seat]
		if s == nil || s.player == nil || s.participant == nil {
			continue
		}
		holeCards := t.game.PlayerCards(seat)
		hole := cardStrings(holeCards[:])
		players = append(players, HandPlayerRecord{
			Seat:          seat + 1,
			PlayerID:      s.participant.playerID,
			DisplayName:   s.participant.displayName,
			StartingStack: t.startingStacks[seat],
			FinalStack:    s.player.ChipDesk,
			Delta:         int64(s.player.ChipDesk) - int64(t.startingStacks[seat]),
			HoleCards:     hole,
			Folded:        t.lastActions[seat] != nil && t.lastActions[seat].Type == "fold",
		})
	}
	record := HandRecord{
		TableID:   t.cfg.TableID,
		HandID:    t.handID,
		StartedAt: t.handStartedAt,
		EndedAt:   time.Now().UTC(),
		Board:     append([]string(nil), board...),
		Players:   players,
		Actions:   append([]HandActionRecord(nil), t.actionLog...),
		Pots:      append([]protocol.PotResult(nil), pots...),
		Winners:   append([]protocol.WinnerResult(nil), winners...),
	}
	if err := t.recorder.RecordHand(record); err != nil {
		slog.Error("record hand history failed", "tableID", t.cfg.TableID, "handID", t.handID, "err", err)
	}
}
