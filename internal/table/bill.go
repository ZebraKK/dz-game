package table

import (
	"encoding/json"

	"github.com/qbgames/qbdz-game/internal/texas"
)

// Bill IDs match C++ GAME_BILL_* constants in SoGameCommon.h.
const (
	BillIDUser  uint32 = 300 // GAME_BILL_USER  — per-player result
	BillIDTable uint32 = 301 // GAME_BILL_TABLE — table aggregate (unused in Phase 5)
)

// billData is the JSON payload embedded inside BillResult.BillData.
// It gives the upper server a human-readable and machine-parseable record
// of this hand's financial outcome for one player.
//
// C++ analogue: rstSOstr serialised by CSoBillInfo::Serialise().
type billData struct {
	HandCount  uint32 `json:"hand"`   // hand sequence number
	BetChips   uint64 `json:"bet"`    // total chips wagered
	WinChips   int64  `json:"win"`    // net chips won (after fee)
	ServiceFee uint64 `json:"fee"`    // rake taken
	Reason     int    `json:"reason"` // ResultReason value
}

// BuildBillResult constructs the BillResult for a single player settlement event.
// e is the EventSettleBill emitted by texas.Game.settle(); e.BillReason carries
// why the hand ended (fold / all-in / normal showdown).
func BuildBillResult(e texas.Event) BillResult {
	data, _ := json.Marshal(billData{
		HandCount:  e.HandCount,
		BetChips:   e.BetChips,
		WinChips:   e.WinChips,
		ServiceFee: e.ServiceFee,
		Reason:     int(e.BillReason),
	})

	return BillResult{
		UserID:     e.BillUserID,
		BillID:     BillIDUser,
		WinChips:   e.WinChips,
		BetChips:   e.BetChips,
		ServiceFee: e.ServiceFee,
		BillData:   string(data),
	}
}
