package webapp

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	grid "goSwitch/modules/grid"
	template "goSwitch/modules/template"
	utils "goSwitch/modules/utils"
)

// STRUCTS

type Config struct {
	Dim            int   `json:"Dim"`
	ToggleSequence []int `json:"ToggleSequence"`
}

type WebAppX struct {
	Config     *Config
	SwitchGame *grid.Grid
	Server     *echo.Echo
}

// WebApp

func NewWebApp() *WebAppX {
	jsonFile, err := os.Open("./config.json")

	if err != nil {
		log.Fatal("Error when opening JSON file: ", err.Error())
	}

	defer jsonFile.Close()

	var config Config
	err = json.NewDecoder(jsonFile).Decode(&config)

	if err != nil {
		log.Fatal("Error when parsing JSON file: ", err.Error())
	}

	switchGame := grid.NewGrid(config.Dim, config.ToggleSequence)
	log.Printf("Possible solution: %v\n", switchGame.GetPossibleSolution())
	switchGame.PrettyPrintGrid()

	server := echo.New()
	server.File("/", "webui/index.html")
	server.File("/favicon.ico", "webui/favicon.ico")
	server.File("/assets/style.css", "webui/assets/style.css")
	server.File("/assets/htmx.min.js", "webui/assets/htmx.min.js")

	template.NewTemplateRenderer(server, "webui/*.html")

	webApp := &WebAppX{
		Config:     &config,
		SwitchGame: switchGame,
		Server:     server,
	}

	return webApp
}

func (wx *WebAppX) gameState() map[string]interface{} {
	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	res := map[string]interface{}{
		"Board":    wx.SwitchGame.GetGrid(),
		"Solution": wx.SwitchGame.GetPossibleSolution(),
		"Moves":    wx.SwitchGame.GetPreviousMoves(),
		"Win":      wx.SwitchGame.CheckWin(),
		"Response": resp,
	}
	return res
}

func (wx *WebAppX) InitHTMX(c echo.Context) error {
	utils.Trace(false)

	return c.Render(http.StatusOK, "index", wx.gameState())
}

func (wx *WebAppX) Reset(c echo.Context) error {
	line := utils.Trace(false)

	resp, jsonMap := utils.ProcessRequestForm(c)

	if resp["Status"] == "ERROR" {
		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	log.Printf("%s Data recieved: %v\n", line, jsonMap)

	dim, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["dim"].([]string)[0]))
	if err != nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: " + err.Error()
		log.Printf("%s %s", line, resp["Error"])

		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	if dim < 2 {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: dim MUST be equal or higher than 2"
		log.Printf("%s %s", line, resp["Error"])

		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	neigh := jsonMap["neighborhood"].([]string)
	if neigh == nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: 'neighborhood' key missing"
		log.Printf("%s %s", line, resp["Error"])

		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	var neighborhood = []int{}
	for _, i := range neigh {
		j, err := strconv.Atoi(i)
		if err != nil {
			resp["Status"] = "ERROR"
			resp["Error"] = "Params error: " + err.Error()
			log.Printf("%s %s", line, resp["Error"])

			res := wx.gameState()
			res["Response"] = resp
			return c.Render(http.StatusOK, "index", res)
		}
		neighborhood = append(neighborhood, j)
	}

	wx.SwitchGame = grid.NewGrid(dim, neighborhood)
	log.Printf("%s Possible solution: %v\n", line, wx.SwitchGame.GetPossibleSolution())
	wx.SwitchGame.PrettyPrintGrid()

	res := wx.gameState()
	res["Response"] = resp
	return c.Render(http.StatusOK, "index", res)
}

func (wx *WebAppX) RevertMove(c echo.Context) error {
	line := utils.Trace(false)

	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	moves := wx.SwitchGame.GetPreviousMoves()
	if moves == nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Not allowed: Nothing to revert to"

		log.Printf("%s %s", line, resp["Error"])

		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	wx.SwitchGame.Switch(moves[len(moves)-1])
	moves = moves[:len(moves)-1]
	wx.SwitchGame.SetPreviousMoves(moves)

	log.Printf("%s Move History: %v\n", line, moves)
	wx.SwitchGame.PrettyPrintGrid()

	res := wx.gameState()
	res["Response"] = resp
	return c.Render(http.StatusOK, "index", res)
}

func (wx *WebAppX) Switch(c echo.Context) error {
	line := utils.Trace(false)

	resp, jsonMap := utils.ProcessRequestQuery(c)

	if resp["Status"] == "ERROR" {
		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	log.Printf("%s Data recieved: %v\n", line, jsonMap)

	row, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["row"].([]string)[0]))

	if err != nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: " + err.Error()
		log.Printf("%s %s", line, resp["Error"])

		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	col, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["col"].([]string)[0]))

	if err != nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: " + err.Error()
		log.Printf("%s %s", line, resp["Error"])

		res := wx.gameState()
		res["Response"] = resp
		return c.Render(http.StatusOK, "index", res)
	}

	pos := (wx.SwitchGame.Rows * row) + col

	wx.SwitchGame.Switch(pos)
	moves := wx.SwitchGame.GetPreviousMoves()
	moves = append(moves, pos)
	wx.SwitchGame.SetPreviousMoves(moves)

	log.Printf("%s Move History: %v\n", line, moves)
	wx.SwitchGame.PrettyPrintGrid()

	res := wx.gameState()

	if res["Win"].(bool) {
		log.Printf("%s Did I Win: Yes", line)
	} else {
		log.Printf("%s Did I Win: No", line)
	}

	res["Response"] = resp
	return c.Render(http.StatusOK, "index", res)
}
