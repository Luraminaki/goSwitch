// Package grid implements the switch-toggle puzzle board: a square grid of
// two-state cells where switching one cell also flips its neighbors according to
// a configurable pattern (self, orthogonal, and/or diagonal).
package grid

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"strings"
	"time"

	utils "goSwitch/modules/utils"
)

type Grid struct {
	Dim          int
	neighborhood []int
	grid         []int
	solution     []int
	moveHistory  []int
	rand         *rand.Rand
}

// maxInitAttempts bounds the "regenerate until not already won" retry loop in
// NewGrid. Some (dim, neighborhood) combinations are structurally degenerate --
// e.g. a 2x2 grid with every pattern enabled has every switch touch all 4 cells
// uniformly, so the board can never be anything but solved -- without a cap the
// loop would spin forever instead of just being unlikely to need many retries.
const maxInitAttempts = 1000

// randSeed returns a seed sourced from crypto/rand rather than time.Now().UnixNano(),
// so two Grids created in the same process at nearly the same instant (e.g. two
// sessions Reset concurrently) can't end up with identical "random" boards/solutions --
// a real risk with a coarse, clock-based seed under concurrent creation. Falls back to
// the clock only if crypto/rand is ever unavailable, since this is puzzle shuffling, not
// a security-sensitive use of randomness.
func randSeed() int64 {
	var buf [8]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(buf[:])) //nolint:gosec // puzzle shuffling, not security-sensitive
}

// NewGrid builds a dim x dim board using neighborhood as the active toggle patterns.
// dim must be >= 1: dim <= 0 panics (a zero-or-negative-size board isn't a meaningful
// precondition to support), and dim == 1 -- while it won't panic -- produces a trivial
// single-cell board whose only two possible states are both already "won", so callers
// wanting an actual puzzle should use dim >= 2.
func NewGrid(dim int, neighborhood []int) *Grid {
	g := &Grid{
		Dim:          dim,
		neighborhood: neighborhood,
		grid:         make([]int, dim*dim),
		rand:         rand.New(rand.NewSource(randSeed())), //nolint:gosec // puzzle shuffling, not security-sensitive; the seed itself comes from crypto/rand
	}

	g.initGame()
	for attempts := 0; g.CheckWin() && attempts < maxInitAttempts; attempts++ {
		g.initGame()
	}

	if g.CheckWin() {
		// Structurally degenerate configuration (see maxInitAttempts doc comment):
		// every reachable Switch() result is itself a win, so there is no sequence of
		// real moves that both starts from a won board and ends unsolved. Force a
		// single raw flip instead -- this deliberately bypasses Switch's neighborhood
		// fanout, so g.solution can't describe it as a move sequence; clear it rather
		// than report a "solution" that doesn't actually solve this board.
		g.grid[0] = 1 - g.grid[0]
		g.solution = nil
	}

	return g
}

func (g *Grid) initGame() {
	gridSize := g.Dim * g.Dim
	hits := make([]int, gridSize)

	start := g.rand.Intn(2)

	for pos := range gridSize {
		g.grid[pos] = start
		hits[pos] = pos
	}

	g.rand.Shuffle(gridSize, func(i, j int) {
		hits[i], hits[j] = hits[j], hits[i]
	})

	randIndex := g.rand.Intn(gridSize)
	g.solution = hits[:randIndex]
	sort.Ints(g.solution)

	for _, hit := range g.solution {
		g.Switch(hit)
	}
}

// coordFlatToCart converts a flat board position into (x, y) cartesian coordinates.
func (g *Grid) coordFlatToCart(pos int) (int, int) {
	if pos >= len(g.grid) {
		return -1, -1
	}
	return pos % g.Dim, pos / g.Dim
}

func (g *Grid) checkOOB(x, y int) bool {
	return (0 <= x && x < g.Dim) && (0 <= y && y < g.Dim)
}

// orthogonalOffsets and diagonalOffsets are the (dx, dy) pairs for the "4" (plus-shaped)
// and "8" (diagonal) neighborhood patterns, relative to the switched cell.
var (
	orthogonalOffsets = [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	diagonalOffsets   = [][2]int{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
)

// neighborsAt returns the in-bounds cells at (x,y)+offset for each offset, discarding
// any that fall off the board.
func (g *Grid) neighborsAt(x, y int, offsets [][2]int) [][2]int {
	coordsToSwitch := [][2]int{}
	for _, off := range offsets {
		nx, ny := x+off[0], y+off[1]
		if g.checkOOB(nx, ny) {
			coordsToSwitch = append(coordsToSwitch, [2]int{nx, ny})
		}
	}
	return coordsToSwitch
}

func (g *Grid) Switch(pos int) {
	x, y := g.coordFlatToCart(pos)

	if !g.checkOOB(x, y) {
		return
	}

	var coordsToSwitch [][2]int
	for _, val := range g.neighborhood {
		switch val {
		case 0:
			coordsToSwitch = append(coordsToSwitch, [2]int{x, y})
		case 4:
			coordsToSwitch = append(coordsToSwitch, g.neighborsAt(x, y, orthogonalOffsets)...)
		case 8:
			coordsToSwitch = append(coordsToSwitch, g.neighborsAt(x, y, diagonalOffsets)...)
		}
	}
	for _, coord := range coordsToSwitch {
		g.grid[coord[0]+g.Dim*coord[1]] = 1 - g.grid[coord[0]+g.Dim*coord[1]]
	}
}

// GetGrid returns a defensive copy of the board, safe to read after the caller
// releases whatever lock was guarding this Grid.
func (g *Grid) GetGrid() [][]int {
	customGrid := make([][]int, g.Dim)
	for idx := 0; idx < g.Dim; idx++ {
		row := make([]int, g.Dim)
		copy(row, g.grid[g.Dim*idx:(idx+1)*g.Dim])
		customGrid[idx] = row
	}
	return customGrid
}

func (g *Grid) GetPossibleSolution() []int {
	return append([]int(nil), g.solution...)
}

func (g *Grid) GetPreviousMoves() []int {
	return append([]int(nil), g.moveHistory...)
}

// SetPreviousMoves stores a defensive copy of moveHistory, matching every Get*
// accessor's own defensive copy on the way out -- so a caller mutating its slice
// afterward can't reach back in and corrupt g.moveHistory.
func (g *Grid) SetPreviousMoves(moveHistory []int) {
	g.moveHistory = append([]int(nil), moveHistory...)
}

func (g *Grid) CheckWin() bool {
	sum := 0
	for _, val := range g.grid {
		sum += val
	}
	return sum == 0 || sum == g.Dim*g.Dim
}

func (g *Grid) PrettyPrintGrid() {
	var sb strings.Builder

	sb.WriteString("Game Layout:\n")
	for _, row := range g.GetGrid() {
		for _, cell := range row {
			fmt.Fprintf(&sb, "%d ", cell)
		}
		sb.WriteString("\n")
	}

	slog.Debug(sb.String(), utils.FuncAttrKey, utils.Caller())
}
