package utils

import (
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
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

	// MaxSessions caps the number of concurrent per-client sessions.
	MaxSessions int `json:"MaxSessions"`
	// SessionTTLSeconds is the absolute max lifetime of a session from creation.
	SessionTTLSeconds int `json:"SessionTTLSeconds"`
	// SessionIdleTimeoutSeconds is the max allowed inactivity for a session, enforced
	// only when MaxSessions has been reached and a new session needs a slot.
	SessionIdleTimeoutSeconds int `json:"SessionIdleTimeoutSeconds"`
	// SessionWaitCheckIntervalSeconds is how often a waiting client is re-checked for
	// a freed-up slot while it holds open its SSE connection.
	SessionWaitCheckIntervalSeconds int `json:"SessionWaitCheckIntervalSeconds"`

	// LogFilePath is where rotated log files are written.
	LogFilePath string `json:"LogFilePath"`
	// LogMaxSizeMB is the max size in megabytes a log file reaches before it's rotated.
	LogMaxSizeMB int `json:"LogMaxSizeMB"`
	// LogMaxBackups is the max number of rotated log files kept around.
	LogMaxBackups int `json:"LogMaxBackups"`
	// LogLevel is the minimum level logged: DEBUG, INFO, WARN, or ERROR.
	LogLevel string `json:"LogLevel"`

	// RateLimitRequestsPerSecond is the sustained per-client-IP request rate allowed.
	RateLimitRequestsPerSecond float64 `json:"RateLimitRequestsPerSecond"`
	// RateLimitBurst is the max number of requests a single client IP can make
	// in a short burst above the sustained rate.
	RateLimitBurst int `json:"RateLimitBurst"`
}

func ParseJsonConfig(path string) Config {
	jsonFile, err := os.Open(path) //nolint:gosec // path is a trusted, operator-supplied startup argument, not user input

	if err != nil {
		log.Fatal("Error when opening JSON file: ", err.Error())
	}

	defer func() { _ = jsonFile.Close() }()

	var config Config
	err = json.NewDecoder(jsonFile).Decode(&config)

	if err != nil {
		log.Fatal("Error when parsing JSON file: ", err.Error())
	}

	if err := validateConfig(&config); err != nil {
		log.Fatal("Error when validating config: ", err.Error())
	}

	return config
}

