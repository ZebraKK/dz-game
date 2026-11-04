package gateway

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/qbgames/qbdz-game/internal/protocol"
	"github.com/qbgames/qbdz-game/internal/tableapp"
)

const (
	maxMessageSize = 64 << 10
	authTimeout    = 10 * time.Second
	joinTimeout    = 10 * time.Second
	pingInterval   = 20 * time.Second
	heartbeatLimit = 50 * time.Second
)

type Server struct {
	table          *tableapp.Table
	identities     map[string]Identity
	serverID       string
	logger         *slog.Logger
	allowedOrigins []string

	sessionsMu sync.Mutex
	sessions   map[string]*session
}

func NewServer(table *tableapp.Table, identities []Identity, serverID string, logger *slog.Logger) *Server {
	registry := make(map[string]Identity, len(identities))
	for _, identity := range identities {
		registry[identity.PlayerID] = identity
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		table:      table,
		identities: registry,
		serverID:   serverID,
		logger:     logger,
		sessions:   make(map[string]*session),
	}
}

func (s *Server) SetAllowedOrigins(origins []string) {
	s.allowedOrigins = append([]string(nil), origins...)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ws" {
		http.NotFound(w, r)
		return
	}
	var acceptOptions *websocket.AcceptOptions
	if len(s.allowedOrigins) > 0 {
		acceptOptions = &websocket.AcceptOptions{OriginPatterns: s.allowedOrigins}
	}
	conn, err := websocket.Accept(w, r, acceptOptions)
	if err != nil {
		s.logger.Warn("websocket accept failed", "err", err)
		return
	}
	conn.SetReadLimit(maxMessageSize)
	ctx, cancel := context.WithCancel(context.Background())
	sess := &session{
		server: s,
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
		out:    make(chan outboundMessage, 256),
		done:   make(chan struct{}),
	}
	sess.touch()
	go sess.writeLoop()
	defer func() {
		if sess.playerID != "" {
			s.table.Disconnect(sess.playerID, sess)
			s.removeSession(sess.playerID, sess)
		}
		sess.close(websocket.StatusNormalClosure, "connection closed")
	}()

	challengeRaw := make([]byte, 32)
	if _, err := rand.Read(challengeRaw); err != nil {
		sess.close(websocket.StatusInternalError, "challenge generation failed")
		return
	}
	challenge := base64.RawURLEncoding.EncodeToString(challengeRaw)
	expiresAt := time.Now().UTC().Add(30 * time.Second)
	if !sess.Send(protocol.TypeAuthChallenge, 0, protocol.AuthChallengePayload{
		ServerID:  s.serverID,
		Challenge: challenge,
		ExpiresAt: expiresAt.Format(time.RFC3339Nano),
	}) {
		return
	}

	identity, authErr := s.authenticate(ctx, sess, challenge, expiresAt)
	if authErr != nil {
		sess.sendError(actorProtocolError(protocol.ErrAuthFailed, authErr.Error()), "")
		time.Sleep(30 * time.Millisecond)
		sess.close(4001, "authentication failed")
		return
	}
	sess.playerID = identity.PlayerID
	sess.displayName = identity.DisplayName
	if !sess.Send(protocol.TypeAuthOK, 0, protocol.AuthOKPayload{
		PlayerID:    identity.PlayerID,
		DisplayName: identity.DisplayName,
		SessionID:   randomID(16),
	}) {
		return
	}

	join, err := s.readJoin(ctx, sess)
	if err != nil {
		sess.sendError(actorProtocolError(protocol.ErrMalformedMessage, err.Error()), "")
		time.Sleep(30 * time.Millisecond)
		sess.close(4006, "join failed")
		return
	}
	if join.TableID != "main" {
		sess.sendError(actorProtocolError(protocol.ErrNotJoined, "桌子不存在"), "")
		time.Sleep(30 * time.Millisecond)
		sess.close(4006, "table not found")
		return
	}
	if _, err := s.table.Join(ctx, identity.PlayerID, identity.DisplayName, sess); err != nil {
		sess.sendTableError(err, "")
		time.Sleep(30 * time.Millisecond)
		sess.close(4006, "join failed")
		return
	}
	old := s.replaceSession(identity.PlayerID, sess)
	if old != nil {
		old.sendError(actorProtocolError(protocol.ErrSessionReplaced, "连接已被新的登录替换"), "")
		go func() {
			time.Sleep(30 * time.Millisecond)
			old.close(4003, "session replaced")
		}()
	}
	go sess.heartbeatLoop()

	for {
		env, err := sess.readEnvelope(context.Background())
		if err != nil {
			if websocket.CloseStatus(err) == -1 && !errors.Is(err, context.Canceled) {
				s.logger.Debug("websocket read ended", "playerID", identity.PlayerID, "err", err)
			}
			return
		}
		sess.touch()
		if !sess.allowMessage() {
			sess.sendError(actorProtocolError(protocol.ErrRateLimited, "请求过快"), env.RequestID)
			if sess.rateViolations() >= 10 {
				sess.close(4008, "rate limited")
				return
			}
			continue
		}
		if err := s.handleClientMessage(ctx, sess, env); err != nil {
			sess.sendTableError(err, env.RequestID)
		}
	}
}

