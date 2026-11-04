package persistence

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/qbgames/qbdz-game/internal/gateway"
)

type IdentityRecord struct {
	PlayerID    string `json:"playerId"`
	DisplayName string `json:"displayName"`
	PublicKey   string `json:"publicKey"`
}

type IdentityFile struct {
	Identities []IdentityRecord `json:"identities"`
}

func LoadIdentities(path string) ([]gateway.Identity, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read identity file: %w", err)
	}
	var file IdentityFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("decode identity file: %w", err)
	}
	identities := make([]gateway.Identity, 0, len(file.Identities))
	seen := make(map[string]bool)
	for _, record := range file.Identities {
		if record.PlayerID == "" || record.DisplayName == "" || record.PublicKey == "" {
			return nil, fmt.Errorf("identity file contains an incomplete record")
		}
		if seen[record.PlayerID] {
			return nil, fmt.Errorf("duplicate playerId %q", record.PlayerID)
		}
		publicKey, err := base64.RawURLEncoding.DecodeString(record.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("decode public key for %s: %w", record.PlayerID, err)
		}
		seen[record.PlayerID] = true
		identities = append(identities, gateway.NewIdentity(record.PlayerID, record.DisplayName, publicKey))
	}
	return identities, nil
}
