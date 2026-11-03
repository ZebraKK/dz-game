package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestIdentityKeyIDAndPublicKeyRoundTrip(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded := EncodePublicKey(pub)
	decoded, err := ParsePublicKey(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Equal(decoded) {
		t.Fatal("public key round-trip mismatch")
	}
	if got := KeyID(pub); len(got) != len("ed25519:")+8 {
		t.Fatalf("unexpected key ID: %s", got)
	}
}
