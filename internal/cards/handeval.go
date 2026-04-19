package cards

import "sort"

// HandType represents a Texas Hold'em hand ranking.
// Values match the C++ ENM_HAND_TYPE enum in SoGameCommon.h exactly.
type HandType int

const (
	HandNull         HandType = 0  // 初始化或出错
	HandHighCard     HandType = 1  // 高牌
	HandPair         HandType = 2  // 一对
	HandTwoPair      HandType = 3  // 两对
	HandThreeOfAKind HandType = 4  // 三张
	HandStraight     HandType = 5  // 顺子
	HandFlush        HandType = 6  // 同花
	HandFullHouse    HandType = 7  // 葫芦
	HandFourOfAKind  HandType = 8  // 四张
	HandStraightFlush HandType = 9 // 同花顺
	HandRoyalFlush   HandType = 10 // 皇家同花顺
)

func (h HandType) String() string {
	names := []string{
		"Null", "高牌", "一对", "两对", "三张",
		"顺子", "同花", "葫芦", "四张", "同花顺", "皇家同花顺",
	}
	if int(h) < len(names) {
		return names[h]
	}
	return "Unknown"
}

// Hand represents the best 5-card hand evaluated from 5–7 cards.
type Hand struct {
	Type  HandType
	Cards [5]Card // the 5 cards that make the hand, sorted best-first
	// Ranks holds the key card levels for tie-breaking, ordered by importance.
	// e.g. for TwoPair: [higherPairLevel, lowerPairLevel, kickerLevel]
	Ranks [5]int8
}

// BestHand evaluates all C(n,5) combinations of cards (n=5..7) and
// returns the highest-ranking 5-card hand.
// C++ reference: CCardsManager::GetBestCards / hand evaluation in SoGameLogic
func BestHand(cards []Card) Hand {
	if len(cards) < 5 {
		return Hand{}
	}
	var best Hand
	combos := combinations(cards, 5)
	for _, combo := range combos {
		h := evaluate5(combo)
		if compareHands(h, best) > 0 {
			best = h
		}
	}
	return best
}

// CompareHands returns >0 if a beats b, <0 if b beats a, 0 if equal.
func CompareHands(a, b Hand) int {
	return compareHands(a, b)
}

