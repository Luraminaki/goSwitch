package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	utils "goSwitch/modules/utils"
	webapp "goSwitch/modules/webapp"
)

// These are integration tests: they run from the repo root (Go places a test
// binary's working directory at its package's source directory, and main.go lives
// at the repo root), so the app's relative "webui/*.html" template/asset paths
// resolve exactly as they do in production. Only config.json is pointed at a
// per-test temp file so tests can vary MaxSessions/timeouts without touching the
// real one.

func newTestConfigFile(t *testing.T, override func(*utils.Config)) string {
	t.Helper()

	dir := t.TempDir()

	config := utils.Config{
		Port:                            "0",
		Cheat:                           false,
		Dim:                             3,
		ToggleSequence:                  []bool{true, true, false},
		AvailableToggleSequence:         []int{0, 4, 8},
		MaxSessions:                     10,
		SessionTTLSeconds:               1800,
		SessionIdleTimeoutSeconds:       300,
		SessionWaitCheckIntervalSeconds: 2,
		LogFilePath:                     filepath.Join(dir, "test.log"),
		LogMaxSizeMB:                    5,
		LogMaxBackups:                   5,
		LogLevel:                        "DEBUG",
		// Generous by default so ordinary tests firing several quick requests never
		// get throttled; TestRateLimitBlocksExcessRequests overrides this deliberately.
		RateLimitRequestsPerSecond: 1000,
		RateLimitBurst:             1000,
	}

	if override != nil {
		override(&config)
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}

	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	return path
}

func newTestServer(t *testing.T, override func(*utils.Config)) *httptest.Server {
	t.Helper()

	wx := webapp.NewWebApp(newTestConfigFile(t, override))
	wx.Server.POST("/reset", wx.Reset)
	wx.Server.POST("/switch", wx.Switch)
	wx.Server.GET("/revert", wx.RevertMove)
	wx.Server.GET("/wait", wx.Wait)
	wx.Server.GET("/", wx.InitHTMX)

	srv := httptest.NewServer(wx.Server)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { _ = wx.LogCloser.Close() })

	return srv
}

func newClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}

	return &http.Client{Jar: jar, Timeout: 10 * time.Second}
}

func mustGet(t *testing.T, client *http.Client, url string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to build GET %s request: %v", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body failed: %v", err)
	}

	return resp.StatusCode, string(body)
}

func mustPostForm(t *testing.T, client *http.Client, rawURL string, form url.Values) (int, string) {
	t.Helper()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, rawURL, body)
	if err != nil {
		t.Fatalf("failed to build POST %s request: %v", rawURL, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s failed: %v", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body failed: %v", err)
	}

	return resp.StatusCode, string(respBody)
}

func TestFullGamePlayFlow(t *testing.T) {
	srv := newTestServer(t, nil)
	client := newClient(t)

	status, body := mustGet(t, client, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", status)
	}
	if !strings.Contains(body, "Sessions: 1/10") {
		t.Fatalf("GET / body missing session badge, got: %s", body)
	}

	status, body = mustPostForm(t, client, srv.URL+"/switch?row=0&col=0", nil)
	if status != http.StatusOK {
		t.Fatalf("POST /switch = %d, want 200", status)
	}
	if !strings.Contains(body, "[0]") {
		t.Fatalf("POST /switch did not record move 0 in history, got: %s", body)
	}

	form := url.Values{}
	form.Set("dim", "4")
	form.Add("neighborhood", "0")
	form.Add("neighborhood", "4")
	form.Set("cheat", "1")
	status, body = mustPostForm(t, client, srv.URL+"/reset", form)
	if status != http.StatusOK {
		t.Fatalf("POST /reset = %d, want 200", status)
	}
	if !strings.Contains(body, `value="4"`) {
		t.Fatalf("POST /reset did not apply the new Dim=4, got: %s", body)
	}

	status, body = mustGet(t, client, srv.URL+"/revert")
	if status != http.StatusOK {
		t.Fatalf("GET /revert = %d, want 200", status)
	}
	if !strings.Contains(body, "Not allowed") {
		t.Fatalf("GET /revert after a reset should report nothing to revert, got: %s", body)
	}
}

func TestPerSessionIsolation(t *testing.T) {
	srv := newTestServer(t, nil)
	clientA := newClient(t)
	clientB := newClient(t)

	mustGet(t, clientA, srv.URL+"/")
	mustGet(t, clientB, srv.URL+"/")

	mustPostForm(t, clientA, srv.URL+"/switch?row=0&col=0", nil)
	mustPostForm(t, clientB, srv.URL+"/switch?row=2&col=2", nil)

	_, bodyA := mustGet(t, clientA, srv.URL+"/")
	_, bodyB := mustGet(t, clientB, srv.URL+"/")

	if !strings.Contains(bodyA, "[0]") || strings.Contains(bodyA, "[8]") {
		t.Fatalf("client A's history should contain only its own move [0], got: %s", bodyA)
	}
	if !strings.Contains(bodyB, "[8]") || strings.Contains(bodyB, "[0]") {
		t.Fatalf("client B's history should contain only its own move [8], got: %s", bodyB)
	}
	if !strings.Contains(bodyA, "Sessions: 2/10") {
		t.Fatalf("expected the session badge to read 2/10, got: %s", bodyA)
	}
}

