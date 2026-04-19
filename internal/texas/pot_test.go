package texas

import (
	"testing"
)

// chips is a helper to build a [MaxSeats]uint64 from a short list.
// Unspecified seats are 0.
func chips(vals ...uint64) [MaxSeats]uint64 {
	var arr [MaxSeats]uint64
	for i, v := range vals {
		if i >= MaxSeats {
			break
		}
		arr[i] = v
	}
	return arr
}

// allin is a helper to build a [MaxSeats]bool from a set of seat indices.
func allin(seats ...int) [MaxSeats]bool {
	var arr [MaxSeats]bool
	for _, s := range seats {
		arr[s] = true
	}
	return arr
}

// active is a helper: all seats with non-zero chips are active unless excluded.
func activeFromChips(c [MaxSeats]uint64, folded ...int) [MaxSeats]bool {
	var arr [MaxSeats]bool
	for i := 0; i < MaxSeats; i++ {
		arr[i] = c[i] > 0
	}
	for _, s := range folded {
		arr[s] = false
	}
	return arr
}

func totalPots(pools []Pool) uint64 {
	var total uint64
	for _, p := range pools {
		total += p.Amount
	}
	return total
}

func TestComputePots_NoAllIn(t *testing.T) {
	// 3 players, no all-in: one single pot.
	c := chips(100, 100, 100)
	ai := allin()
	act := activeFromChips(c)

	pools := ComputePots(c, ai, act)

	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d: %v", len(pools), pools)
	}
	if pools[0].Amount != 300 {
		t.Errorf("expected pot=300, got %d", pools[0].Amount)
	}
	if !pools[0].Eligible[0] || !pools[0].Eligible[1] || !pools[0].Eligible[2] {
		t.Error("all 3 seats should be eligible")
	}
	if totalPots(pools) != 300 {
		t.Errorf("total %d ≠ chip sum 300", totalPots(pools))
	}
}

func TestComputePots_OneAllIn_ShortStack(t *testing.T) {
	// Seat 0: all-in for 50 (short stack)
	// Seat 1: called full 100
	// Seat 2: called full 100
	// Expected: side pot 50*3=150 (all eligible), main pot 50+50=100 (seats 1&2 only)
	c := chips(50, 100, 100)
	ai := allin(0)
	act := activeFromChips(c)

	pools := ComputePots(c, ai, act)

	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d: %v", len(pools), pools)
	}

	sidePot := pools[0]
	mainPot := pools[1]

	if sidePot.Amount != 150 {
		t.Errorf("side pot: expected 150, got %d", sidePot.Amount)
	}
	if !sidePot.Eligible[0] || !sidePot.Eligible[1] || !sidePot.Eligible[2] {
		t.Error("all 3 seats should be eligible for side pot")
	}

	if mainPot.Amount != 100 {
		t.Errorf("main pot: expected 100, got %d", mainPot.Amount)
	}
	if mainPot.Eligible[0] {
		t.Error("all-in seat 0 should NOT be eligible for main pot")
	}
	if !mainPot.Eligible[1] || !mainPot.Eligible[2] {
		t.Error("seats 1&2 should be eligible for main pot")
	}

	if total := totalPots(pools); total != 250 {
		t.Errorf("total %d ≠ chip sum 250", total)
	}
}

func TestComputePots_TwoAllIn_DifferentStacks(t *testing.T) {
	// Seat 0: all-in for 30
	// Seat 1: all-in for 80
	// Seat 2: called full 100
	// Expected:
	//   Pool A: 30*3 = 90 (all eligible)
	//   Pool B: (80-30)*2 = 100 (seats 1&2)
	//   Pool C: (100-80)*1 = 20 (seat 2 only)
	c := chips(30, 80, 100)
	ai := allin(0, 1)
	act := activeFromChips(c)

	pools := ComputePots(c, ai, act)

	if len(pools) != 3 {
		t.Fatalf("expected 3 pools, got %d", len(pools))
	}

	// Pool A: 90
	if pools[0].Amount != 90 {
		t.Errorf("pool A: expected 90, got %d", pools[0].Amount)
	}
	if !pools[0].Eligible[0] || !pools[0].Eligible[1] || !pools[0].Eligible[2] {
		t.Error("pool A: all 3 seats eligible")
	}

	// Pool B: 100
	if pools[1].Amount != 100 {
		t.Errorf("pool B: expected 100, got %d", pools[1].Amount)
	}
	if pools[1].Eligible[0] {
		t.Error("pool B: seat 0 NOT eligible")
	}
	if !pools[1].Eligible[1] || !pools[1].Eligible[2] {
		t.Error("pool B: seats 1&2 eligible")
	}

	// Pool C: 20
	if pools[2].Amount != 20 {
		t.Errorf("pool C: expected 20, got %d", pools[2].Amount)
	}
	if pools[2].Eligible[0] || pools[2].Eligible[1] {
		t.Error("pool C: only seat 2 eligible")
	}
	if !pools[2].Eligible[2] {
		t.Error("pool C: seat 2 must be eligible")
	}

	if total := totalPots(pools); total != 210 {
		t.Errorf("total %d ≠ chip sum 210", total)
	}
}

