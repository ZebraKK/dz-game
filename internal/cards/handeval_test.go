package cards

import (
	"math/rand"
	"testing"
)

// makeCard is a test helper to create cards quickly.
func makeCard(shape Shape, number Number) Card {
	return newCard(shape, number)
}

// makeHand is a test helper: parses "As Kh Qd Jc 10s" style strings.
// For brevity in tests we pass Card values directly.
func hand(cards ...Card) []Card { return cards }

var (
	// Shorthand card constructors for tests
	As  = makeCard(ShapeSpade, NumberA)
	Ks  = makeCard(ShapeSpade, NumberK)
	Qs  = makeCard(ShapeSpade, NumberQ)
	Js  = makeCard(ShapeSpade, NumberJ)
	T10s = makeCard(ShapeSpade, Number10)
	N9s = makeCard(ShapeSpade, Number9)
	N8s = makeCard(ShapeSpade, Number8)
	N7s = makeCard(ShapeSpade, Number7)
	N6s = makeCard(ShapeSpade, Number6)
	N5s = makeCard(ShapeSpade, Number5)
	N4s = makeCard(ShapeSpade, Number4)
	N3s = makeCard(ShapeSpade, Number3)
	N2s = makeCard(ShapeSpade, Number2)

	Ah  = makeCard(ShapeHearts, NumberA)
	Kh  = makeCard(ShapeHearts, NumberK)
	Qh  = makeCard(ShapeHearts, NumberQ)
	Jh  = makeCard(ShapeHearts, NumberJ)
	T10h = makeCard(ShapeHearts, Number10)
	N9h = makeCard(ShapeHearts, Number9)
	N8h = makeCard(ShapeHearts, Number8)
	N5h = makeCard(ShapeHearts, Number5)
	N4h = makeCard(ShapeHearts, Number4)
	N3h = makeCard(ShapeHearts, Number3)
	N2h = makeCard(ShapeHearts, Number2)

	Ad  = makeCard(ShapeDiamonds, NumberA)
	Kd  = makeCard(ShapeDiamonds, NumberK)
	Qd  = makeCard(ShapeDiamonds, NumberQ)
	Jd  = makeCard(ShapeDiamonds, NumberJ)
	T10d = makeCard(ShapeDiamonds, Number10)
	N9d = makeCard(ShapeDiamonds, Number9)
	N7d = makeCard(ShapeDiamonds, Number7)
	N5d = makeCard(ShapeDiamonds, Number5)
	N4d = makeCard(ShapeDiamonds, Number4)
	N3d = makeCard(ShapeDiamonds, Number3)
	N2d = makeCard(ShapeDiamonds, Number2)

	Ac  = makeCard(ShapeClubs, NumberA)
	Kc  = makeCard(ShapeClubs, NumberK)
	Qc  = makeCard(ShapeClubs, NumberQ)
	Jc  = makeCard(ShapeClubs, NumberJ)
	T10c = makeCard(ShapeClubs, Number10)
	N9c = makeCard(ShapeClubs, Number9)
	N8c = makeCard(ShapeClubs, Number8)
	N7h = makeCard(ShapeHearts, Number7)
	N5c = makeCard(ShapeClubs, Number5)
	N4c = makeCard(ShapeClubs, Number4)
	N3c = makeCard(ShapeClubs, Number3)
	N2c = makeCard(ShapeClubs, Number2)
)

func TestHandTypes(t *testing.T) {
	tests := []struct {
		name     string
		cards    []Card
		wantType HandType
	}{
		{
			name:     "皇家同花顺 Royal Flush",
			cards:    hand(As, Ks, Qs, Js, T10s),
			wantType: HandRoyalFlush,
		},
		{
			name:     "同花顺 Straight Flush",
			cards:    hand(N9s, N8s, N7s, N6s, N5s),
			wantType: HandStraightFlush,
		},
		{
			name:     "A-low 同花顺 (A2345 spades)",
			cards:    hand(As, N2s, N3s, N4s, N5s),
			wantType: HandStraightFlush,
		},
		{
			name:     "四张 Four of a Kind",
			cards:    hand(As, Ah, Ad, Ac, Ks),
			wantType: HandFourOfAKind,
		},
		{
			name:     "葫芦 Full House",
			cards:    hand(As, Ah, Ad, Ks, Kh),
			wantType: HandFullHouse,
		},
		{
			name:     "同花 Flush",
			cards:    hand(As, Ks, Qs, Js, N9s),
			wantType: HandFlush,
		},
		{
			name:     "顺子 Straight (A-high)",
			cards:    hand(As, Kh, Qd, Jc, T10s),
			wantType: HandStraight,
		},
		{
			name:     "顺子 Straight (5-high wheel A2345)",
			cards:    hand(As, N2h, N3d, N4c, N5s),
			wantType: HandStraight,
		},
		{
			name:     "三张 Three of a Kind",
			cards:    hand(As, Ah, Ad, Ks, Qd),
			wantType: HandThreeOfAKind,
		},
		{
			name:     "两对 Two Pair",
			cards:    hand(As, Ah, Ks, Kh, Qd),
			wantType: HandTwoPair,
		},
		{
			name:     "一对 One Pair",
			cards:    hand(As, Ah, Kd, Qc, Jh),
			wantType: HandPair,
		},
		{
			name:     "高牌 High Card",
			cards:    hand(As, Kh, Qd, Jc, N9s),
			wantType: HandHighCard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := BestHand(tt.cards)
			if h.Type != tt.wantType {
				t.Errorf("BestHand(%v) = %v, want %v", tt.name, h.Type, tt.wantType)
			}
		})
	}
}

