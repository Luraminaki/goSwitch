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