func (s *Server) authenticate(ctx context.Context, sess *session, challenge string, expiresAt time.Time) (Identity, error) {
	authCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	env, err := sess.readEnvelope(authCtx)
	if err != nil {
		return Identity{}, fmt.Errorf("read auth: %w", err)
	}
	if env.Type != protocol.TypeAuth {
		return Identity{}, fmt.Errorf("expected auth, got %s", env.Type)
	}
	if time.Now().After(expiresAt) {
		return Identity{}, fmt.Errorf("challenge expired")
	}
	var payload protocol.AuthPayload
	if err := env.DecodePayload(&payload); err != nil {
		return Identity{}, fmt.Errorf("decode auth: %w", err)
	}
	identity, ok := s.identities[payload.PlayerID]
	if !ok {
		return Identity{}, fmt.Errorf("unknown player")
	}
	if payload.Method != "ed25519" {
		return Identity{}, fmt.Errorf("unsupported auth method")
	}
	expectedKeyID := KeyID(identity.PublicKey)
	if payload.KeyID != expectedKeyID {
		return Identity{}, fmt.Errorf("unknown key id")
	}
	signature, err := base64.RawURLEncoding.DecodeString(payload.Signature)
	if err != nil {
		return Identity{}, fmt.Errorf("decode signature: %w", err)
	}
	text := AuthSigningText(s.serverID, challenge, payload.PlayerID, payload.KeyID, protocol.Version)
	if !ed25519.Verify(identity.PublicKey, text, signature) {
		return Identity{}, fmt.Errorf("signature verification failed")
	}
	return identity, nil
}

func (s *Server) readJoin(ctx context.Context, sess *session) (protocol.JoinPayload, error) {
	joinCtx, cancel := context.WithTimeout(ctx, joinTimeout)
	defer cancel()
	env, err := sess.readEnvelope(joinCtx)
	if err != nil {
		return protocol.JoinPayload{}, fmt.Errorf("read join: %w", err)
	}
	if env.Type != protocol.TypeJoin {
		return protocol.JoinPayload{}, fmt.Errorf("expected join, got %s", env.Type)
	}
	var payload protocol.JoinPayload
	if err := env.DecodePayload(&payload); err != nil {
		return protocol.JoinPayload{}, fmt.Errorf("decode join: %w", err)
	}
	return payload, nil
}

func (s *Server) handleClientMessage(ctx context.Context, sess *session, env protocol.Envelope) error {
	switch env.Type {
	case protocol.TypeStatus:
		return s.table.Status(ctx, sess.playerID)
	case protocol.TypeSit:
		var payload protocol.SitPayload
		if err := env.DecodePayload(&payload); err != nil {
			return actorProtocolError(protocol.ErrMalformedMessage, "入座请求格式错误")
		}
		if payload.PreferredSeat != nil && (*payload.PreferredSeat < 1 || *payload.PreferredSeat > 9) {
			return actorProtocolError(protocol.ErrInvalidSeat, "座位号必须在 1 到 9 之间")
		}
		return s.table.Sit(ctx, sess.playerID, payload.PreferredSeat)
	case protocol.TypeCancelSit:
		return s.table.CancelSit(ctx, sess.playerID)
	case protocol.TypeSitOut:
		return s.table.SitOut(ctx, sess.playerID)
	case protocol.TypeLeave:
		return s.table.Leave(ctx, sess.playerID)
	case protocol.TypeAction:
		var payload protocol.ActionPayload
		if err := env.DecodePayload(&payload); err != nil {
			return actorProtocolError(protocol.ErrMalformedMessage, "动作格式错误")
		}
		return s.table.Action(ctx, sess.playerID, payload)
	case protocol.TypePong:
		return nil
	default:
		return actorProtocolError(protocol.ErrMalformedMessage, "未知消息类型")
	}
}

func (s *Server) replaceSession(playerID string, sess *session) *session {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	old := s.sessions[playerID]
	s.sessions[playerID] = sess
	return old
}

