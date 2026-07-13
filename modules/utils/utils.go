// Package utils holds cross-cutting helpers shared by the rest of goSwitch:
// config loading/validation, structured logging setup, and request parsing.
package utils

import (
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strconv"

	"encoding/json"

	"github.com/labstack/echo/v4"
)

// supportedNeighborhoodPatterns must match the values grid.Grid.Switch understands
// (0: self, 4: orthogonal, 8: diagonal). Duplicated here rather than imported from the
// grid package to avoid a circular dependency (grid already imports utils for logging).
var supportedNeighborhoodPatterns = []int{0, 4, 8}

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
	// MaxWaitingConnections caps how many clients can hold an open /wait SSE
	// connection at once, independent of MaxSessions -- without this, a client with no
	// real session could still hold an unbounded number of idle connections open.
	MaxWaitingConnections int `json:"MaxWaitingConnections"`

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

	// TrustProxyHeaders declares whether goSwitch is running behind a reverse proxy
	// (e.g. Render's edge) that terminates TLS and sets X-Forwarded-For/-Proto. When
	// true, those headers are trusted for client-IP-based rate limiting and for marking
	// the session cookie Secure. When false (the safe default, matching a bare local
	// `go run .`), they're ignored entirely so a direct, unproxied client can't spoof
	// them. Overridable per-deployment via the GOSWITCH_TRUST_PROXY_HEADERS environment
	// variable without editing this committed file.
	TrustProxyHeaders bool `json:"TrustProxyHeaders"`
}

// trustProxyHeadersEnvVar lets a deployment (e.g. Render, which sits behind exactly the
// kind of reverse proxy TrustProxyHeaders is about) override the committed config.json's
// value without needing a separate config file per environment.
const trustProxyHeadersEnvVar = "GOSWITCH_TRUST_PROXY_HEADERS"

func ParseJSONConfig(path string) Config {
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

	if raw, set := os.LookupEnv(trustProxyHeadersEnvVar); set {
		trust, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			log.Fatalf("Error when parsing %s=%q: %v", trustProxyHeadersEnvVar, raw, parseErr)
		}
		config.TrustProxyHeaders = trust
	}

	if err := validateConfig(&config); err != nil {
		log.Fatal("Error when validating config: ", err.Error())
	}

	return config
}

func validateConfig(config *Config) error {
	if config.Port == "" {
		return fmt.Errorf("'Port' must not be empty")
	}
	// 0 is included as valid: it's the standard net.Listen convention for "let the OS
	// pick an ephemeral port", used by this project's own test suite.
	if port, err := strconv.Atoi(config.Port); err != nil || port < 0 || port > 65535 {
		return fmt.Errorf("'Port' must be a number in [0, 65535], got %q", config.Port)
	}

	if config.Dim < 2 || config.Dim > 5 {
		return fmt.Errorf("'Dim' must be in [2, 5], got %d", config.Dim)
	}

	if len(config.ToggleSequence) != len(config.AvailableToggleSequence) {
		return fmt.Errorf("'ToggleSequence' (len %d) must match 'AvailableToggleSequence' (len %d)",
			len(config.ToggleSequence), len(config.AvailableToggleSequence))
	}

	seenPatterns := make(map[int]bool, len(config.AvailableToggleSequence))
	for _, val := range config.AvailableToggleSequence {
		if !slices.Contains(supportedNeighborhoodPatterns, val) {
			return fmt.Errorf("'AvailableToggleSequence' value %d is not a supported pattern (must be one of %v)",
				val, supportedNeighborhoodPatterns)
		}
		if seenPatterns[val] {
			return fmt.Errorf("'AvailableToggleSequence' value %d is duplicated", val)
		}
		seenPatterns[val] = true
	}

	if config.SessionIdleTimeoutSeconds < 1 || config.SessionIdleTimeoutSeconds > config.SessionTTLSeconds {
		return fmt.Errorf("'SessionIdleTimeoutSeconds' must be in [1, SessionTTLSeconds=%d], got %d",
			config.SessionTTLSeconds, config.SessionIdleTimeoutSeconds)
	}

	if config.LogFilePath == "" {
		return fmt.Errorf("'LogFilePath' must not be empty")
	}

	if _, err := ParseLogLevel(config.LogLevel); err != nil {
		return err
	}

	if config.RateLimitRequestsPerSecond <= 0 {
		return fmt.Errorf("'RateLimitRequestsPerSecond' must be > 0, got %v", config.RateLimitRequestsPerSecond)
	}

	// These share the same ">= 1" shape; the checks above don't (each has its own
	// distinct condition), so they're left explicit rather than folded in here too.
	for _, field := range []struct {
		name string
		val  int
	}{
		{"MaxSessions", config.MaxSessions},
		{"SessionTTLSeconds", config.SessionTTLSeconds},
		{"SessionWaitCheckIntervalSeconds", config.SessionWaitCheckIntervalSeconds},
		{"MaxWaitingConnections", config.MaxWaitingConnections},
		{"LogMaxSizeMB", config.LogMaxSizeMB},
		{"LogMaxBackups", config.LogMaxBackups},
		{"RateLimitBurst", config.RateLimitBurst},
	} {
		if err := atLeastOne(field.name, field.val); err != nil {
			return err
		}
	}

	return nil
}

// atLeastOne returns an error if val is below 1, naming the offending field.
func atLeastOne(name string, val int) error {
	if val < 1 {
		return fmt.Errorf("'%s' must be >= 1, got %d", name, val)
	}
	return nil
}

