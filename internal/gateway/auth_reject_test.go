package gateway

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/qbgames/qbdz-game/internal/protocol"
	"github.com/qbgames/qbdz-game/internal/tableapp"
)

func TestGatewayRejectsInvalidSignature(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	identity := NewIdentity("alice", "Alice", pub)
	table := tableapp.NewTable(tableapp.DefaultConfig())
	defer table.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewServer(table, []Identity{identity}, "test-server", logger)
	httpServer := httptest.NewServer(gw)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	c := &wsTestClient{conn: conn}
	challengeEnv := c.read(t)
	var challenge protocol.AuthChallengePayload
	if err := challengeEnv.DecodePayload(&challenge); err != nil {
		t.Fatal(err)
	}
	badSignature := base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	if err := sendClientMessage(conn, protocol.TypeAuth, protocol.AuthPayload{
		Method:    "ed25519",
		PlayerID:  "alice",
		KeyID:     KeyID(pub),
		Signature: badSignature,
	}); err != nil {
		t.Fatal(err)
	}
	errorEnv := c.read(t)
	if errorEnv.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", errorEnv.Type)
	}
	var payload protocol.ErrorPayload
	if err := errorEnv.DecodePayload(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != protocol.ErrAuthFailed {
		t.Fatalf("expected AUTH_FAILED, got %s", payload.Code)
	}
}