// TestMalformedRequestsDontCrash is a regression test for a fixed bug where hitting
// /switch or /reset without the expected params paniced with no recover middleware,
// killing the whole server process.
func TestMalformedRequestsDontCrash(t *testing.T) {
	srv := newTestServer(t, nil)
	client := newClient(t)

	status, _ := mustPostForm(t, client, srv.URL+"/switch", nil)
	if status != http.StatusOK {
		t.Fatalf("POST /switch with no params = %d, want 200 (graceful error, not a crash)", status)
	}

	status, _ = mustPostForm(t, client, srv.URL+"/reset", nil)
	if status != http.StatusOK {
		t.Fatalf("POST /reset with no params = %d, want 200 (graceful error, not a crash)", status)
	}

	// The server must still be responsive afterward.
	status, _ = mustGet(t, client, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("server did not survive malformed requests: GET / = %d", status)
	}
}

// TestResetErrorIsEscaped is a regression test for a fixed reflected-XSS bug: error
// messages that echo back user input must be HTML-escaped, not raw.
func TestResetErrorIsEscaped(t *testing.T) {
	srv := newTestServer(t, nil)
	client := newClient(t)

	mustGet(t, client, srv.URL+"/")

	form := url.Values{}
	form.Set("dim", "<script>alert(1)</script>")
	_, body := mustPostForm(t, client, srv.URL+"/reset", form)

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("response contains an unescaped script tag: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("response does not contain the expected escaped form: %s", body)
	}
}

func TestCapacityAndSSEWait(t *testing.T) {
	srv := newTestServer(t, func(c *utils.Config) {
		c.MaxSessions = 1
		c.SessionIdleTimeoutSeconds = 1
		c.SessionWaitCheckIntervalSeconds = 1
	})

	clientA := newClient(t)
	clientB := newClient(t)

	_, bodyA := mustGet(t, clientA, srv.URL+"/")
	if !strings.Contains(bodyA, "Sessions: 1/1") {
		t.Fatalf("client A should hold the only slot, got: %s", bodyA)
	}

	_, bodyB := mustGet(t, clientB, srv.URL+"/")
	if !strings.Contains(bodyB, "All Tables Are Busy") {
		t.Fatalf("client B should be waiting (capacity full), got: %s", bodyB)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/wait", nil)
	if err != nil {
		t.Fatalf("failed to build /wait request: %v", err)
	}

	resp, err := clientB.Do(req)
	if err != nil {
		t.Fatalf("GET /wait failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /wait response failed (did it never send the ready event?): %v", err)
	}

	if !strings.Contains(string(body), "event: ready") {
		t.Fatalf("expected an SSE 'ready' event once client A's idle timeout elapsed, got: %s", body)
	}

	// Client B should now be able to load a live game.
	_, bodyB = mustGet(t, clientB, srv.URL+"/")
	if strings.Contains(bodyB, "All Tables Are Busy") {
		t.Fatalf("client B should have a session after the wait resolved, got: %s", bodyB)
	}
}

func TestRateLimitBlocksExcessRequests(t *testing.T) {
	srv := newTestServer(t, func(c *utils.Config) {
		c.RateLimitRequestsPerSecond = 1
		c.RateLimitBurst = 1
	})
	client := newClient(t)

	sawOK := false
	sawThrottled := false

	for i := 0; i < 20 && !sawThrottled; i++ {
		status, _ := mustGet(t, client, srv.URL+"/")
		switch status {
		case http.StatusOK:
			sawOK = true
		case http.StatusTooManyRequests:
			sawThrottled = true
		default:
			t.Fatalf("GET / = %d, want 200 or 429", status)
		}
	}

	if !sawOK {
		t.Fatal("expected at least one request to succeed before throttling kicked in")
	}
	if !sawThrottled {
		t.Fatal("expected the rate limiter to eventually respond with 429 Too Many Requests")
	}
}

// TestSessionExpiryNotice exercises the UX gap where a session gets silently purged
// while its owner is away: the owner should be told, not just handed a blank fresh
// board with no explanation.
func TestSessionExpiryNotice(t *testing.T) {
	srv := newTestServer(t, func(c *utils.Config) {
		c.MaxSessions = 1
		c.SessionIdleTimeoutSeconds = 1
	})

	clientA := newClient(t)
	clientB := newClient(t)

	_, bodyA := mustGet(t, clientA, srv.URL+"/")
	if strings.Contains(bodyA, "SYSTEM MESSAGE") {
		t.Fatalf("a first-ever visit should not show an expiry notice, got: %s", bodyA)
	}

	time.Sleep(1500 * time.Millisecond) // let client A's session go idle

	// A second client claims the only slot, evicting A's now-idle session.
	_, bodyB := mustGet(t, clientB, srv.URL+"/")
	if strings.Contains(bodyB, "SYSTEM MESSAGE") {
		t.Fatalf("a brand new client should never see an expiry notice, got: %s", bodyB)
	}

	time.Sleep(1500 * time.Millisecond) // let client B's session go idle too

	// Client A comes back with its now-stale cookie; its old session is gone, so it
	// gets a fresh one and should be told about it.
	_, bodyA = mustGet(t, clientA, srv.URL+"/")
	if !strings.Contains(bodyA, "SYSTEM MESSAGE") {
		t.Fatalf("client A should see the expiry notice after its session was purged, got: %s", bodyA)
	}
}