// BuildNeighborhoodFromConfig assumes len(config.ToggleSequence) >=
// len(config.AvailableToggleSequence), which validateConfig guarantees for any Config
// that's passed validation. Callers constructing a Config by hand (bypassing
// ParseJSONConfig/validateConfig) that violate this get a neighborhood built from
// however many entries are actually available, rather than a panic.
func BuildNeighborhoodFromConfig(config *Config) []int {
	neighborhood := make([]int, 0, len(config.AvailableToggleSequence))

	for idx, val := range config.AvailableToggleSequence {
		if idx >= len(config.ToggleSequence) {
			break
		}
		if config.ToggleSequence[idx] {
			neighborhood = append(neighborhood, val)
		}
	}

	return neighborhood
}

func BuildToggleSequenceFromRequest(neighborhood []int, availableToggleSequence []int) []bool {
	togglesequence := make([]bool, 0, len(availableToggleSequence))

	for _, val := range availableToggleSequence {
		valFound := false

		for _, neigh := range neighborhood {
			valFound = val == neigh
			if valFound {
				break
			}
		}

		togglesequence = append(togglesequence, valFound)
	}

	return togglesequence
}

func valuesToJSONMap(values url.Values) map[string]interface{} {
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
	return valuesToJSONMap(form)
}

// ProcessRequestQuery reads the request's query string params. There is no failure
// mode: a missing/empty field simply results in a missing key, handled by the Parse*
// helpers.
func ProcessRequestQuery(c echo.Context) map[string]interface{} {
	return valuesToJSONMap(c.QueryParams())
}

// formValues safely extracts all values of a multi-value form/query field (e.g.
// repeated checkboxes) produced by ProcessRequestForm / ProcessRequestQuery, without
// panicking if the key is missing or holds an unexpected type (both reachable by
// hitting the endpoints directly).
func formValues(jsonMap map[string]interface{}, key string) ([]string, bool) {
	raw, ok := jsonMap[key]
	if !ok || raw == nil {
		return nil, false
	}

	values, ok := raw.([]string)
	if !ok {
		return nil, false
	}

	return values, true
}

// firstFormValue safely extracts the first value of a single-value form/query field.
func firstFormValue(jsonMap map[string]interface{}, key string) (string, bool) {
	values, ok := formValues(jsonMap, key)
	if !ok || len(values) == 0 {
		return "", false
	}

	return values[0], true
}

// OKResp returns a fresh success response, the shape every Parse* helper expects to
// start from.
func OKResp() map[string]interface{} {
	return map[string]interface{}{"Status": "SUCCESS", "Error": ""}
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

// ParseNeighborhood parses the request's 'neighborhood' values and validates each one
// is a member of availableToggleSequence (the server's configured, understood set of
// patterns), with no duplicates -- otherwise BuildToggleSequenceFromRequest's checkbox
// state and grid.NewGrid's actual board could silently diverge, or an unrecognized
// value could make the resulting board permanently unswitchable.
func ParseNeighborhood(jsonMap map[string]interface{}, resp map[string]interface{}, availableToggleSequence []int) ([]int, map[string]interface{}) {
	values, ok := formValues(jsonMap, "neighborhood")
	if !ok {
		slog.Warn(fail(resp, "Params error: 'neighborhood' key missing"), FuncAttrKey, Caller())
		return make([]int, 0), resp
	}

	seen := make(map[int]bool, len(values))
	var neighborhood = []int{}
	for _, i := range values {
		j, err := strconv.Atoi(i)
		if err != nil {
			slog.Warn(fail(resp, "Params error: "+err.Error()), FuncAttrKey, Caller())
			return make([]int, 0), resp
		}

		if !slices.Contains(availableToggleSequence, j) {
			slog.Warn(fail(resp, fmt.Sprintf("Params error: 'neighborhood' value %d is not a supported pattern", j)), FuncAttrKey, Caller())
			return make([]int, 0), resp
		}
		if seen[j] {
			slog.Warn(fail(resp, fmt.Sprintf("Params error: 'neighborhood' value %d is duplicated", j)), FuncAttrKey, Caller())
			return make([]int, 0), resp
		}
		seen[j] = true

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

// parseIntField extracts and parses a single required int field. On failure it marks
// resp as an error (via fail) and returns the failure message for the caller to log
// itself -- callers must do their own slog.Warn(msg, FuncAttrKey, Caller()) rather than
// this helper doing it, so Caller() still reports the real call site's function name.
func parseIntField(jsonMap map[string]interface{}, resp map[string]interface{}, key string) (val int, failMsg string, ok bool) {
	raw, found := firstFormValue(jsonMap, key)
	if !found {
		return 0, fail(resp, fmt.Sprintf("Params error: '%s' key missing", key)), false
	}

	val, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fail(resp, "Params error: "+err.Error()), false
	}

	return val, "", true
}

func ParseRowCol(jsonMap map[string]interface{}, resp map[string]interface{}) (int, int, map[string]interface{}) {
	row, msg, ok := parseIntField(jsonMap, resp, "row")
	if !ok {
		slog.Warn(msg, FuncAttrKey, Caller())
		return -1, -1, resp
	}

	col, msg, ok := parseIntField(jsonMap, resp, "col")
	if !ok {
		slog.Warn(msg, FuncAttrKey, Caller())
		return -1, -1, resp
	}

	return row, col, resp
}