func (s *Server) removeSession(playerID string, sess *session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if s.sessions[playerID] == sess {
		delete(s.sessions, playerID)
	}
}

type outboundMessage struct {
	messageType  string
	stateVersion int64
	payload      any
}

type session struct {
	server      *Server
	conn        *websocket.Conn
	ctx         context.Context
	cancel      context.CancelFunc
	out         chan outboundMessage
	done        chan struct{}
	closeOnce   sync.Once
	sequence    atomic.Int64
	lastSeen    atomic.Int64
	playerID    string
	displayName string

	rateMu              sync.Mutex
	rateTokens          float64
	rateLast            time.Time
	rateViolationsCount int
}

func (s *session) Send(messageType string, stateVersion int64, payload any) bool {
	msg := outboundMessage{messageType: messageType, stateVersion: stateVersion, payload: payload}
	select {
	case s.out <- msg:
		return true
	case <-s.done:
		return false
	default:
		go s.close(websocket.StatusTryAgainLater, "send queue full")
		return false
	}
}

func (s *session) touch() {
	s.lastSeen.Store(time.Now().UnixNano())
}

func (s *session) allowMessage() bool {
	const ratePerSecond = 20.0
	const burst = 40.0
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	now := time.Now()
	if s.rateLast.IsZero() {
		s.rateLast = now
		s.rateTokens = burst
	}
	elapsed := now.Sub(s.rateLast).Seconds()
	s.rateTokens = minFloat(burst, s.rateTokens+elapsed*ratePerSecond)
	s.rateLast = now
	if s.rateTokens < 1 {
		s.rateViolationsCount++
		return false
	}
	s.rateTokens--
	return true
}

func (s *session) rateViolations() int {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	return s.rateViolationsCount
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (s *session) writeLoop() {
	for {
		select {
		case <-s.done:
			return
		case msg := <-s.out:
			seq := s.sequence.Add(1)
			env, err := protocol.NewServerEnvelope(msg.messageType, seq, msg.stateVersion, msg.payload)
			if err != nil {
				s.server.logger.Warn("encode server message failed", "type", msg.messageType, "err", err)
				continue
			}
			raw, err := protocol.MarshalEnvelope(env)
			if err != nil {
				s.server.logger.Warn("marshal server message failed", "type", msg.messageType, "err", err)
				continue
			}
			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = s.conn.Write(writeCtx, websocket.MessageText, raw)
			cancel()
			if err != nil {
				s.close(websocket.StatusInternalError, "write failed")
				return
			}
		}
	}
}

func (s *session) readEnvelope(ctx context.Context) (protocol.Envelope, error) {
	messageType, raw, err := s.conn.Read(ctx)
	if err != nil {
		return protocol.Envelope{}, err
	}
	if messageType != websocket.MessageText {
		return protocol.Envelope{}, fmt.Errorf("expected text message")
	}
	return protocol.DecodeEnvelope(raw)
}

func (s *session) heartbeatLoop() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			lastSeen := time.Unix(0, s.lastSeen.Load())
			if time.Since(lastSeen) > heartbeatLimit {
				s.sendError(actorProtocolError(protocol.ErrHeartbeatTimeout, "连接心跳超时"), "")
				time.Sleep(20 * time.Millisecond)
				s.close(4005, "heartbeat timeout")
				return
			}
			nonce := randomID(12)
			if !s.Send(protocol.TypePing, 0, protocol.PingPayload{Nonce: nonce}) {
				return
			}
		}
	}
}

func (s *session) sendError(err *tableapp.Error, requestID string) {
	if err == nil {
		return
	}
	s.Send(protocol.TypeError, 0, protocol.ErrorPayload{
		Code:      err.Code,
		Message:   err.Message,
		Retryable: err.Retryable,
		RequestID: requestID,
		Details:   err.Details,
	})
}

func (s *session) sendTableError(err error, requestID string) {
	if err == nil {
		return
	}
	var tableErr *tableapp.Error
	if errors.As(err, &tableErr) {
		s.sendError(tableErr, requestID)
		return
	}
	s.sendError(actorProtocolError(protocol.ErrInternal, "服务端内部错误"), requestID)
	s.server.logger.Error("gateway request failed", "err", err)
}

func (s *session) close(code websocket.StatusCode, reason string) {
	s.closeOnce.Do(func() {
		close(s.done)
		s.cancel()
		_ = s.conn.Close(code, reason)
	})
}

func actorProtocolError(code, message string) *tableapp.Error {
	return &tableapp.Error{Code: code, Message: message}
}

func randomID(size int) string {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

var _ tableapp.ClientSink = (*session)(nil)
