package utils

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"encoding/json"

	"github.com/labstack/echo/v4"
)

type Config struct {
	Port                    string `json:"Port"`
	Cheat                   bool   `json:"Cheat"`
	Dim                     int    `json:"Dim"`
	ToggleSequence          []bool `json:"ToggleSequence"`
	AvailableToggleSequence []int  `json:"AvailableToggleSequence"`
}

func Trace(debug bool) string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()

	var line string

	if debug {
		line = fmt.Sprintf("%s:%d -- %s --", frame.File, frame.Line, frame.Function)
		fmt.Println(line)
	} else {
		line = fmt.Sprintf("%s --", frame.Function)
		fmt.Println(line)
	}

	return line
}

func ParseJsonConfig(path string) Config {
	jsonFile, err := os.Open(path)

	if err != nil {
		log.Fatal("Error when opening JSON file: ", err.Error())
	}

	defer jsonFile.Close()

	var config Config
	err = json.NewDecoder(jsonFile).Decode(&config)

	if err != nil {
		log.Fatal("Error when parsing JSON file: ", err.Error())
	}

	return config
}

func BuildNeighborhoodFromConfig(config *Config) []int {
	neighborhood := make([]int, 0, len(config.AvailableToggleSequence))

	for idx, val := range config.AvailableToggleSequence {
		if config.ToggleSequence[idx] {
			neighborhood = append(neighborhood, val)
		}
	}

	return neighborhood
}

func BuildToggleSequenceFromRequest(neighborhood []int, availableToggleSequence []int) []bool {
	togglesequence := make([]bool, 0, len(availableToggleSequence))
	val_found := false

	for _, val := range availableToggleSequence {
		val_found = false

		for _, neigh := range neighborhood {
			val_found = val == neigh
			if val_found {
				break
			}
		}

		togglesequence = append(togglesequence, val_found)
	}

	return togglesequence
}

func ProcessRequestJson(c echo.Context) (map[string]interface{}, map[string]interface{}) {
	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	jsonMap := make(map[string]interface{})

	err := json.NewDecoder(c.Request().Body).Decode(&jsonMap)
	if err != nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: " + err.Error()
	}

	return resp, jsonMap
}

func ProcessRequestForm(c echo.Context) (map[string]interface{}, map[string]interface{}) {
	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	jsonMap := make(map[string]interface{})

	form, _ := c.FormParams()
	for k, v := range form {
		switch len(v) {
		case 0:
			continue
		default:
			jsonMap[k] = v
		}
	}

	return resp, jsonMap
}

func ProcessRequestQuery(c echo.Context) (map[string]interface{}, map[string]interface{}) {
	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	jsonMap := make(map[string]interface{})

	query := c.QueryParams()
	for k, v := range query {
		switch len(v) {
		case 0:
			continue
		default:
			jsonMap[k] = v
		}
	}

	return resp, jsonMap
}

func ParseDim(jsonMap map[string]interface{}, resp map[string]interface{}, line string) (int, map[string]interface{}) {
	dim, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["dim"].([]string)[0]))

	if err != nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: " + err.Error()
		log.Printf("%s %s", line, resp["Error"])

		return -1, resp
	}

	if dim < 2 || dim > 5 {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: dim ∈ [2, 5]"
		log.Printf("%s %s", line, resp["Error"])

		return -1, resp
	}

	return dim, resp
}

func ParseNeighborhood(jsonMap map[string]interface{}, resp map[string]interface{}, line string) ([]int, map[string]interface{}) {
	neigh := jsonMap["neighborhood"]
	if neigh == nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: 'neighborhood' key missing"
		log.Printf("%s %s", line, resp["Error"])

		return make([]int, 0), resp
	}

	var neighborhood = []int{}
	for _, i := range neigh.([]string) {
		j, err := strconv.Atoi(i)
		if err != nil {
			resp["Status"] = "ERROR"
			resp["Error"] = "Params error: " + err.Error()
			log.Printf("%s %s", line, resp["Error"])

			return make([]int, 0), resp
		}
		neighborhood = append(neighborhood, j)
	}

	if len(neighborhood) == 0 {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: 'neighborhood' value is empty"
		log.Printf("%s %s", line, resp["Error"])

		return make([]int, 0), resp
	}

	return neighborhood, resp
}

func ParseCheat(jsonMap map[string]interface{}, resp map[string]interface{}, line string) (bool, map[string]interface{}) {
	cheat := false
	if jsonMap["cheat"] != nil {
		cheatInt, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["cheat"].([]string)[0]))
		if err != nil {
			resp["Status"] = "ERROR"
			resp["Error"] = "Params error: " + err.Error()
			log.Printf("%s %s", line, resp["Error"])

			return cheat, resp
		}
		cheat = cheatInt != 0
	}

	return cheat, resp
}

func ParseRowCol(jsonMap map[string]interface{}, resp map[string]interface{}, line string) (int, int, map[string]interface{}) {
	row, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["row"].([]string)[0]))

	if err != nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: " + err.Error()
		log.Printf("%s %s", line, resp["Error"])

		return -1, -1, resp
	}

	col, err := strconv.Atoi(fmt.Sprintf("%v", jsonMap["col"].([]string)[0]))

	if err != nil {
		resp["Status"] = "ERROR"
		resp["Error"] = "Params error: " + err.Error()
		log.Printf("%s %s", line, resp["Error"])

		return -1, -1, resp
	}

	return row, col, resp
}

func UpdateStateResponse(state map[string]interface{}, resp map[string]interface{}) map[string]interface{} {
	state["Response"] = resp
	return state
}

func ConvertSlice[E any](in []any) (out []E) {
	out = make([]E, 0, len(in))
	for _, v := range in {
		out = append(out, v.(E))
	}
	return
}
