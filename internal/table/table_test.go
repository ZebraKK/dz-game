package table_test

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/stub"
	"github.com/qbgames/qbdz-game/internal/table"
)

// testConfig returns a minimal SceneConfig for tests.
func testConfig() *config.SceneConfig {
	cfg := config.DefaultSceneConfig()
	cfg.SeatCount = 2
	cfg.SmallBlind = 50
	cfg.BigBlind = 100
	cfg.MinBuyIn = 500
	cfg.MaxBuyIn = 5000
	cfg.ActionTimeSec = 5 // short timeout for tests
	return &cfg
}

// runTable starts a table in the background and returns a cancel func.
func runTable(t *table.Table) context.CancelFunc {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go t.Run(ctx)
	return cancel
}

// TestHeadsUpFold: 2 players, one immediately folds → other wins pot.
func TestHeadsUpFold(t *testing.T) {
	st := stub.NewTableStub()
	st.AddPlayer(table.PlayerInfo{UserID: 1001, NickName: "Alice", GameMoney: 5000})
	st.AddPlayer(table.PlayerInfo{UserID: 1002, NickName: "Bob", GameMoney: 5000})

	cfg := testConfig()
	tbl := table.NewTable(1, cfg, st)
	cancel := runTable(tbl)
	defer cancel()

	// Sit both players (triggers countdown)
	tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: 1001, Chair: 0, NickName: "Alice", ChipDesk: 2000})
	tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: 1002, Chair: 1, NickName: "Bob", ChipDesk: 2000})

	// Wait for hand to start (countdown ~3s + a tick)
	st.WaitForCmd(t, table.CmdNotifyGameResult, 8*time.Second)

	// A game result must have been sent (one player folded via timeout or explicit fold).
	if !st.HasCmd(table.CmdNotifyGameResult) {
		t.Fatal("expected CmdNotifyGameResult")
	}
}

// TestHeadsUpExplicitFold: first actor folds, hand ends immediately.
// With 2 players: dealer=seat0(2001), SB=seat1(2002), BB=seat0(2001).
// PreFlop first actor = seat after BB = seat1 (2002).
func TestHeadsUpExplicitFold(t *testing.T) {
	st := stub.NewTableStub()
	cfg := testConfig()
	tbl := table.NewTable(2, cfg, st)
	cancel := runTable(tbl)
	defer cancel()

	tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: 2001, Chair: 0, NickName: "P1", ChipDesk: 1000})
	tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: 2002, Chair: 1, NickName: "P2", ChipDesk: 1000})

	// Wait for hand start (CmdNotifyAction = it's someone's turn)
	st.WaitForCmd(t, table.CmdNotifyAction, 8*time.Second)

	// In preflop with 2 players, seat1 (2002) acts first (UTG / SB position)
	tbl.Send(table.Msg{
		Kind:   table.MsgKindGameMessage,
		UserID: 2002,
		Cmd:    table.CmdReqFold,
	})

	// Game result should arrive quickly (well within action timeout)
	st.WaitForCmd(t, table.CmdNotifyGameResult, 3*time.Second)

	if !st.HasCmd(table.CmdNotifyGameResult) {
		t.Fatal("expected game result after fold")
	}
}

// TestBillsAreIssued: after a hand, at least one bill is recorded.
func TestBillsAreIssued(t *testing.T) {
	st := stub.NewTableStub()
	cfg := testConfig()
	tbl := table.NewTable(3, cfg, st)
	cancel := runTable(tbl)
	defer cancel()

	tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: 3001, Chair: 0, NickName: "X", ChipDesk: 2000})
	tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: 3002, Chair: 1, NickName: "Y", ChipDesk: 2000})

	// Wait for result (timeout-driven fold)
	st.WaitForCmd(t, table.CmdNotifyGameResult, 9*time.Second)

	st.WaitForCmd(t, table.CmdNotifyGameResult, time.Second) // double-check settled

	// At least one bill should have been issued
	allBills := append(st.BillFor(3001), st.BillFor(3002)...)
	if len(allBills) == 0 {
		t.Error("expected at least one bill after hand completion")
	}
}

// TestRaceDetector: run two tables concurrently to catch data races.
func TestRaceDetector(t *testing.T) {
	run := func(tableID uint32) {
		st := stub.NewTableStub()
		cfg := testConfig()
		tbl := table.NewTable(tableID, cfg, st)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		go tbl.Run(ctx)

		tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: tableID*10 + 1, Chair: 0, NickName: "A", ChipDesk: 1000})
		tbl.Send(table.Msg{Kind: table.MsgKindPlayerSit, UserID: tableID*10 + 2, Chair: 1, NickName: "B", ChipDesk: 1000})

		// Just let timers fire — we're only checking for races
		<-ctx.Done()
	}

	done := make(chan struct{}, 3)
	for i := uint32(100); i < 103; i++ {
		go func(id uint32) {
			run(id)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 3; i++ {
		<-done
	}
}
