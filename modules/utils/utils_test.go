package utils

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func freshResp() map[string]interface{} {
	return map[string]interface{}{"Status": "SUCCESS", "Error": ""}
}

func TestParseDim(t *testing.T) {
	tests := []struct {
		name    string
		jsonMap map[string]interface{}
		wantErr bool
		want    int
	}{
		{"valid", map[string]interface{}{"dim": []string{"3"}}, false, 3},
		{"missing key", map[string]interface{}{}, true, -1},
		{"not a string slice", map[string]interface{}{"dim": 3}, true, -1},
		{"not a number", map[string]interface{}{"dim": []string{"abc"}}, true, -1},
		{"below range", map[string]interface{}{"dim": []string{"1"}}, true, -1},
		{"above range", map[string]interface{}{"dim": []string{"6"}}, true, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, resp := ParseDim(tt.jsonMap, freshResp())
			if got != tt.want {
				t.Errorf("ParseDim() = %d, want %d", got, tt.want)
			}
			isErr := resp["Status"] == "ERROR"
			if isErr != tt.wantErr {
				t.Errorf("ParseDim() error status = %v, want error = %v (resp=%v)", isErr, tt.wantErr, resp)
			}
		})
	}
}

func TestParseNeighborhood(t *testing.T) {
	available := []int{0, 4, 8}

	tests := []struct {
		name    string
		jsonMap map[string]interface{}
		wantErr bool
		want    []int
	}{
		{"valid single", map[string]interface{}{"neighborhood": []string{"4"}}, false, []int{4}},
		{"valid multiple", map[string]interface{}{"neighborhood": []string{"0", "8"}}, false, []int{0, 8}},
		{"missing key", map[string]interface{}{}, true, []int{}},
		{"nil value", map[string]interface{}{"neighborhood": nil}, true, []int{}},
		{"not a number", map[string]interface{}{"neighborhood": []string{"x"}}, true, []int{}},
		{"empty slice", map[string]interface{}{"neighborhood": []string{}}, true, []int{}},
		{"unsupported value", map[string]interface{}{"neighborhood": []string{"99"}}, true, []int{}},
		{"duplicate value", map[string]interface{}{"neighborhood": []string{"4", "4"}}, true, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, resp := ParseNeighborhood(tt.jsonMap, freshResp(), available)
			if !intSliceEqual(got, tt.want) {
				t.Errorf("ParseNeighborhood() = %v, want %v", got, tt.want)
			}
			isErr := resp["Status"] == "ERROR"
			if isErr != tt.wantErr {
				t.Errorf("ParseNeighborhood() error status = %v, want error = %v (resp=%v)", isErr, tt.wantErr, resp)
			}
		})
	}
}

func TestParseCheat(t *testing.T) {
	tests := []struct {
		name    string
		jsonMap map[string]interface{}
		wantErr bool
		want    bool
	}{
		{"absent defaults false", map[string]interface{}{}, false, false},
		{"1 is true", map[string]interface{}{"cheat": []string{"1"}}, false, true},
		{"0 is false", map[string]interface{}{"cheat": []string{"0"}}, false, false},
		{"invalid", map[string]interface{}{"cheat": []string{"x"}}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, resp := ParseCheat(tt.jsonMap, freshResp())
			if got != tt.want {
				t.Errorf("ParseCheat() = %v, want %v", got, tt.want)
			}
			isErr := resp["Status"] == "ERROR"
			if isErr != tt.wantErr {
				t.Errorf("ParseCheat() error status = %v, want error = %v (resp=%v)", isErr, tt.wantErr, resp)
			}
		})
	}
}

func TestParseRowCol(t *testing.T) {
	tests := []struct {
		name    string
		jsonMap map[string]interface{}
		wantErr bool
		wantRow int
		wantCol int
	}{
		{"valid", map[string]interface{}{"row": []string{"1"}, "col": []string{"2"}}, false, 1, 2},
		{"missing row", map[string]interface{}{"col": []string{"2"}}, true, -1, -1},
		{"missing col", map[string]interface{}{"row": []string{"1"}}, true, -1, -1},
		{"invalid row", map[string]interface{}{"row": []string{"x"}, "col": []string{"2"}}, true, -1, -1},
		{"invalid col", map[string]interface{}{"row": []string{"1"}, "col": []string{"x"}}, true, -1, -1},
		{"no params at all (regression: used to panic)", map[string]interface{}{}, true, -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col, resp := ParseRowCol(tt.jsonMap, freshResp())
			if row != tt.wantRow || col != tt.wantCol {
				t.Errorf("ParseRowCol() = (%d, %d), want (%d, %d)", row, col, tt.wantRow, tt.wantCol)
			}
			isErr := resp["Status"] == "ERROR"
			if isErr != tt.wantErr {
				t.Errorf("ParseRowCol() error status = %v, want error = %v (resp=%v)", isErr, tt.wantErr, resp)
			}
		})
	}
}

func TestBuildNeighborhoodFromConfig(t *testing.T) {
	config := &Config{
		ToggleSequence:          []bool{true, false, true},
		AvailableToggleSequence: []int{0, 4, 8},
	}

	got := BuildNeighborhoodFromConfig(config)
	want := []int{0, 8}
	if !intSliceEqual(got, want) {
		t.Errorf("BuildNeighborhoodFromConfig() = %v, want %v", got, want)
	}
}

