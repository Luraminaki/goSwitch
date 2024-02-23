package grid

import (
	"fmt"
	"math/rand"
	"time"
)

type Grid struct {
	rows, cols int
	grid       []int
	rand       *rand.Rand
}

func NewGrid(cols, rows int) *Grid {
	g := &Grid{
		rows: rows,
		cols: cols,
		grid: make([]int, cols*rows),
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
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

func (g *Grid) checkOOB(x, y int) bool {
	return x >= 0 && x < g.cols && y >= 0 && y < g.rows
}

func (g *Grid) CheckWin() bool {
	sum := 0
	for _, val := range g.grid {
		sum += val
	}
	return sum == 0 || sum == g.rows*g.cols
}

func (g *Grid) switchV4(x, y int) [][2]int {
	var coordsToSwitch [][2]int
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
	var coordsToSwitch [][2]int
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

func (g *Grid) SwitchCell(x, y int, neighborhood []int) {
	if !g.checkOOB(x, y) {
		return
	}
	var coordsToSwitch [][2]int
	for _, val := range neighborhood {
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
		cx, cy := coord[0], coord[1]
		g.grid[cx+g.cols*cy] = 1 - g.grid[cx+g.cols*cy]
	}
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
