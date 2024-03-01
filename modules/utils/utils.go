package utils

import (
	"fmt"
	"runtime"

	"encoding/json"

	"github.com/labstack/echo/v4"
)

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

func ConvertSlice[E any](in []any) (out []E) {
	out = make([]E, 0, len(in))
	for _, v := range in {
		out = append(out, v.(E))
	}
	return
}
