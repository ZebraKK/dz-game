package tableapp

import (
	"time"

	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/protocol"
)

const MaxSeats = 9

type Config struct {
	TableID         string
	MinPlayers      int
	SmallBlind      uint64
	BigBlind        uint64
	Ante            uint64
	StartingStack   uint64
	RakePercent     int
	ActionTimeout   time.Duration
	DisconnectGrace time.Duration
	StartCountdown  time.Duration
	ResultDisplay   time.Duration
}

func DefaultConfig() Config {
	return Config{
		TableID:         "main",
		MinPlayers:      2,
		SmallBlind:      10,
		BigBlind:        20,
		Ante:            0,
		StartingStack:   2000,
		RakePercent:     0,
		ActionTimeout:   90 * time.Second,
		DisconnectGrace: 90 * time.Second,
		StartCountdown:  3 * time.Second,
		ResultDisplay:   5 * time.Second,
	}
}

func (c Config) runtimeConfig() protocol.RuntimeConfig {
	return protocol.RuntimeConfig{
		MaxSeats:          MaxSeats,
		SmallBlind:        c.SmallBlind,
		BigBlind:          c.BigBlind,
		Ante:              c.Ante,
		StartingStack:     c.StartingStack,
		RakePercent:       c.RakePercent,
		ActionTimeoutMs:   c.ActionTimeout.Milliseconds(),
		DisconnectGraceMs: c.DisconnectGrace.Milliseconds(),
		StartCountdownMs:  c.StartCountdown.Milliseconds(),
		ResultDisplayMs:   c.ResultDisplay.Milliseconds(),
	}
}

func (c Config) sceneConfig() config.SceneConfig {
	cfg := config.DefaultSceneConfig()
	cfg.SeatCount = MaxSeats
	cfg.SmallBlind = c.SmallBlind
	cfg.BigBlind = c.BigBlind
	cfg.Ante = uint32(c.Ante)
	cfg.ServiceCharge = 0
	cfg.ActionTimeSec = uint32(c.ActionTimeout / time.Second)
	if cfg.ActionTimeSec == 0 {
		cfg.ActionTimeSec = 1
	}
	return cfg
}
