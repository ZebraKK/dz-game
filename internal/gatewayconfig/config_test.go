package gatewayconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaultsAndOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	content := `addr: 127.0.0.1:19090
serverId: test-server
identityFile: ./identities.json
allowedOrigins:
  - https://poker.example.com
table:
  smallBlind: 25
  bigBlind: 50
  actionTimeoutMs: 15000
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:19090" || cfg.ServerID != "test-server" {
		t.Fatalf("unexpected gateway config: %+v", cfg)
	}
	if cfg.Table.SmallBlind != 25 || cfg.Table.BigBlind != 50 || cfg.Table.ActionTimeoutMs != 15000 {
		t.Fatalf("unexpected table overrides: %+v", cfg.Table)
	}
	if cfg.Table.StartingStack != 2000 || cfg.Table.StartCountdownMs != 3000 {
		t.Fatalf("defaults were not preserved: %+v", cfg.Table)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://poker.example.com" {
		t.Fatalf("unexpected origins: %v", cfg.AllowedOrigins)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(path, []byte("unknownField: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestValidateRejectsInvalidTableConfig(t *testing.T) {
	cfg := Default()
	cfg.Table.BigBlind = cfg.Table.SmallBlind
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid blinds to be rejected")
	}
}

func TestNewTableConfig(t *testing.T) {
	cfg := Default()
	cfg.Table.SmallBlind = 25
	cfg.Table.BigBlind = 50
	cfg.Table.ActionTimeoutMs = 12000
	got := cfg.NewTableConfig()
	if got.SmallBlind != 25 || got.BigBlind != 50 {
		t.Fatalf("unexpected blinds: %+v", got)
	}
	if got.ActionTimeout != 12*time.Second {
		t.Fatalf("unexpected action timeout: %s", got.ActionTimeout)
	}
}

func TestLoadTLSConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	content := `tlsCertFile: /etc/poker/fullchain.pem
tlsKeyFile: /etc/poker/privkey.pem
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TLSEnabled() || cfg.TLSCertFile != "/etc/poker/fullchain.pem" || cfg.TLSKeyFile != "/etc/poker/privkey.pem" {
		t.Fatalf("unexpected TLS config: %+v", cfg)
	}
}

func TestValidateRejectsPartialTLSConfig(t *testing.T) {
	cfg := Default()
	cfg.TLSCertFile = "/etc/poker/fullchain.pem"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected partial TLS config to be rejected")
	}
}
