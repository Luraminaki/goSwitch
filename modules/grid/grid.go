package grid

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"
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

func NewGrid(dim int, neighborhood []int) *Grid {
	g := &Grid{
		Dim:          dim,
		neighborhood: neighborhood,
		grid:         make([]int, dim*dim),
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	g.initGame()
	for attempts := 0; g.CheckWin() && attempts < maxInitAttempts; attempts++ {
		g.initGame()
	}

	if g.CheckWin() {
		// Structurally degenerate configuration: force a single flip so the
		// board isn't handed to the player already solved.
		g.grid[0] = 1 - g.grid[0]
		g.solution = append(g.solution, 0)
		sort.Ints(g.solution)
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

func (g *Grid) coordFlatToCart(dim int) (int, int) {
	if dim >= len(g.grid) {
		return -1, -1
	}
	return dim % g.Dim, dim / g.Dim
}

func (g *Grid) checkOOB(x, y int) bool {
	return (0 <= x && x < g.Dim) && (0 <= y && y < g.Dim)
}

func (g *Grid) switchV4(x, y int) [][2]int {
	coordsToSwitch := [][2]int{}
	if g.checkOOB(x+1, y) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x + 1, y})
	}
	if g.checkOOB(x, y+1) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x, y + 1})
	}
	if g.checkOOB(x-1, y) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x - 1, y})
	}
	if g.checkOOB(x, y-1) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x, y - 1})
	}
	return coordsToSwitch
}

func (g *Grid) switchV8(x, y int) [][2]int {
	coordsToSwitch := [][2]int{}
	if g.checkOOB(x+1, y+1) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x + 1, y + 1})
	}
	if g.checkOOB(x-1, y-1) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x - 1, y - 1})
	}
	if g.checkOOB(x+1, y-1) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x + 1, y - 1})
	}
	if g.checkOOB(x-1, y+1) {
		coordsToSwitch = append(coordsToSwitch, [2]int{x - 1, y + 1})
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
			coordsToSwitch = append(coordsToSwitch, g.switchV4(x, y)...)
		case 8:
			coordsToSwitch = append(coordsToSwitch, g.switchV8(x, y)...)
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

func (g *Grid) SetPreviousMoves(moveHistory []int) {
	g.moveHistory = moveHistory
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
	for r := 0; r < g.Dim; r++ {
		for c := 0; c < g.Dim; c++ {
			fmt.Fprintf(&sb, "%d ", g.grid[c+r*g.Dim])
		}
		sb.WriteString("\n")
	}

	log.Print(sb.String())
}
