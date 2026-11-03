package gateway

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
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

type wsTestClient struct {
	id         string
	privateKey ed25519.PrivateKey
	conn       *websocket.Conn
	messages   chan protocol.Envelope
	readErr    chan error
}

func TestGatewayAuthJoinAndTableFlow(t *testing.T) {
	aliceID, aliceKey := newTestIdentity(t, "alice", "Alice")
	bobID, bobKey := newTestIdentity(t, "bob", "Bob")

	cfg := tableapp.DefaultConfig()
	cfg.StartCountdown = 15 * time.Millisecond
	cfg.ActionTimeout = 2 * time.Second
	cfg.ResultDisplay = 50 * time.Millisecond
	table := tableapp.NewTable(cfg)
	defer table.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewServer(table, []Identity{aliceID, bobID}, "test-server", logger)
	httpServer := httptest.NewServer(gw)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	alice := dialAndJoin(t, wsURL, aliceID, aliceKey)
	defer alice.conn.Close(websocket.StatusNormalClosure, "done")
	bob := dialAndJoin(t, wsURL, bobID, bobKey)
	defer bob.conn.Close(websocket.StatusNormalClosure, "done")

	if err := sendClientMessage(alice.conn, protocol.TypeSit, protocol.SitPayload{}); err != nil {
		t.Fatalf("sit alice: %v", err)
	}
	if err := sendClientMessage(bob.conn, protocol.TypeSit, protocol.SitPayload{}); err != nil {
		t.Fatalf("sit bob: %v", err)
	}

	alice.startReader()
	bob.startReader()

	deadline := time.After(3 * time.Second)
	settledAlice := false
	settledBob := false
	actions := 0
	for !settledAlice || !settledBob {
		select {
		case env := <-alice.messages:
			switch env.Type {
			case protocol.TypeActionRequest:
				var req protocol.ActionRequestPayload
				if err := env.DecodePayload(&req); err != nil {
					t.Fatalf("decode alice action request: %v", err)
				}
				sendAllIn(t, alice, req, actions)
				actions++
			case protocol.TypeHandSettled:
				settledAlice = true
			case protocol.TypeError:
				var payload protocol.ErrorPayload
				_ = env.DecodePayload(&payload)
				t.Fatalf("alice received error: %s %s", payload.Code, payload.Message)
			}
		case env := <-bob.messages:
			switch env.Type {
			case protocol.TypeActionRequest:
				var req protocol.ActionRequestPayload
				if err := env.DecodePayload(&req); err != nil {
					t.Fatalf("decode bob action request: %v", err)
				}
				sendAllIn(t, bob, req, actions)
				actions++
			case protocol.TypeHandSettled:
				settledBob = true
			case protocol.TypeError:
				var payload protocol.ErrorPayload
				_ = env.DecodePayload(&payload)
				t.Fatalf("bob received error: %s %s", payload.Code, payload.Message)
			}
		case err := <-alice.readErr:
			t.Fatalf("alice read failed: %v", err)
		case err := <-bob.readErr:
			t.Fatalf("bob read failed: %v", err)
		case <-deadline:
			t.Fatalf("timed out: settledAlice=%v settledBob=%v actions=%d", settledAlice, settledBob, actions)
		}
	}
	if actions < 2 {
		t.Fatalf("expected at least 2 actions, got %d", actions)
	}
}

func newTestIdentity(t *testing.T, playerID, displayName string) (Identity, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return NewIdentity(playerID, displayName, pub), priv
}

func dialAndJoin(t *testing.T, wsURL string, identity Identity, privateKey ed25519.PrivateKey) *wsTestClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c := &wsTestClient{
		id:         identity.PlayerID,
		privateKey: privateKey,
		conn:       conn,
		messages:   make(chan protocol.Envelope, 256),
		readErr:    make(chan error, 1),
	}
	challengeEnv := c.read(t)
	if challengeEnv.Type != protocol.TypeAuthChallenge {
		t.Fatalf("expected auth challenge, got %s", challengeEnv.Type)
	}
	var challenge protocol.AuthChallengePayload
	if err := challengeEnv.DecodePayload(&challenge); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	keyID := KeyID(identity.PublicKey)
	signingText := AuthSigningText(challenge.ServerID, challenge.Challenge, identity.PlayerID, keyID, protocol.Version)
	signature := base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, signingText))
	if err := sendClientMessage(conn, protocol.TypeAuth, protocol.AuthPayload{
		Method:    "ed25519",
		PlayerID:  identity.PlayerID,
		KeyID:     keyID,
		Signature: signature,
	}); err != nil {
		t.Fatalf("send auth: %v", err)
	}
	authOK := c.read(t)
	if authOK.Type != protocol.TypeAuthOK {
		t.Fatalf("expected auth.ok, got %s", authOK.Type)
	}
	if err := sendClientMessage(conn, protocol.TypeJoin, protocol.JoinPayload{TableID: "main"}); err != nil {
		t.Fatalf("send join: %v", err)
	}
	if joinOK := c.read(t); joinOK.Type != protocol.TypeJoinOK {
		t.Fatalf("expected join.ok, got %s", joinOK.Type)
	}
	if snapshot := c.read(t); snapshot.Type != protocol.TypeSnapshot {
		t.Fatalf("expected snapshot, got %s", snapshot.Type)
	}
	return c
}

func (c *wsTestClient) read(t *testing.T) protocol.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	messageType, raw, err := c.conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("unexpected websocket message type %v", messageType)
	}
	env, err := protocol.DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return env
}

func (c *wsTestClient) startReader() {
	go func() {
		for {
			_, raw, err := c.conn.Read(context.Background())
			if err != nil {
				c.readErr <- err
				return
			}
			env, err := protocol.DecodeEnvelope(raw)
			if err != nil {
				c.readErr <- err
				return
			}
			c.messages <- env
		}
	}()
}

func sendClientMessage(conn *websocket.Conn, messageType string, payload any) error {
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	env, err := protocol.NewClientEnvelope(messageType, requestID, payload)
	if err != nil {
		return err
	}
	raw, err := protocol.MarshalEnvelope(env)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, raw)
}

func sendAllIn(t *testing.T, client *wsTestClient, req protocol.ActionRequestPayload, seq int) {
	t.Helper()
	var allIn protocol.LegalAction
	for _, action := range req.LegalActions {
		if action.Type == "all_in" {
			allIn = action
			break
		}
	}
	if allIn.Type == "" {
		t.Fatalf("no all-in action available: %+v", req.LegalActions)
	}
	env, err := protocol.NewClientEnvelope(protocol.TypeAction, fmt.Sprintf("req-action-%d", seq), protocol.ActionPayload{
		ActionID: fmt.Sprintf("action-%d", seq),
		HandID:   req.HandID,
		TurnID:   req.TurnID,
		Action:   protocol.PlayerAction{Type: "all_in", Amount: allIn.Amount},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := protocol.MarshalEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
}