func compareHands(a, b Hand) int {
	if a.Type != b.Type {
		if a.Type > b.Type {
			return 1
		}
		return -1
	}
	// Same type: compare tie-breaker ranks lexicographically
	for i := 0; i < 5; i++ {
		if a.Ranks[i] != b.Ranks[i] {
			if a.Ranks[i] > b.Ranks[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// evaluate5 scores exactly 5 cards.
func evaluate5(c [5]Card) Hand {
	// Sort descending by level for easier pattern matching.
	sorted := c
	sort.Slice(sorted[:], func(i, j int) bool {
		return sorted[i].Level > sorted[j].Level
	})

	flush := isFlush(sorted)
	straight, highLevel := isStraight(sorted)

	if flush && straight {
		if highLevel == 14 { // A-high straight flush = royal flush
			return Hand{
				Type:  HandRoyalFlush,
				Cards: sorted,
				Ranks: [5]int8{highLevel},
			}
		}
		return Hand{
			Type:  HandStraightFlush,
			Cards: sorted,
			Ranks: [5]int8{highLevel},
		}
	}

	// Count occurrences of each level
	counts := countLevels(sorted)

	// Four of a kind
	if quad, kicker, ok := findNOfAKind(counts, sorted, 4); ok {
		return Hand{
			Type:  HandFourOfAKind,
			Cards: sorted,
			Ranks: [5]int8{quad, kicker},
		}
	}

	// Full house
	if trip, pair, ok := findFullHouse(counts, sorted); ok {
		return Hand{
			Type:  HandFullHouse,
			Cards: sorted,
			Ranks: [5]int8{trip, pair},
		}
	}

	if flush {
		var ranks [5]int8
		for i, card := range sorted {
			ranks[i] = card.Level
		}
		return Hand{Type: HandFlush, Cards: sorted, Ranks: ranks}
	}

	if straight {
		return Hand{
			Type:  HandStraight,
			Cards: sorted,
			Ranks: [5]int8{highLevel},
		}
	}

	// Three of a kind
	if trip, kickers, ok := findThreeOfAKind(counts, sorted); ok {
		return Hand{
			Type:  HandThreeOfAKind,
			Cards: sorted,
			Ranks: [5]int8{trip, kickers[0], kickers[1]},
		}
	}

	// Two pair
	if hi, lo, kicker, ok := findTwoPair(counts, sorted); ok {
		return Hand{
			Type:  HandTwoPair,
			Cards: sorted,
			Ranks: [5]int8{hi, lo, kicker},
		}
	}

	// One pair
	if pair, kickers, ok := findOnePair(counts, sorted); ok {
		return Hand{
			Type:  HandPair,
			Cards: sorted,
			Ranks: [5]int8{pair, kickers[0], kickers[1], kickers[2]},
		}
	}

	// High card
	var ranks [5]int8
	for i, card := range sorted {
		ranks[i] = card.Level
	}
	return Hand{Type: HandHighCard, Cards: sorted, Ranks: ranks}
}

// isFlush returns true if all 5 cards share the same suit.
func isFlush(c [5]Card) bool {
	for i := 1; i < 5; i++ {
		if c[i].Shape != c[0].Shape {
			return false
		}
	}
	return true
}

// isStraight checks for a 5-card straight (including A-low: A2345).
// Returns (true, highCardLevel) on success.
func isStraight(c [5]Card) (bool, int8) {
	// Normal straight: levels must be consecutive descending
	levels := [5]int8{c[0].Level, c[1].Level, c[2].Level, c[3].Level, c[4].Level}
	normal := true
	for i := 1; i < 5; i++ {
		if levels[i] != levels[i-1]-1 {
			normal = false
			break
		}
	}
	if normal {
		return true, levels[0]
	}

	// Wheel: A-2-3-4-5 (A plays as 1)
	// After descending sort, levels would be [14,5,4,3,2]
	if levels[0] == 14 && levels[1] == 5 && levels[2] == 4 && levels[3] == 3 && levels[4] == 2 {
		return true, 5 // 5-high straight
	}
	return false, 0
}

// levelCount is a map from card level to count and representative card.
type levelCount struct {
	level int8
	count int
}

func countLevels(c [5]Card) []levelCount {
	m := make(map[int8]int, 5)
	for _, card := range c {
		m[card.Level]++
	}
	result := make([]levelCount, 0, len(m))
	for l, cnt := range m {
		result = append(result, levelCount{l, cnt})
	}
	// Sort: higher count first, then higher level
	sort.Slice(result, func(i, j int) bool {
		if result[i].count != result[j].count {
			return result[i].count > result[j].count
		}
		return result[i].level > result[j].level
	})
	return result
}

func findNOfAKind(counts []levelCount, _ [5]Card, n int) (quad, kicker int8, ok bool) {
	for _, lc := range counts {
		if lc.count == n {
			quad = lc.level
			ok = true
		} else if ok {
			kicker = lc.level
		}
	}
	return
}

func findFullHouse(counts []levelCount, _ [5]Card) (trip, pair int8, ok bool) {
	if len(counts) == 2 && counts[0].count == 3 && counts[1].count == 2 {
		return counts[0].level, counts[1].level, true
	}
	return
}

func findThreeOfAKind(counts []levelCount, _ [5]Card) (trip int8, kickers [2]int8, ok bool) {
	if counts[0].count == 3 && len(counts) == 3 {
		trip = counts[0].level
		kickers[0] = counts[1].level
		kickers[1] = counts[2].level
		ok = true
	}
	return
}

func findTwoPair(counts []levelCount, _ [5]Card) (hi, lo, kicker int8, ok bool) {
	if len(counts) == 3 && counts[0].count == 2 && counts[1].count == 2 {
		hi = counts[0].level
		lo = counts[1].level
		kicker = counts[2].level
		ok = true
	}
	return
}

func findOnePair(counts []levelCount, _ [5]Card) (pair int8, kickers [3]int8, ok bool) {
	if counts[0].count == 2 && len(counts) == 4 {
		pair = counts[0].level
		kickers[0] = counts[1].level
		kickers[1] = counts[2].level
		kickers[2] = counts[3].level
		ok = true
	}
	return
}

// combinations returns all C(n,5) 5-card subsets from n cards.
func combinations(cards []Card, k int) [][5]Card {
	n := len(cards)
	var result [][5]Card
	indices := make([]int, k)
	for i := range indices {
		indices[i] = i
	}
	for {
		var combo [5]Card
		for i, idx := range indices {
			combo[i] = cards[idx]
		}
		result = append(result, combo)

		// Find rightmost index that can be incremented
		i := k - 1
		for i >= 0 && indices[i] == i+n-k {
			i--
		}
		if i < 0 {
			break
		}
		indices[i]++
		for j := i + 1; j < k; j++ {
			indices[j] = indices[j-1] + 1
		}
	}
	return result
}
