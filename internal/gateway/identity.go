package gateway

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

type Identity struct {
	PlayerID    string
	DisplayName string
	PublicKey   ed25519.PublicKey
}

func NewIdentity(playerID, displayName string, publicKey ed25519.PublicKey) Identity {
	return Identity{PlayerID: playerID, DisplayName: displayName, PublicKey: append(ed25519.PublicKey(nil), publicKey...)}
}

func KeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return "ed25519:" + strings.ToUpper(hex.EncodeToString(sum[:4]))
}

func AuthSigningText(serverID, challenge, playerID, keyID string, protocolVersion int) []byte {
	return []byte(fmt.Sprintf("poker-auth-v1\n%s\n%s\n%s\n%s\n%d\n", serverID, challenge, playerID, keyID, protocolVersion))
}

func ParsePublicKey(encoded string) (ed25519.PublicKey, error) {
	candidates := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	var lastErr error
	for _, encoding := range candidates {
		raw, err := encoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil {
			lastErr = err
			continue
		}
		if len(raw) != ed25519.PublicKeySize {
			lastErr = fmt.Errorf("public key must be %d bytes, got %d", ed25519.PublicKeySize, len(raw))
			continue
		}
		return ed25519.PublicKey(raw), nil
	}
	return nil, fmt.Errorf("decode public key: %w", lastErr)
}

func EncodePublicKey(publicKey ed25519.PublicKey) string {
	return base64.RawURLEncoding.EncodeToString(publicKey)
}