func TestBuildToggleSequenceFromRequest(t *testing.T) {
	got := BuildToggleSequenceFromRequest([]int{0, 8}, []int{0, 4, 8})
	want := []bool{true, false, true}

	if len(got) != len(want) {
		t.Fatalf("BuildToggleSequenceFromRequest() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BuildToggleSequenceFromRequest() = %v, want %v", got, want)
		}
	}
}

func TestProcessRequestFormAndQuery(t *testing.T) {
	e := echo.New()

	form := url.Values{}
	form.Set("dim", "3")
	req := httptest.NewRequest(http.MethodPost, "/reset", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	c := e.NewContext(req, httptest.NewRecorder())

	jsonMap := ProcessRequestForm(c)
	if v, ok := firstFormValue(jsonMap, "dim"); !ok || v != "3" {
		t.Errorf("ProcessRequestForm() dim = %v, %v, want \"3\", true", v, ok)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/switch?row=1&col=2", nil)
	c2 := e.NewContext(req2, httptest.NewRecorder())

	jsonMap2 := ProcessRequestQuery(c2)
	if v, ok := firstFormValue(jsonMap2, "row"); !ok || v != "1" {
		t.Errorf("ProcessRequestQuery() row = %v, %v, want \"1\", true", v, ok)
	}
	if v, ok := firstFormValue(jsonMap2, "col"); !ok || v != "2" {
		t.Errorf("ProcessRequestQuery() col = %v, %v, want \"2\", true", v, ok)
	}
}

func TestValidateConfig(t *testing.T) {
	base := func() Config {
		return Config{
			Dim:                             3,
			ToggleSequence:                  []bool{true, true, false},
			AvailableToggleSequence:         []int{0, 4, 8},
			MaxSessions:                     10,
			SessionTTLSeconds:               1800,
			SessionIdleTimeoutSeconds:       300,
			SessionWaitCheckIntervalSeconds: 2,
			MaxWaitingConnections:           50,
			LogFilePath:                     "./logs/goswitch.log",
			LogMaxSizeMB:                    5,
			LogMaxBackups:                   5,
			LogLevel:                        "INFO",
			RateLimitRequestsPerSecond:      5,
			RateLimitBurst:                  10,
		}
	}

	if err := validateConfig(ref(base())); err != nil {
		t.Fatalf("validateConfig() on a valid config returned an error: %v", err)
	}

	tests := []struct {
		name   string
		modify func(*Config)
	}{
		{"dim too small", func(c *Config) { c.Dim = 1 }},
		{"dim too large", func(c *Config) { c.Dim = 6 }},
		{"mismatched toggle sequence length", func(c *Config) { c.ToggleSequence = []bool{true} }},
		{"zero max sessions", func(c *Config) { c.MaxSessions = 0 }},
		{"zero ttl", func(c *Config) { c.SessionTTLSeconds = 0 }},
		{"idle timeout exceeds ttl", func(c *Config) { c.SessionIdleTimeoutSeconds = c.SessionTTLSeconds + 1 }},
		{"zero idle timeout", func(c *Config) { c.SessionIdleTimeoutSeconds = 0 }},
		{"zero wait check interval", func(c *Config) { c.SessionWaitCheckIntervalSeconds = 0 }},
		{"zero max waiting connections", func(c *Config) { c.MaxWaitingConnections = 0 }},
		{"unsupported available toggle sequence value", func(c *Config) { c.AvailableToggleSequence = []int{0, 4, 99} }},
		{"empty log file path", func(c *Config) { c.LogFilePath = "" }},
		{"zero log max size", func(c *Config) { c.LogMaxSizeMB = 0 }},
		{"zero log max backups", func(c *Config) { c.LogMaxBackups = 0 }},
		{"invalid log level", func(c *Config) { c.LogLevel = "VERBOSE" }},
		{"zero rate limit", func(c *Config) { c.RateLimitRequestsPerSecond = 0 }},
		{"zero rate limit burst", func(c *Config) { c.RateLimitBurst = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.modify(&c)
			if err := validateConfig(&c); err == nil {
				t.Errorf("validateConfig() with %s did not return an error", tt.name)
			}
		})
	}
}

func TestParseJSONConfigValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	content := `{
		"Port": "10000",
		"Cheat": false,
		"Dim": 3,
		"ToggleSequence": [true, true, false],
		"AvailableToggleSequence": [0, 4, 8],
		"MaxSessions": 10,
		"SessionTTLSeconds": 1800,
		"SessionIdleTimeoutSeconds": 300,
		"SessionWaitCheckIntervalSeconds": 2,
		"MaxWaitingConnections": 50,
		"LogFilePath": "./logs/goswitch.log",
		"LogMaxSizeMB": 5,
		"LogMaxBackups": 5,
		"LogLevel": "INFO",
		"RateLimitRequestsPerSecond": 5,
		"RateLimitBurst": 10
	}`

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	config := ParseJSONConfig(path)

	if config.Port != "10000" || config.Dim != 3 || config.MaxSessions != 10 {
		t.Errorf("ParseJSONConfig() = %+v, unexpected values", config)
	}
}

func ref(c Config) *Config {
	return &c
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
