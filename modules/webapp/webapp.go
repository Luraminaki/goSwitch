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
)

type Response struct {
	Status string
	Win    bool
	Error  string
}

type Config struct {
	Dim            int   `json:"Dim"`
	ToggleSequence []int `json:"ToggleSequence"`
}

type WebAppX struct {
	Config     *Config
	SwitchGame *grid.Grid
	Server     *echo.Echo
}

func NewWebApp() *WebAppX {
	jsonFile, err := os.Open("./config.json")

	if err != nil {
		log.Fatal("Error when opening JSON file: ", err.Error())
	}

	defer jsonFile.Close()

	if err != nil {
		log.Fatal("Error when reading JSON file: ", err.Error())
	}

	var config Config
	err = json.NewDecoder(jsonFile).Decode(&config)

	if err != nil {
		log.Fatal("Error when parsing JSON file: ", err.Error())
	}

	switchGame := grid.NewGrid(config.Dim, config.ToggleSequence)
	switchGame.PrettyPrintGrid()

	server := echo.New()
	server.File("/", "webui/index.html")
	server.File("/favicon.ico", "webui/favicon.ico")

	webApp := &WebAppX{
		Config:     &config,
		SwitchGame: switchGame,
		Server:     server,
	}

	return webApp
}

func (wx *WebAppX) ToggleButton(c echo.Context) error {
	resp := Response{
		Status: "SUCCESS",
		Win:    false,
		Error:  "",
	}

	pos, err := strconv.Atoi(c.Param("pos"))
	if err != nil {
		resp.Status = "ERROR"
		resp.Error = "Params error: " + err.Error()
		return c.JSON(http.StatusOK, resp)
	}

	fmt.Printf("Switching (%d)\n\n", pos)
	wx.SwitchGame.Switch(pos)
	wx.SwitchGame.PrettyPrintGrid()

	resp.Win = wx.SwitchGame.CheckWin()
	if resp.Win {
		fmt.Println("Did I Win: Yes")
	} else {
		fmt.Println("Did I Win: No")
	}

	return c.JSON(http.StatusOK, resp)
}