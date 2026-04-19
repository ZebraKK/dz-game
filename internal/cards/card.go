// Package cards implements the poker card engine: deck, shuffle, deal,
// and Texas Hold'em hand evaluation. All types are pure values with no I/O.
//
// C++ reference: Source/CardAssistant/cardsdef.h, CardsSet.h/.cpp
package cards

// Shape represents the suit of a card.
// Values match the C++ ENM_CARD_SHAPE enum in cardsdef.h.
type Shape int8

const (
	ShapeNone     Shape = 0 // 大小王或无花色
	ShapeSpade    Shape = 1 // 黑桃 ♠
	ShapeHearts   Shape = 2 // 红桃 ♥
	ShapeClubs    Shape = 3 // 梅花 ♣
	ShapeDiamonds Shape = 4 // 方块 ♦
)

func (s Shape) String() string {
	switch s {
	case ShapeSpade:
		return "♠"
	case ShapeHearts:
		return "♥"
	case ShapeClubs:
		return "♣"
	case ShapeDiamonds:
		return "♦"
	}
	return "?"
}

// Number represents the face value of a card.
// Values match the C++ ENM_CARD_NUMBER enum in cardsdef.h.
type Number int8

const (
	NumberNone    Number = 0  // 牌背（无效）
	NumberA       Number = 1  // A
	Number2       Number = 2
	Number3       Number = 3
	Number4       Number = 4
	Number5       Number = 5
	Number6       Number = 6
	Number7       Number = 7
	Number8       Number = 8
	Number9       Number = 9
	Number10      Number = 10
	NumberJ       Number = 11
	NumberQ       Number = 12
	NumberK       Number = 13
	NumberJoker   Number = 14 // 小王（本工程德州不使用）
	NumberJOKER   Number = 15 // 大王（本工程德州不使用）
	NumberInvalid Number = 16 // 无效标记
)

func (n Number) String() string {
	switch n {
	case NumberA:
		return "A"
	case Number10:
		return "10"
	case NumberJ:
		return "J"
	case NumberQ:
		return "Q"
	case NumberK:
		return "K"
	case NumberJoker:
		return "joker"
	case NumberJOKER:
		return "JOKER"
	}
	if n >= Number2 && n <= Number9 {
		return string(rune('0' + n))
	}
	return "?"
}

// Card represents a single playing card.
// Mirrors the C++ CARD struct in cardsdef.h.
type Card struct {
	Shape  Shape
	Number Number
	// Level holds the comparison weight of the card (higher = stronger).
	// For Texas Hold'em, A is the highest (level 14), 2 is the lowest (level 2).
	Level int8
}

// IsValid returns true if the card is a real, playable card.
func (c Card) IsValid() bool {
	return c.Shape >= ShapeSpade && c.Shape <= ShapeDiamonds &&
		c.Number >= NumberA && c.Number <= NumberK
}

// Zero is the zero value of Card — represents an empty/unset card slot.
var Zero = Card{}

// newCard creates a standard 52-deck card assigning Level correctly.
// A has level 14 (highest), K=13, Q=12, J=11, 10=10, ... 2=2.
func newCard(shape Shape, number Number) Card {
	level := int8(number)
	if number == NumberA {
		level = 14 // A is the highest card in Texas Hold'em
	}
	return Card{Shape: shape, Number: number, Level: level}
}
