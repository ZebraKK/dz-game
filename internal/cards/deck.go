package cards

import "math/rand"

// DeckSize is the number of cards in a standard deck (no jokers).
const DeckSize = 52

// Deck represents a standard 52-card deck.
// C++ reference: CCardsSet in CardsSet.h/.cpp
type Deck struct {
	cards [DeckSize]Card
	top   int // index of the next card to deal
}

// NewDeck creates a fresh, ordered 52-card deck (not shuffled).
func NewDeck() Deck {
	var d Deck
	i := 0
	for _, shape := range []Shape{ShapeSpade, ShapeHearts, ShapeClubs, ShapeDiamonds} {
		for _, number := range []Number{
			NumberA, Number2, Number3, Number4, Number5, Number6, Number7,
			Number8, Number9, Number10, NumberJ, NumberQ, NumberK,
		} {
			d.cards[i] = newCard(shape, number)
			i++
		}
	}
	d.top = 0
	return d
}

// Shuffle randomizes the deck using the provided random source.
// Pass a seeded *rand.Rand for reproducible results (useful in tests).
// C++ reference: CCardsSet::Shuffle
func (d *Deck) Shuffle(r *rand.Rand) {
	d.top = 0
	r.Shuffle(DeckSize, func(i, j int) {
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	})
}

// Deal removes and returns the top card from the deck.
// Returns Zero and false if the deck is exhausted.
func (d *Deck) Deal() (Card, bool) {
	if d.top >= DeckSize {
		return Zero, false
	}
	c := d.cards[d.top]
	d.top++
	return c, true
}

// Remaining returns how many cards are left in the deck.
func (d *Deck) Remaining() int {
	return DeckSize - d.top
}

// Reset returns all cards to the deck without reshuffling.
func (d *Deck) Reset() {
	d.top = 0
}
