package grid

import (
	"testing"
	"time"
)

func TestNewGridProducesConsistentUnsolvedBoard(t *testing.T) {
	for _, dim := range []int{2, 3, 4, 5} {
		for _, neighborhood := range [][]int{{0}, {4}, {8}, {0, 4}, {4, 8}, {0, 4, 8}} {
			// Repeated to exercise the "regenerate until not already won" retry loop.
			for i := 0; i < 20; i++ {
				g := NewGrid(dim, neighborhood)

				if g.Dim != dim {
					t.Fatalf("dim=%d: Dim = %d, want %d", dim, g.Dim, dim)
				}

				board := g.GetGrid()
				if len(board) != dim {
					t.Fatalf("dim=%d: GetGrid() returned %d rows, want %d", dim, len(board), dim)
				}
				for _, row := range board {
					if len(row) != dim {
						t.Fatalf("dim=%d: GetGrid() row has %d cols, want %d", dim, len(row), dim)
					}
				}

				if g.CheckWin() {
					t.Fatalf("dim=%d neighborhood=%v: NewGrid produced an already-won board", dim, neighborhood)
				}

				solution := g.GetPossibleSolution()
				for _, pos := range solution {
					if pos < 0 || pos >= dim*dim {
						t.Fatalf("solution position %d out of range for dim=%d", pos, dim)
					}
				}
				for i := 1; i < len(solution); i++ {
					if solution[i-1] > solution[i] {
						t.Fatalf("solution %v is not sorted ascending", solution)
					}
				}
			}
		}
	}
}

// TestGetPossibleSolutionIsActuallyValid is a regression test for a bug where the
// maxInitAttempts fallback recorded a "solution" that didn't actually solve the board:
// whenever GetPossibleSolution() is non-empty, applying every move in it, in order, via
// Switch must reach CheckWin()==true. This would have caught that bug directly, instead
// of relying on eyeballing the rendered cheat hint. An empty solution is a separate,
// valid case (the structurally-degenerate maxInitAttempts fallback deliberately leaves
// it empty, since no real move sequence solves that board -- see NewGrid) and is skipped
// here rather than treated as a failure to reach a win.
func TestGetPossibleSolutionIsActuallyValid(t *testing.T) {
	for _, dim := range []int{2, 3, 4, 5} {
		for _, neighborhood := range [][]int{{0}, {4}, {8}, {0, 4}, {4, 8}, {0, 4, 8}} {
			for i := 0; i < 20; i++ {
				g := NewGrid(dim, neighborhood)

				solution := g.GetPossibleSolution()
				if len(solution) == 0 {
					continue
				}

				for _, pos := range solution {
					g.Switch(pos)
				}

				if !g.CheckWin() {
					t.Fatalf("dim=%d neighborhood=%v: applying GetPossibleSolution() %v did not reach a win, board: %v",
						dim, neighborhood, solution, g.GetGrid())
				}
			}
		}
	}
}

