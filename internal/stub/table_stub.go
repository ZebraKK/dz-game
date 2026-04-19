// Package stub provides a TableHost implementation that records all calls
// in memory. Use it in unit/integration tests to verify what the game logic
// "sent" without needing a real network or upper server.
package stub

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/table"
)

// SentMessage records one call to SendToAll / SendToPlayer / SendExcept.
type SentMessage struct {
	TargetUserID uint32  // 0 = broadcast
	ExcludeUserID uint32 // non-zero for SendExcept
	Cmd          uint32
	Payload      []byte
}

// TableStub is an in-memory implementation of table.TableHost.
// All methods are goroutine-safe.
type TableStub struct {
	mu sync.Mutex

	SceneTypeVal  uint32
	SceneLevelVal uint32
	TableIDVal    uint32
	IsMatchVal    bool
	SysTime       int64

	// Recorded calls (appended on each call)
	Messages   []SentMessage
	Bills      []table.BillResult
	CoinDeltas []CoinDelta
	Kicks      []uint32 // userIDs kicked

	// Players available for GetPlayerInfo
	Players map[uint32]table.PlayerInfo

	// Error to return from all methods (inject for error-path testing)
	SendErr error
	BillErr error
}

// CoinDelta records a SetGameCoin call.
type CoinDelta struct {
	UserID uint32
	Delta  int64
	Source uint32
}

// NewTableStub creates a stub with sensible defaults.
func NewTableStub() *TableStub {
	return &TableStub{
		SceneTypeVal:  10301,
		SceneLevelVal: 10301,
		TableIDVal:    1,
		SysTime:       time.Now().Unix(),
		Players:       make(map[uint32]table.PlayerInfo),
	}
}

// AddPlayer registers a player so GetPlayerInfo succeeds.
func (s *TableStub) AddPlayer(p table.PlayerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Players[p.UserID] = p
}

// ── table.TableHost implementation ──────────────────────────────────────────

func (s *TableStub) SendToAll(cmd uint32, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SendErr != nil {
		return s.SendErr
	}
	s.Messages = append(s.Messages, SentMessage{Cmd: cmd, Payload: copyBytes(payload)})
	return nil
}

func (s *TableStub) SendToPlayer(userID uint32, cmd uint32, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SendErr != nil {
		return s.SendErr
	}
	s.Messages = append(s.Messages, SentMessage{TargetUserID: userID, Cmd: cmd, Payload: copyBytes(payload)})
	return nil
}

func (s *TableStub) SendExcept(excludeUserID uint32, cmd uint32, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SendErr != nil {
		return s.SendErr
	}
	s.Messages = append(s.Messages, SentMessage{ExcludeUserID: excludeUserID, Cmd: cmd, Payload: copyBytes(payload)})
	return nil
}

func (s *TableStub) SetGameStart() error  { return nil }
func (s *TableStub) SetGameEnd() error    { return nil }
func (s *TableStub) DismissGame() error   { return nil }

func (s *TableStub) SetGameBill(bill table.BillResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.BillErr != nil {
		return s.BillErr
	}
	s.Bills = append(s.Bills, bill)
	return nil
}

func (s *TableStub) SetGameCoin(userID uint32, delta int64, source uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CoinDeltas = append(s.CoinDeltas, CoinDelta{userID, delta, source})
	return nil
}

func (s *TableStub) SetGameExp(userID uint32, _ int32, _ uint32) error { return nil }

func (s *TableStub) KickPlayer(userID uint32, _ uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Kicks = append(s.Kicks, userID)
	return nil
}

func (s *TableStub) MakePlayerStand(userID uint32) error { return nil }

func (s *TableStub) GetPlayerInfo(userID uint32) (table.PlayerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.Players[userID]
	if !ok {
		return table.PlayerInfo{}, fmt.Errorf("stub: player %d not found", userID)
	}
	return p, nil
}

func (s *TableStub) GetPlayerGameMoney(userID uint32) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.Players[userID]
	if !ok {
		return 0, fmt.Errorf("stub: player %d not found", userID)
	}
	return p.GameMoney, nil
}

func (s *TableStub) SetMatchChip(_ uint32, _ int32, _ uint32) error   { return nil }
func (s *TableStub) SetMatchWeedOut(_ uint32, _ bool, _ uint32) error  { return nil }
func (s *TableStub) CowboyBet(_ uint32, _ uint64) error               { return nil }

func (s *TableStub) GetSceneType() uint32        { return s.SceneTypeVal }
func (s *TableStub) GetSceneLevel() uint32       { return s.SceneLevelVal }
func (s *TableStub) IsMatch() bool               { return s.IsMatchVal }
func (s *TableStub) IsUnderMaintenance() bool    { return false }
func (s *TableStub) GetSysTime() int64           { return s.SysTime }
func (s *TableStub) GetTableID() uint32          { return s.TableIDVal }
func (s *TableStub) SendTaskEvent(_ uint32, _ uint32) error { return nil }
func (s *TableStub) SendBroadcastEvent(_ uint32, _ uint32, _ uint32) {}
func (s *TableStub) ReportJackpot(_ []uint32, _ uint32)              {}

// ── Test helpers ─────────────────────────────────────────────────────────────

// HasCmd returns true if any message with the given cmd was recorded.
func (s *TableStub) HasCmd(cmd uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.Messages {
		if m.Cmd == cmd {
			return true
		}
	}
	return false
}

// WaitForCmd blocks until a message with cmd appears or timeout elapses.
// Use in integration tests that run a real table goroutine.
func (s *TableStub) WaitForCmd(t *testing.T, cmd uint32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.HasCmd(cmd) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("WaitForCmd: cmd %d not seen within %v", cmd, timeout)
}

// BillFor returns all bills recorded for a given userID.
func (s *TableStub) BillFor(userID uint32) []table.BillResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []table.BillResult
	for _, b := range s.Bills {
		if b.UserID == userID {
			out = append(out, b)
		}
	}
	return out
}

// Reset clears all recorded data (useful between sub-tests).
func (s *TableStub) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = s.Messages[:0]
	s.Bills = s.Bills[:0]
	s.CoinDeltas = s.CoinDeltas[:0]
	s.Kicks = s.Kicks[:0]
}

func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
