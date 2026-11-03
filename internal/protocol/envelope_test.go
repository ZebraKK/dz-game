package protocol

import (
	"encoding/json"
	"testing"
)

func TestDecodeEnvelopeIgnoresUnknownFields(t *testing.T) {
	raw := []byte(`{"v":1,"type":"ping","payload":{},"futureField":true}`)
	env, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if env.Type != TypePing {
		t.Fatalf("unexpected type: %s", env.Type)
	}
}

func TestServerEnvelopeRoundTrip(t *testing.T) {
	env, err := NewServerEnvelope(TypePong, 7, 12, PongPayload{Nonce: "n1"})
	if err != nil {
		t.Fatalf("new envelope failed: %v", err)
	}
	raw, err := MarshalEnvelope(env)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	decoded, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	var payload PongPayload
	if err := decoded.DecodePayload(&payload); err != nil {
		t.Fatalf("payload decode failed: %v", err)
	}
	if payload.Nonce != "n1" || decoded.DeliverySeq != 7 || decoded.StateVersion != 12 {
		t.Fatalf("unexpected decoded envelope: %+v payload=%+v", decoded, payload)
	}
}

func TestNewClientEnvelope(t *testing.T) {
	env, err := NewClientEnvelope(TypeStatus, "req-1", StatusPayload{})
	if err != nil {
		t.Fatalf("new envelope failed: %v", err)
	}
	if env.V != Version || env.RequestID != "req-1" || !json.Valid(env.Payload) {
		t.Fatalf("unexpected client envelope: %+v", env)
	}
}
