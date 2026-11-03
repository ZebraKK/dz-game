// Package protocol defines the versioned JSON wire contract shared by the CLI
// and the player-facing backend gateway.
package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const Version = 1

const (
	TypeAuthChallenge = "auth.challenge"
	TypeAuth          = "auth"
	TypeAuthOK        = "auth.ok"
	TypeJoin          = "join"
	TypeJoinOK        = "join.ok"
	TypeSnapshot      = "table.snapshot"
	TypeSit           = "sit"
	TypeCancelSit     = "cancelsit"
	TypeSitOut        = "sitout"
	TypeLeave         = "leave"
	TypeSitPending    = "sit.pending"
	TypeSitCancelled  = "sit.cancelled"
	TypeSeatAssigned  = "seat.assigned"
	TypePlayerSitOut  = "player.sitout"
	TypePlayerLeft    = "player.left"
	TypeHandStarted   = "hand.started"
	TypeBlindPosted   = "blind.posted"
	TypeHoleCards     = "hole.cards"
	TypeActionRequest = "action.request"
	TypeAction        = "action"
	TypeActionResult  = "action.result"
	TypeBoardDealt    = "board.dealt"
	TypeShowdown      = "showdown.revealed"
	TypeHandSettled   = "hand.settled"
	TypeStatus        = "status"
	TypePing          = "ping"
	TypePong          = "pong"
	TypeError         = "error"
)

type Envelope struct {
	V            int             `json:"v"`
	Type         string          `json:"type"`
	DeliverySeq  int64           `json:"deliverySeq,omitempty"`
	StateVersion int64           `json:"stateVersion,omitempty"`
	TS           string          `json:"ts,omitempty"`
	RequestID    string          `json:"requestId,omitempty"`
	Payload      json.RawMessage `json:"payload"`
}

func DecodeEnvelope(data []byte) (Envelope, error) {
	var env Envelope
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&env); err != nil {
		return Envelope{}, err
	}
	if env.V != Version {
		return Envelope{}, fmt.Errorf("unsupported protocol version %d", env.V)
	}
	if env.Type == "" {
		return Envelope{}, fmt.Errorf("missing message type")
	}
	if len(env.Payload) == 0 {
		env.Payload = json.RawMessage(`{}`)
	}
	return env, nil
}

func NewServerEnvelope(messageType string, deliverySeq, stateVersion int64, payload any) (Envelope, error) {
	raw, err := marshalPayload(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		V:            Version,
		Type:         messageType,
		DeliverySeq:  deliverySeq,
		StateVersion: stateVersion,
		TS:           time.Now().UTC().Format(time.RFC3339Nano),
		Payload:      raw,
	}, nil
}

func NewClientEnvelope(messageType, requestID string, payload any) (Envelope, error) {
	raw, err := marshalPayload(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		V:         Version,
		Type:      messageType,
		RequestID: requestID,
		Payload:   raw,
	}, nil
}

func (e Envelope) DecodePayload(v any) error {
	return json.Unmarshal(e.Payload, v)
}

func MarshalEnvelope(env Envelope) ([]byte, error) {
	return json.Marshal(env)
}

func marshalPayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		return json.RawMessage(`{}`), nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}