func TestBestHand7Cards(t *testing.T) {
	// 7-card scenarios (2 hole + 5 community)
	tests := []struct {
		name     string
		cards    []Card
		wantType HandType
	}{
		{
			name:     "皇家同花顺 from 7 cards",
			cards:    hand(As, Ks, Qs, Js, T10s, N2h, N3d),
			wantType: HandRoyalFlush,
		},
		{
			name:     "四张 from 7 cards",
			cards:    hand(As, Ah, Ad, Ac, Ks, Kh, Qd),
			wantType: HandFourOfAKind,
		},
		{
			name:     "葫芦 选最优 from 7 cards",
			cards:    hand(As, Ah, Ad, Ks, Kh, Qd, Qc),
			wantType: HandFullHouse, // AAA KK beats AA KKK? no: AAA+KK葫芦
		},
		{
			name:     "同花 from 7 cards",
			cards:    hand(As, Ks, Qs, Js, N9s, N2h, N3d),
			wantType: HandFlush,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := BestHand(tt.cards)
			if h.Type != tt.wantType {
				t.Errorf("BestHand(%v) = %v, want %v", tt.name, h.Type, tt.wantType)
			}
		})
	}
}

func TestCompareHands(t *testing.T) {
	tests := []struct {
		name string
		a, b []Card
		want int // >0 a wins, <0 b wins, 0 tie
	}{
		{
			name: "皇家同花顺 > 同花顺",
			a:    hand(As, Ks, Qs, Js, T10s),
			b:    hand(N9s, N8s, N7s, N6s, N5s),
			want: 1,
		},
		{
			name: "同花顺 > 四张",
			a:    hand(N9s, N8s, N7s, N6s, N5s),
			b:    hand(As, Ah, Ad, Ac, Ks),
			want: 1,
		},
		{
			name: "四张 A > 四张 K",
			a:    hand(As, Ah, Ad, Ac, N2h),
			b:    hand(Ks, Kh, Kd, Kc, As),
			want: 1,
		},
		{
			name: "同花 K-high > 同花 Q-high",
			a:    hand(Ks, Qs, Js, T10s, N8s),
			b:    hand(Qh, Jh, T10h, N9h, N7h),
			want: 1,
		},
		{
			name: "同一手牌平局",
			a:    hand(As, Kh, Qd, Jc, N9s),
			b:    hand(Ah, Kd, Qc, Js, N9h),
			want: 0,
		},
		{
			name: "两对 AA-KK > 两对 AA-QQ",
			a:    hand(As, Ah, Ks, Kh, Jd),
			b:    hand(Ad, Ac, Qs, Qh, Jc),
			want: 1,
		},
		{
			name: "A-high 顺子 > K-high 顺子",
			a:    hand(As, Kh, Qd, Jc, T10s),
			b:    hand(Kd, Qc, Jh, T10d, N9s),
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ha := BestHand(tt.a)
			hb := BestHand(tt.b)
			got := CompareHands(ha, hb)
			if (got > 0) != (tt.want > 0) || (got < 0) != (tt.want < 0) || (got == 0) != (tt.want == 0) {
				t.Errorf("CompareHands(%v): got %d, want sign %d\n  a=%v(%v)\n  b=%v(%v)",
					tt.name, got, tt.want, ha.Type, ha.Ranks, hb.Type, hb.Ranks)
			}
		})
	}
}

func TestDeckShuffle(t *testing.T) {
	d := NewDeck()
	r := rand.New(rand.NewSource(42))
	d.Shuffle(r)

	seen := make(map[[2]int8]bool)
	for i := 0; i < DeckSize; i++ {
		c, ok := d.Deal()
		if !ok {
			t.Fatalf("deck exhausted at card %d", i)
		}
		key := [2]int8{int8(c.Shape), int8(c.Number)}
		if seen[key] {
			t.Errorf("duplicate card: shape=%v number=%v", c.Shape, c.Number)
		}
		seen[key] = true
	}
	if len(seen) != DeckSize {
		t.Errorf("expected %d unique cards, got %d", DeckSize, len(seen))
	}

	// Deck should be exhausted
	_, ok := d.Deal()
	if ok {
		t.Error("expected deck exhausted after 52 deals")
	}
}

func TestDeckRemainingAndReset(t *testing.T) {
	d := NewDeck()
	r := rand.New(rand.NewSource(1))
	d.Shuffle(r)

	if d.Remaining() != 52 {
		t.Errorf("expected 52, got %d", d.Remaining())
	}
	d.Deal()
	d.Deal()
	if d.Remaining() != 50 {
		t.Errorf("expected 50, got %d", d.Remaining())
	}
	d.Reset()
	if d.Remaining() != 52 {
		t.Errorf("after reset expected 52, got %d", d.Remaining())
	}
}