func TestComputePots_FoldedPlayer(t *testing.T) {
	// Seat 0: folded but already put in 60
	// Seat 1: called 100
	// Seat 2: all-in for 40
	// Seat 0 is NOT active (folded)
	// Expected:
	//   Pool A: 40+40+40=120 (seats 1&2 eligible; seat 0 contributed but folded)
	//   Pool B: (100-40)+(60-40) = 60+20 = 80  seat 1 only eligible (seat 0 folded, seat 2 all-in at 40)
	c := chips(60, 100, 40)
	ai := allin(2)
	act := activeFromChips(c, 0) // seat 0 folded

	pools := ComputePots(c, ai, act)

	// Total chips = 60+100+40 = 200
	if total := totalPots(pools); total != 200 {
		t.Errorf("total chips must be conserved: got %d, want 200", total)
	}

	// Seat 2 (all-in) must not be eligible for pools above their contribution (40).
	for _, p := range pools {
		if p.Eligible[2] && p.Amount > 120 {
			t.Error("seat 2 should not be eligible for pots above their all-in amount")
		}
	}

	// Seat 0 (folded) must never be eligible for any pool.
	for _, p := range pools {
		if p.Eligible[0] {
			t.Error("folded seat 0 must not be eligible for any pool")
		}
	}
}

func TestComputePots_AllPlayersAllIn_EqualStacks(t *testing.T) {
	// All 3 players all-in for equal amounts: one pot, all eligible.
	c := chips(200, 200, 200)
	ai := allin(0, 1, 2)
	act := activeFromChips(c)

	pools := ComputePots(c, ai, act)

	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].Amount != 600 {
		t.Errorf("expected 600, got %d", pools[0].Amount)
	}
}

func TestComputePots_HeadsUp_AllIn(t *testing.T) {
	// Heads-up: seat 0 all-in for 300, seat 1 has 500.
	// Pool A: 600 (both eligible)
	// Pool B: 200 (seat 1 only)
	c := chips(300, 500)
	ai := allin(0)
	act := activeFromChips(c)

	pools := ComputePots(c, ai, act)

	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}
	if pools[0].Amount != 600 {
		t.Errorf("pool A: expected 600, got %d", pools[0].Amount)
	}
	if pools[1].Amount != 200 {
		t.Errorf("pool B: expected 200, got %d", pools[1].Amount)
	}
	if pools[1].Eligible[0] {
		t.Error("pool B: seat 0 not eligible")
	}
	if total := totalPots(pools); total != 800 {
		t.Errorf("total %d ≠ 800", total)
	}
}

func TestComputePots_ThreeAllIn_Ascending(t *testing.T) {
	// 4 players: 3 all-in at different amounts, 1 caller with most chips
	// Seat 0: all-in 100
	// Seat 1: all-in 200
	// Seat 2: all-in 300
	// Seat 3: called 400
	c := chips(100, 200, 300, 400)
	ai := allin(0, 1, 2)
	act := activeFromChips(c)

	pools := ComputePots(c, ai, act)

	if total := totalPots(pools); total != 1000 {
		t.Errorf("total %d ≠ 1000", total)
	}

	// Pool 1: 100*4=400, all eligible
	// Pool 2: (200-100)*3=300, seats 1,2,3 eligible
	// Pool 3: (300-200)*2=200, seats 2,3 eligible
	// Pool 4: (400-300)*1=100, seat 3 only
	if len(pools) != 4 {
		t.Fatalf("expected 4 pools, got %d", len(pools))
	}
	expects := []struct{ amount uint64; eligible [4]bool }{
		{400, [4]bool{true, true, true, true}},
		{300, [4]bool{false, true, true, true}},
		{200, [4]bool{false, false, true, true}},
		{100, [4]bool{false, false, false, true}},
	}
	for i, exp := range expects {
		if pools[i].Amount != exp.amount {
			t.Errorf("pool %d: expected %d, got %d", i, exp.amount, pools[i].Amount)
		}
		for s := 0; s < 4; s++ {
			if pools[i].Eligible[s] != exp.eligible[s] {
				t.Errorf("pool %d seat %d: eligible=%v want %v",
					i, s, pools[i].Eligible[s], exp.eligible[s])
			}
		}
	}
}

func TestComputePots_ChipConservation(t *testing.T) {
	// Property: total chips in all pots must always equal total chips contributed.
	cases := [][MaxSeats]uint64{
		chips(50, 50, 50),
		chips(100, 200, 300, 400),
		chips(1, 1000, 1000),
		chips(0, 0, 500, 500, 250),
	}
	allIns := [][MaxSeats]bool{
		allin(),
		allin(0, 1, 2),
		allin(0),
		allin(2, 3),
	}
	for i, c := range cases {
		ai := allIns[i]
		act := activeFromChips(c)
		pools := ComputePots(c, ai, act)
		var sum uint64
		for _, v := range c {
			sum += v
		}
		if total := totalPots(pools); total != sum {
			t.Errorf("case %d: total chips %d ≠ pot sum %d", i, sum, total)
		}
	}
}
