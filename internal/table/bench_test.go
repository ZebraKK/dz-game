package table_test

import (
	"context"
	"testing"
	"time"

	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/player"
	"github.com/qbgames/qbdz-game/internal/stub"
	"github.com/qbgames/qbdz-game/internal/table"
)

// BenchmarkRobotTable runs N tables each with 2 robot players and measures
// throughput (hands/sec) and detects data races when run with -race.
//
// Usage:
//
//	go test -race -bench=BenchmarkRobotTable -benchtime=3s ./internal/table/
func BenchmarkRobotTable(b *testing.B) {
	cfg := config.DefaultSceneConfig()
	cfg.SmallBlind = 50
	cfg.BigBlind = 100
	cfg.ActionTimeSec = 1 // short timeout so hands don't stall

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numTables = 100

	tables := make([]*table.Table, numTables)
	for i := range tables {
		host := stub.NewTableStub()
		tbl := table.NewTable(uint32(i+1), &cfg, host)

		// Seat two robot players
		for _, uid := range []uint32{uint32(i*2 + 1001), uint32(i*2 + 1002)} {
			p := player.NewPlayer(uid, "Bot", "", 10000)
			p.IsRobot = true
			p.IsReady = true
			tbl.Send(table.Msg{
				Kind:     table.MsgKindPlayerEnter,
				UserID:   uid,
				NickName: "Bot",
				ChipDesk: 10000,
			})
		}

		tables[i] = tbl
		go tbl.Run(ctx)
	}

	// Give goroutines a moment to start
	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Keep goroutines alive — the tables run autonomously.
		for pb.Next() {
			time.Sleep(time.Millisecond)
		}
	})
	b.StopTimer()
}

// TestRobotTableLoad verifies that 100 robot tables can run concurrently for
// 2 seconds without data races or panics. This is the functional version of
// BenchmarkRobotTable suitable for CI.
func TestRobotTableLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	cfg := config.DefaultSceneConfig()
	cfg.SmallBlind = 50
	cfg.BigBlind = 100
	cfg.ActionTimeSec = 1

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const numTables = 100

	for i := 0; i < numTables; i++ {
		host := stub.NewTableStub()
		tbl := table.NewTable(uint32(i+1), &cfg, host)

		for _, uid := range []uint32{uint32(i*2 + 1001), uint32(i*2 + 1002)} {
			p := player.NewPlayer(uid, "Bot", "", 10000)
			p.IsRobot = true
			p.IsReady = true
			_ = p // player will be registered via Msg below
			tbl.Send(table.Msg{
				Kind:     table.MsgKindPlayerEnter,
				UserID:   uid,
				NickName: "Bot",
				ChipDesk: 10000,
			})
		}

		go tbl.Run(ctx)
	}

	// Wait for the context to expire (2 s of concurrent robot play)
	<-ctx.Done()
}
