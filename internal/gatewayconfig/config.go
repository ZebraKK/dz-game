package gatewayconfig

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/qbgames/qbdz-game/internal/tableapp"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Addr           string      `yaml:"addr"`
	ServerID       string      `yaml:"serverId"`
	IdentityFile   string      `yaml:"identityFile"`
	HistoryFile    string      `yaml:"historyFile"`
	TLSCertFile    string      `yaml:"tlsCertFile"`
	TLSKeyFile     string      `yaml:"tlsKeyFile"`
	AllowedOrigins []string    `yaml:"allowedOrigins"`
	Table          TableConfig `yaml:"table"`
}

type TableConfig struct {
	SmallBlind        uint64 `yaml:"smallBlind"`
	BigBlind          uint64 `yaml:"bigBlind"`
	StartingStack     uint64 `yaml:"startingStack"`
	ActionTimeoutMs   int64  `yaml:"actionTimeoutMs"`
	DisconnectGraceMs int64  `yaml:"disconnectGraceMs"`
	StartCountdownMs  int64  `yaml:"startCountdownMs"`
	ResultDisplayMs   int64  `yaml:"resultDisplayMs"`
}

func Default() Config {
	return Config{
		Addr:     ":8080",
		ServerID: "poker-local-1",
		Table: TableConfig{
			SmallBlind:        10,
			BigBlind:          20,
			StartingStack:     2000,
			ActionTimeoutMs:   90000,
			DisconnectGraceMs: 90000,
			StartCountdownMs:  3000,
			ResultDisplayMs:   5000,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read gateway config %q: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse gateway config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate gateway config %q: %w", path, err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("addr must not be empty")
	}
	if strings.TrimSpace(c.ServerID) == "" {
		return fmt.Errorf("serverId must not be empty")
	}
	if (strings.TrimSpace(c.TLSCertFile) == "") != (strings.TrimSpace(c.TLSKeyFile) == "") {
		return fmt.Errorf("tlsCertFile and tlsKeyFile must be set together")
	}
	for i, origin := range c.AllowedOrigins {
		if strings.TrimSpace(origin) == "" {
			return fmt.Errorf("allowedOrigins[%d] must not be empty", i)
		}
	}
	if c.Table.SmallBlind == 0 {
		return fmt.Errorf("table.smallBlind must be greater than zero")
	}
	if c.Table.BigBlind <= c.Table.SmallBlind {
		return fmt.Errorf("table.bigBlind must be greater than table.smallBlind")
	}
	if c.Table.StartingStack < c.Table.BigBlind {
		return fmt.Errorf("table.startingStack must be at least table.bigBlind")
	}
	if c.Table.ActionTimeoutMs <= 0 {
		return fmt.Errorf("table.actionTimeoutMs must be greater than zero")
	}
	if c.Table.DisconnectGraceMs <= 0 {
		return fmt.Errorf("table.disconnectGraceMs must be greater than zero")
	}
	if c.Table.StartCountdownMs < 0 {
		return fmt.Errorf("table.startCountdownMs must not be negative")
	}
	if c.Table.ResultDisplayMs <= 0 {
		return fmt.Errorf("table.resultDisplayMs must be greater than zero")
	}
	return nil
}

func (c Config) NewTableConfig() tableapp.Config {
	cfg := tableapp.DefaultConfig()
	cfg.SmallBlind = c.Table.SmallBlind
	cfg.BigBlind = c.Table.BigBlind
	cfg.StartingStack = c.Table.StartingStack
	cfg.ActionTimeout = time.Duration(c.Table.ActionTimeoutMs) * time.Millisecond
	cfg.DisconnectGrace = time.Duration(c.Table.DisconnectGraceMs) * time.Millisecond
	cfg.StartCountdown = time.Duration(c.Table.StartCountdownMs) * time.Millisecond
	cfg.ResultDisplay = time.Duration(c.Table.ResultDisplayMs) * time.Millisecond
	return cfg
}

func (c Config) TLSEnabled() bool {
	return strings.TrimSpace(c.TLSCertFile) != "" && strings.TrimSpace(c.TLSKeyFile) != ""
}
