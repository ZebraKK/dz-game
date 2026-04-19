package texas

// MaxSeats is the maximum number of seats at a Texas table.
// Matches C++ MAX_SEAT_COUNT = 9.
const MaxSeats = 9

// MaxPools is the maximum number of pots (main + side pots).
// Matches C++ NUM_POOL = 9.
const MaxPools = 9

// Pool represents one pot (main or side) and the set of seats eligible to win it.
type Pool struct {
	Amount uint64
	// Eligible[i] is true if seat i is eligible to win this pool.
	Eligible [MaxSeats]bool
}

// ComputePots calculates the main pot and any side pots created by all-in players.
//
// Parameters:
//   - chipsIn: total chips each seat has put in during the current hand.
//     A seat with 0 is not in the hand.
//   - allIn: true if the seat went all-in (creates a side pot boundary).
//   - active: true if the seat is still in the hand (not folded).
//
// Returns a slice of Pool values ordered from smallest boundary to largest.
// This is the critical correctness function — tested exhaustively.
//
// C++ reference: CSoGameLogic::GetPoolsInfo in SoGameLogic.cpp
func ComputePots(chipsIn [MaxSeats]uint64, allIn [MaxSeats]bool, active [MaxSeats]bool) []Pool {
	return computePotsClassic(chipsIn, allIn, active)
}

// computePotsClassic is the reference implementation.
// It repeatedly finds the lowest all-in amount, creates a pot from that level,
// then subtracts and repeats. This is O(n²) but n≤9 so it's fine.
func computePotsClassic(chipsIn [MaxSeats]uint64, allIn [MaxSeats]bool, active [MaxSeats]bool) []Pool {
	remaining := chipsIn
	var pools []Pool

	for {
		// Find the minimum non-zero contribution among all-in players.
		minAllIn := uint64(0)
		for i := 0; i < MaxSeats; i++ {
			if remaining[i] > 0 && allIn[i] {
				if minAllIn == 0 || remaining[i] < minAllIn {
					minAllIn = remaining[i]
				}
			}
		}
		if minAllIn == 0 {
			break // no more all-in boundaries
		}

		// Build a pot up to minAllIn per seat.
		var pool Pool
		for i := 0; i < MaxSeats; i++ {
			if remaining[i] == 0 {
				continue
			}
			contrib := min64(remaining[i], minAllIn)
			pool.Amount += contrib
			remaining[i] -= contrib
			if active[i] && chipsIn[i] >= minAllIn {
				pool.Eligible[i] = true
			}
		}
		if pool.Amount > 0 {
			pools = append(pools, pool)
		}

		// The all-in player(s) at this level are now fully committed; clear them.
		for i := 0; i < MaxSeats; i++ {
			if allIn[i] && remaining[i] == 0 {
				allIn[i] = false // won't create another boundary
			}
		}
	}

	// Remaining chips (from non-all-in players) go into the final main pot.
	var mainPool Pool
	for i := 0; i < MaxSeats; i++ {
		if remaining[i] > 0 {
			mainPool.Amount += remaining[i]
			if active[i] {
				mainPool.Eligible[i] = true
			}
		}
	}
	if mainPool.Amount > 0 {
		pools = append(pools, mainPool)
	}

	return pools
}

// uniqueSortedAllInAmounts returns sorted unique chip amounts for all-in players.
func uniqueSortedAllInAmounts(chipsIn [MaxSeats]uint64, allIn [MaxSeats]bool) []uint64 {
	seen := make(map[uint64]bool)
	var amounts []uint64
	for i := 0; i < MaxSeats; i++ {
		if allIn[i] && chipsIn[i] > 0 && !seen[chipsIn[i]] {
			seen[chipsIn[i]] = true
			amounts = append(amounts, chipsIn[i])
		}
	}
	// Insertion sort (n≤9)
	for i := 1; i < len(amounts); i++ {
		for j := i; j > 0 && amounts[j] < amounts[j-1]; j-- {
			amounts[j], amounts[j-1] = amounts[j-1], amounts[j]
		}
	}
	return amounts
}

// sumContributed returns how many chips from seat i have already been allocated to pools.
func sumContributed(chipsIn [MaxSeats]uint64, pools []Pool, seat int) uint64 {
	_ = chipsIn
	_ = seat
	var total uint64
	for _, p := range pools {
		_ = p
	}
	return total
}

func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
