package grid

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

type Grid struct {
	Rows         int
	Cols         int
	neighborhood []int
	grid         []int
	solution     []int
	moveHistory  []int
	rand         *rand.Rand
}

func NewGrid(dim int, neighborhood []int) *Grid {
	g := &Grid{
		Rows:         dim,
		Cols:         dim,
		neighborhood: neighborhood,
		grid:         make([]int, dim*dim),
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	g.initGame()
	for g.CheckWin() {
		g.initGame()
	}
	return g
}

func (g *Grid) initGame() {
	gridSize := g.Rows * g.Cols
	hits := make([]int, gridSize)

	start := g.rand.Intn(2)

	for pos := range gridSize {
		g.grid[pos] = start
		hits[pos] = pos
	}

	g.rand.Shuffle(gridSize, func(i, j int) {
		hits[i], hits[j] = hits[j], hits[i]
	})

	randIndex := rand.Intn(gridSize)
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
	return dim % g.Cols, dim / g.Rows
}

func (g *Grid) checkOOB(x, y int) bool {
	return (0 <= x && x < g.Cols) && (0 <= y && y < g.Rows)
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

func (g *Grid) Switch(pos int) []int {
	x, y := g.coordFlatToCart(pos)

	if !g.checkOOB(x, y) {
		return g.grid
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
		g.grid[coord[0]+g.Cols*coord[1]] = 1 - g.grid[coord[0]+g.Cols*coord[1]]
	}
	return g.grid
}

func (g *Grid) GetGrid() [][]int {
	customGrid := make([][]int, len(g.grid)/g.Rows)
	for idx := 0; idx < len(g.grid)/g.Rows; idx++ {
		customGrid[idx] = g.grid[g.Cols*idx : (idx+1)*g.Cols]
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
	return sum == 0 || sum == g.Rows*g.Cols
}

func (g *Grid) PrettyPrintGrid() {
	fmt.Println("Game Layout:")
	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			fmt.Printf("%d ", g.grid[c+r*g.Cols])
		}
		fmt.Println()
	}
	fmt.Println()
}
