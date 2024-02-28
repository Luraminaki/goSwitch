package grid

import (
	"fmt"
	"math/rand"
	"time"
)

type Grid struct {
	dim          int
	rows         int
	cols         int
	neighborhood []int
	grid         []int
	rand         *rand.Rand
}

func NewGrid(dim int, neighborhood []int) *Grid {
	g := &Grid{
		dim:          dim,
		rows:         dim,
		cols:         dim,
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
	for r := 0; r < g.rows; r++ {
		for c := 0; c < g.cols; c++ {
			g.grid[c+r*g.cols] = g.rand.Intn(2)
		}
	}
}

func (g *Grid) coordFlatToCart(dim int) (int, int) {
	if dim >= len(g.grid) {
		return -1, -1
	}
	return dim % g.dim, dim / g.dim
}

func (g *Grid) checkOOB(x, y int) bool {
	return (0 <= x && x < g.cols) && (0 <= y && y < g.rows)
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

func (g *Grid) GetGrid() []int {
	return append([]int(nil), g.grid...)
}

func (g *Grid) Switch(dim int) []int {
	x, y := g.coordFlatToCart(dim)
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
		g.grid[coord[0]+g.cols*coord[1]] = 1 - g.grid[coord[0]+g.cols*coord[1]]
	}
	return g.grid
}

func (g *Grid) CheckWin() bool {
	sum := 0
	for _, val := range g.grid {
		sum += val
	}
	return sum == 0 || sum == g.rows*g.cols
}

func (g *Grid) PrettyPrintGrid() {
	fmt.Println("Game Layout:")
	for r := 0; r < g.rows; r++ {
		for c := 0; c < g.cols; c++ {
			fmt.Printf("%d ", g.grid[c+r*g.cols])
		}
		fmt.Println()
	}
	fmt.Println()
}
