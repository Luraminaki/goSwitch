package webapp

import (
	"log"

	"net/http"

	"github.com/labstack/echo/v4"

	grid "goSwitch/modules/grid"
	template "goSwitch/modules/template"
	utils "goSwitch/modules/utils"
)

// STRUCTS

type WebAppX struct {
	Config     *utils.Config
	SwitchGame *grid.Grid
	Server     *echo.Echo
}

// WebApp

func NewWebApp() *WebAppX {
	config := utils.ParseJsonConfig("./config.json")

	switchGame := grid.NewGrid(config.Dim, utils.BuildNeighborhoodFromConfig(&config))
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
		"Config":   wx.Config,
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
		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
	}

	log.Printf("%s Data recieved: %v\n", line, jsonMap)

	dim, resp := utils.ParseDim(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
	}

	neighborhood, resp := utils.ParseNeighborhood(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
	}

	cheat, resp := utils.ParseCheat(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
	}

	wx.Config.Dim = dim
	wx.Config.ToggleSequence = utils.BuildToggleSequenceFromRequest(neighborhood, wx.Config.AvailableToggleSequence)
	wx.Config.Cheat = cheat

	wx.SwitchGame = grid.NewGrid(dim, neighborhood)
	log.Printf("%s Possible solution: %v\n", line, wx.SwitchGame.GetPossibleSolution())
	wx.SwitchGame.PrettyPrintGrid()

	return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
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

		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
	}

	wx.SwitchGame.Switch(moves[len(moves)-1])
	moves = moves[:len(moves)-1]
	wx.SwitchGame.SetPreviousMoves(moves)

	log.Printf("%s Move History: %v\n", line, moves)
	wx.SwitchGame.PrettyPrintGrid()

	return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
}

func (wx *WebAppX) Switch(c echo.Context) error {
	line := utils.Trace(false)

	resp, jsonMap := utils.ProcessRequestQuery(c)
	if resp["Status"] == "ERROR" {
		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
	}

	log.Printf("%s Data recieved: %v\n", line, jsonMap)

	row, col, resp := utils.ParseRowCol(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
	}

	pos := (wx.SwitchGame.Rows * row) + col

	wx.SwitchGame.Switch(pos)
	moves := wx.SwitchGame.GetPreviousMoves()
	moves = append(moves, pos)
	wx.SwitchGame.SetPreviousMoves(moves)

	log.Printf("%s Move History: %v\n", line, moves)
	wx.SwitchGame.PrettyPrintGrid()

	return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(), resp))
}
