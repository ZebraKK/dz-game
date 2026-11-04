package gateway

import "testing"

func TestSessionRateLimitBurst(t *testing.T) {
	s := &session{}
	for i := 0; i < 40; i++ {
		if !s.allowMessage() {
			t.Fatalf("burst message %d was rejected", i+1)
		}
	}
	if s.allowMessage() {
		t.Fatal("expected message above burst to be rate limited")
	}
	if s.rateViolations() != 1 {
		t.Fatalf("expected one violation, got %d", s.rateViolations())
	}
}
