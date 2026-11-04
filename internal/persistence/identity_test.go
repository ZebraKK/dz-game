package persistence

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIdentities(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "identities.json")
	content := `{"identities":[{"playerId":"alice","displayName":"Alice","publicKey":"` + base64.RawURLEncoding.EncodeToString(pub) + `"}]}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	identities, err := LoadIdentities(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(identities) != 1 || identities[0].PlayerID != "alice" || !identities[0].PublicKey.Equal(pub) {
		t.Fatalf("unexpected identities: %+v", identities)
	}
}