// TestSwitchAtCorner exercises switchV4/switchV8's OOB-clipping logic, which the
// center-of-a-3x3-grid cases used elsewhere in this file can't reach: a corner has
// fewer than 4 orthogonal/diagonal neighbors, so clipping bugs are only visible here.
func TestSwitchAtCorner(t *testing.T) {
	tests := []struct {
		name         string
		neighborhood []int
		want         []int
	}{
		{"self", []int{0}, []int{1, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"orthogonal", []int{4}, []int{0, 1, 0, 1, 0, 0, 0, 0, 0}},
		{"diagonal", []int{8}, []int{0, 0, 0, 0, 1, 0, 0, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Grid{Dim: 3, neighborhood: tt.neighborhood, grid: make([]int, 9)}
			g.Switch(0) // top-left corner
			assertGrid(t, g, tt.want)
		})
	}
}

// TestNewGridNeverHangsOnDegenerateNeighborhood is a regression test: a 2x2 grid
// with every pattern enabled ({0,4,8}) makes every switch touch all 4 cells
// uniformly, so the board can never be anything but solved. NewGrid must bail out
// via maxInitAttempts instead of spinning forever.
func TestNewGridNeverHangsOnDegenerateNeighborhood(t *testing.T) {
	done := make(chan *Grid, 1)

	go func() {
		done <- NewGrid(2, []int{0, 4, 8})
	}()

	select {
	case g := <-done:
		if g.CheckWin() {
			t.Fatalf("NewGrid returned an already-won board for a degenerate configuration: %v", g.GetGrid())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("NewGrid hung on a degenerate (dim=2, neighborhood={0,4,8}) configuration")
	}
}

func TestSwitchSelfNeighborhood(t *testing.T) {
	g := &Grid{Dim: 3, neighborhood: []int{0}, grid: make([]int, 9)}

	g.Switch(4) // center of a 3x3 grid

	want := []int{0, 0, 0, 0, 1, 0, 0, 0, 0}
	assertGrid(t, g, want)
}

func TestSwitchPlusNeighborhood(t *testing.T) {
	g := &Grid{Dim: 3, neighborhood: []int{4}, grid: make([]int, 9)}

	g.Switch(4) // center: flips its 4 orthogonal neighbors, not itself

	want := []int{0, 1, 0, 1, 0, 1, 0, 1, 0}
	assertGrid(t, g, want)
}

func TestSwitchDiagonalNeighborhood(t *testing.T) {
	g := &Grid{Dim: 3, neighborhood: []int{8}, grid: make([]int, 9)}

	g.Switch(4) // center: flips its 4 diagonal neighbors, not itself

	want := []int{1, 0, 1, 0, 0, 0, 1, 0, 1}
	assertGrid(t, g, want)
}

func TestSwitchCombinedNeighborhoods(t *testing.T) {
	g := &Grid{Dim: 3, neighborhood: []int{0, 4, 8}, grid: make([]int, 9)}

	g.Switch(4) // self + orthogonal + diagonal covers every cell of a 3x3 grid

	want := []int{1, 1, 1, 1, 1, 1, 1, 1, 1}
	assertGrid(t, g, want)
}

func TestSwitchOutOfBoundsIsNoOp(t *testing.T) {
	for _, pos := range []int{-1, -100, 9, 100} {
		g := &Grid{Dim: 3, neighborhood: []int{0, 4, 8}, grid: make([]int, 9)}

		g.Switch(pos)

		assertGrid(t, g, make([]int, 9))
	}
}

func TestGetGridReturnsDefensiveCopy(t *testing.T) {
	g := &Grid{Dim: 2, grid: []int{1, 0, 0, 1}}

	board := g.GetGrid()
	board[0][0] = 99
	board[1][1] = 42

	if g.grid[0] != 1 || g.grid[3] != 1 {
		t.Fatalf("mutating GetGrid()'s result affected the underlying grid: %v", g.grid)
	}
}

func TestGetPossibleSolutionReturnsDefensiveCopy(t *testing.T) {
	g := &Grid{solution: []int{1, 2, 3}}

	sol := g.GetPossibleSolution()
	sol[0] = 99

	if g.solution[0] != 1 {
		t.Fatalf("mutating GetPossibleSolution()'s result affected internal state: %v", g.solution)
	}
}

func TestPreviousMovesRoundTrip(t *testing.T) {
	g := &Grid{}

	if moves := g.GetPreviousMoves(); moves != nil {
		t.Fatalf("GetPreviousMoves() on a fresh Grid = %v, want nil", moves)
	}

	g.SetPreviousMoves([]int{1, 2, 3})

	moves := g.GetPreviousMoves()
	if len(moves) != 3 || moves[0] != 1 || moves[1] != 2 || moves[2] != 3 {
		t.Fatalf("GetPreviousMoves() = %v, want [1 2 3]", moves)
	}

	moves[0] = 99
	if g.moveHistory[0] != 1 {
		t.Fatalf("mutating GetPreviousMoves()'s result affected internal state: %v", g.moveHistory)
	}
}

// TestSetPreviousMovesDefensiveCopy is a regression test: SetPreviousMoves must copy
// its argument, matching every Get* accessor's own defensive copy, so a caller
// mutating the slice it passed in afterward can't reach back in and corrupt state.
func TestSetPreviousMovesDefensiveCopy(t *testing.T) {
	g := &Grid{}

	moves := []int{1, 2, 3}
	g.SetPreviousMoves(moves)

	moves[0] = 99
	if g.moveHistory[0] != 1 {
		t.Fatalf("mutating the slice passed to SetPreviousMoves() affected internal state: %v", g.moveHistory)
	}
}

// TestRecordMoveTogglesMembership is a regression test for the toggle semantics
// RecordMove exists for: switching the same pos twice cancels out on the board (see
// Switch's doc comment), so the recorded history should reflect that -- a repeat
// entry removes the existing one instead of appending a duplicate -- while leaving
// the relative order of any other still-recorded moves untouched.
func TestRecordMoveTogglesMembership(t *testing.T) {
	g := &Grid{}

	g.RecordMove(0)
	if moves := g.GetPreviousMoves(); len(moves) != 1 || moves[0] != 0 {
		t.Fatalf("after RecordMove(0), moves = %v, want [0]", moves)
	}

	g.RecordMove(4)
	if moves := g.GetPreviousMoves(); len(moves) != 2 || moves[0] != 0 || moves[1] != 4 {
		t.Fatalf("after RecordMove(0), RecordMove(4), moves = %v, want [0 4]", moves)
	}

	// Reclicking 0 (not the most recent entry) should cancel it out, leaving 4 as
	// the sole (and now most recent) still-effective move.
	g.RecordMove(0)
	if moves := g.GetPreviousMoves(); len(moves) != 1 || moves[0] != 4 {
		t.Fatalf("after re-recording 0, moves = %v, want [4]", moves)
	}

	// Re-clicking 4 a third time overall (present once) should remove it entirely.
	g.RecordMove(4)
	if moves := g.GetPreviousMoves(); moves != nil {
		t.Fatalf("after canceling out the only remaining move, moves = %v, want nil", moves)
	}
}

// TestPopLastMove is a regression test for RevertMove's undo semantics: it must pop
// the most recently recorded move and report ok=false once history is empty, rather
// than the old bare-nil-slice check.
func TestPopLastMove(t *testing.T) {
	g := &Grid{}

	if _, ok := g.PopLastMove(); ok {
		t.Fatal("PopLastMove() on a fresh Grid should report ok=false")
	}

	g.RecordMove(1)
	g.RecordMove(2)

	pos, ok := g.PopLastMove()
	if !ok || pos != 2 {
		t.Fatalf("PopLastMove() = (%d, %v), want (2, true)", pos, ok)
	}
	if moves := g.GetPreviousMoves(); len(moves) != 1 || moves[0] != 1 {
		t.Fatalf("after popping 2, moves = %v, want [1]", moves)
	}

	pos, ok = g.PopLastMove()
	if !ok || pos != 1 {
		t.Fatalf("PopLastMove() = (%d, %v), want (1, true)", pos, ok)
	}
	if _, ok := g.PopLastMove(); ok {
		t.Fatal("PopLastMove() on an emptied history should report ok=false")
	}
}

func TestCheckWin(t *testing.T) {
	tests := []struct {
		name string
		grid []int
		want bool
	}{
		{"all zeros", []int{0, 0, 0, 0}, true},
		{"all ones", []int{1, 1, 1, 1}, true},
		{"mixed", []int{1, 0, 1, 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Grid{Dim: 2, grid: tt.grid}
			if got := g.CheckWin(); got != tt.want {
				t.Errorf("CheckWin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewGridDimEdgeCases documents NewGrid's behavior at the edges of its exported
// contract (dim=0, dim=1) -- none of these are reachable via the HTTP API, since
// utils.ParseDim clamps to [2,5], but NewGrid itself has no such guard, so this pins
// down the actual behavior for any other caller.
func TestNewGridDimEdgeCases(t *testing.T) {
	t.Run("dim=0 panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("NewGrid(0, ...) did not panic (rand.Intn(0) is documented to)")
			}
		}()
		NewGrid(0, []int{0})
	})

	t.Run("dim=1 is tautologically always won", func(t *testing.T) {
		g := NewGrid(1, []int{0})
		if !g.CheckWin() {
			t.Fatal("dim=1's single cell should always satisfy CheckWin() (sum is always 0 or Dim*Dim)")
		}
	})
}

// TestSwitchWithDuplicateOrUnknownNeighborhood documents Switch's actual behavior for
// neighborhood values outside {0,4,8} or containing duplicates -- not reachable via the
// HTTP API today (utils.ParseNeighborhood rejects both), but Switch itself has no
// validation of its own, so this pins down what a direct caller actually gets.
func TestSwitchWithDuplicateOrUnknownNeighborhood(t *testing.T) {
	t.Run("duplicate pattern cancels out to a no-op", func(t *testing.T) {
		g := &Grid{Dim: 3, neighborhood: []int{4, 4}, grid: make([]int, 9)}
		g.Switch(4)
		assertGrid(t, g, make([]int, 9)) // each orthogonal neighbor toggled twice = unchanged
	})

	t.Run("unknown pattern value is silently ignored", func(t *testing.T) {
		g := &Grid{Dim: 3, neighborhood: []int{99}, grid: make([]int, 9)}
		g.Switch(4)
		assertGrid(t, g, make([]int, 9)) // no case in Switch's switch matches 99
	})
}

func assertGrid(t *testing.T, g *Grid, want []int) {
	t.Helper()

	if len(g.grid) != len(want) {
		t.Fatalf("grid length = %d, want %d", len(g.grid), len(want))
	}
	for i := range want {
		if g.grid[i] != want[i] {
			t.Fatalf("grid = %v, want %v", g.grid, want)
		}
	}
}