func validateConfig(config *Config) error {
	if config.Dim < 2 || config.Dim > 5 {
		return fmt.Errorf("'Dim' must be in [2, 5], got %d", config.Dim)
	}

	if len(config.ToggleSequence) != len(config.AvailableToggleSequence) {
		return fmt.Errorf("'ToggleSequence' (len %d) must match 'AvailableToggleSequence' (len %d)",
			len(config.ToggleSequence), len(config.AvailableToggleSequence))
	}

	if config.MaxSessions < 1 {
		return fmt.Errorf("'MaxSessions' must be >= 1, got %d", config.MaxSessions)
	}

	if config.SessionTTLSeconds < 1 {
		return fmt.Errorf("'SessionTTLSeconds' must be >= 1, got %d", config.SessionTTLSeconds)
	}

	if config.SessionIdleTimeoutSeconds < 1 || config.SessionIdleTimeoutSeconds > config.SessionTTLSeconds {
		return fmt.Errorf("'SessionIdleTimeoutSeconds' must be in [1, SessionTTLSeconds=%d], got %d",
			config.SessionTTLSeconds, config.SessionIdleTimeoutSeconds)
	}

	if config.SessionWaitCheckIntervalSeconds < 1 {
		return fmt.Errorf("'SessionWaitCheckIntervalSeconds' must be >= 1, got %d", config.SessionWaitCheckIntervalSeconds)
	}

	if config.LogFilePath == "" {
		return fmt.Errorf("'LogFilePath' must not be empty")
	}

	if config.LogMaxSizeMB < 1 {
		return fmt.Errorf("'LogMaxSizeMB' must be >= 1, got %d", config.LogMaxSizeMB)
	}

	if config.LogMaxBackups < 1 {
		return fmt.Errorf("'LogMaxBackups' must be >= 1, got %d", config.LogMaxBackups)
	}

	if _, err := ParseLogLevel(config.LogLevel); err != nil {
		return err
	}

	if config.RateLimitRequestsPerSecond <= 0 {
		return fmt.Errorf("'RateLimitRequestsPerSecond' must be > 0, got %v", config.RateLimitRequestsPerSecond)
	}

	if config.RateLimitBurst < 1 {
		return fmt.Errorf("'RateLimitBurst' must be >= 1, got %d", config.RateLimitBurst)
	}

	return nil
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

func valuesToJsonMap(values url.Values) map[string]interface{} {
	jsonMap := make(map[string]interface{})

	for k, v := range values {
		if len(v) == 0 {
			continue
		}
		jsonMap[k] = v
	}

	return jsonMap
}

// ProcessRequestForm reads the request's form fields. There is no failure mode: a
// missing/empty field simply results in a missing key, handled by the Parse* helpers.
func ProcessRequestForm(c echo.Context) map[string]interface{} {
	form, _ := c.FormParams()
	return valuesToJsonMap(form)
}

// ProcessRequestQuery reads the request's query string params. There is no failure
// mode: a missing/empty field simply results in a missing key, handled by the Parse*
// helpers.
func ProcessRequestQuery(c echo.Context) map[string]interface{} {
	return valuesToJsonMap(c.QueryParams())
}

// firstFormValue safely extracts the first value of a form/query field produced by
// ProcessRequestForm / ProcessRequestQuery, without panicking if the key is missing
// or holds an unexpected type (both reachable by hitting the endpoints directly).
func firstFormValue(jsonMap map[string]interface{}, key string) (string, bool) {
	raw, ok := jsonMap[key]
	if !ok {
		return "", false
	}

	values, ok := raw.([]string)
	if !ok || len(values) == 0 {
		return "", false
	}

	return values[0], true
}

// fail marks resp as an error with msg, logs it (at the call site's own function name,
// since these are client-input validation issues, not server faults), and returns msg.
func fail(resp map[string]interface{}, msg string) string {
	resp["Status"] = "ERROR"
	resp["Error"] = msg
	return msg
}

func ParseDim(jsonMap map[string]interface{}, resp map[string]interface{}) (int, map[string]interface{}) {
	raw, ok := firstFormValue(jsonMap, "dim")
	if !ok {
		slog.Warn(fail(resp, "Params error: 'dim' key missing"), FuncAttrKey, Caller())
		return -1, resp
	}

	dim, err := strconv.Atoi(raw)

	if err != nil {
		slog.Warn(fail(resp, "Params error: "+err.Error()), FuncAttrKey, Caller())
		return -1, resp
	}

	if dim < 2 || dim > 5 {
		slog.Warn(fail(resp, "Params error: dim ∈ [2, 5]"), FuncAttrKey, Caller())
		return -1, resp
	}

	return dim, resp
}

func ParseNeighborhood(jsonMap map[string]interface{}, resp map[string]interface{}) ([]int, map[string]interface{}) {
	raw, ok := jsonMap["neighborhood"]
	values, valuesOk := raw.([]string)
	if !ok || raw == nil || !valuesOk {
		slog.Warn(fail(resp, "Params error: 'neighborhood' key missing"), FuncAttrKey, Caller())
		return make([]int, 0), resp
	}

	var neighborhood = []int{}
	for _, i := range values {
		j, err := strconv.Atoi(i)
		if err != nil {
			slog.Warn(fail(resp, "Params error: "+err.Error()), FuncAttrKey, Caller())
			return make([]int, 0), resp
		}
		neighborhood = append(neighborhood, j)
	}

	if len(neighborhood) == 0 {
		slog.Warn(fail(resp, "Params error: 'neighborhood' value is empty"), FuncAttrKey, Caller())
		return make([]int, 0), resp
	}

	return neighborhood, resp
}

func ParseCheat(jsonMap map[string]interface{}, resp map[string]interface{}) (bool, map[string]interface{}) {
	cheat := false
	if raw, ok := firstFormValue(jsonMap, "cheat"); ok {
		cheatInt, err := strconv.Atoi(raw)
		if err != nil {
			slog.Warn(fail(resp, "Params error: "+err.Error()), FuncAttrKey, Caller())
			return cheat, resp
		}
		cheat = cheatInt != 0
	}

	return cheat, resp
}

func ParseRowCol(jsonMap map[string]interface{}, resp map[string]interface{}) (int, int, map[string]interface{}) {
	rowRaw, ok := firstFormValue(jsonMap, "row")
	if !ok {
		slog.Warn(fail(resp, "Params error: 'row' key missing"), FuncAttrKey, Caller())
		return -1, -1, resp
	}

	row, err := strconv.Atoi(rowRaw)

	if err != nil {
		slog.Warn(fail(resp, "Params error: "+err.Error()), FuncAttrKey, Caller())
		return -1, -1, resp
	}

	colRaw, ok := firstFormValue(jsonMap, "col")
	if !ok {
		slog.Warn(fail(resp, "Params error: 'col' key missing"), FuncAttrKey, Caller())
		return -1, -1, resp
	}

	col, err := strconv.Atoi(colRaw)

	if err != nil {
		slog.Warn(fail(resp, "Params error: "+err.Error()), FuncAttrKey, Caller())
		return -1, -1, resp
	}

	return row, col, resp
}

func UpdateStateResponse(state map[string]interface{}, resp map[string]interface{}) map[string]interface{} {
	state["Response"] = resp
	return state
}
