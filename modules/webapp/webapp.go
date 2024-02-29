package webapp

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"encoding/json"
	"net/http"
	"text/template"

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

type Template struct {
	Templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.Templates.ExecuteTemplate(w, name, data)
}

func NewTemplateRenderer(e *echo.Echo, paths ...string) {
	tmpl := &template.Template{}
	for i := range paths {
		template.Must(tmpl.ParseGlob(paths[i]))
	}
	t := newTemplate(tmpl)
	e.Renderer = t
}

func newTemplate(templates *template.Template) echo.Renderer {
	return &Template{
		Templates: templates,
	}
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

	fmt.Printf("Possible solution: %v\n", switchGame.GetPossibleSolution())

	switchGame.PrettyPrintGrid()

	server := echo.New()

	server.File("/", "webui/index.html")
	server.File("/favicon.ico", "webui/favicon.ico")
	server.File("/assets/style.css", "webui/assets/style.css")
	server.File("/assets/htmx.min.js", "webui/assets/htmx.min.js")

	NewTemplateRenderer(server, "webui/*.html")

	webApp := &WebAppX{
		Config:     &config,
		SwitchGame: switchGame,
		Server:     server,
	}

	return webApp
}

func (wx *WebAppX) InitGrid(c echo.Context) error {
	res := map[string]interface{}{
		"Name": "Luraminaki",
	}
	return c.Render(http.StatusOK, "index", res)
}

func (wx *WebAppX) ToggleButton(c echo.Context) error {
	resp := Response{
		Status: "SUCCESS",
		Win:    false,
		Error:  "",
	}

	jsonMap := make(map[string]interface{})

	err := json.NewDecoder(c.Request().Body).Decode(&jsonMap)
	if err != nil {
		resp.Status = "ERROR"
		resp.Error = "Params error: " + err.Error()
		return c.JSON(http.StatusOK, resp)
	}

	fmt.Printf("Data recieved: %v\n", jsonMap)

	pos, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["pos"]))

	if err != nil {
		resp.Status = "ERROR"
		resp.Error = "Params error: " + err.Error()
		return c.JSON(http.StatusOK, resp)
	}

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
