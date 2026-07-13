// Package webapp wires goSwitch's HTTP handlers, session resolution, and
// templates together into the echo server that main.go starts.
package webapp

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	grid "goSwitch/modules/grid"
	session "goSwitch/modules/session"
	template "goSwitch/modules/template"
	utils "goSwitch/modules/utils"
)

const sessionCookieName = "goswitch_sid"

// STRUCTS

type WebAppX struct {
	Config   *utils.Config
	Sessions *session.Manager
	Server   *echo.Echo
	Version  string

	// LogCloser releases the rotating log file's handle. Production code doesn't
	// need to call it (the process holds it for its whole lifetime), but callers
	// that need the log file removable afterward -- e.g. tests cleaning up a temp
	// directory -- should Close() it once done.
	LogCloser io.Closer

	// waitingConns counts clients currently parked in Wait(), independent of and
	// uncapped by MaxSessions -- without this, a client that never gets (or doesn't
	// even have) a real session could still hold an unbounded number of open SSE
	// connections/goroutines.
	waitingConns atomic.Int32
}

// configView adapts a session's live game settings plus the app-wide list of
// available patterns into the shape the existing templates expect at .Config.
type configView struct {
	Dim                     int
	Cheat                   bool
	ToggleSequence          []bool
	AvailableToggleSequence []int
}

// WebApp

func NewWebApp(configPath string) *WebAppX {
	config := utils.ParseJSONConfig(configPath)
	logCloser := utils.SetupLogging(&config)

	server := echo.New()
	// Echo's default RealIP() trusts X-Forwarded-For unconditionally, which lets any
	// direct client spoof its way around the per-IP rate limiter below. Only trust it
	// when explicitly told we're behind a real reverse proxy (Config.TrustProxyHeaders);
	// ExtractIPFromXFFHeader's defaults (trust loopback/link-local/private-net) are
	// exactly right for a PaaS edge proxy on a private network, e.g. Render's.
	if config.TrustProxyHeaders {
		server.IPExtractor = echo.ExtractIPFromXFFHeader()
	} else {
		server.IPExtractor = echo.ExtractIPDirect()
	}
	server.Use(middleware.Recover())
	server.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:  rate.Limit(config.RateLimitRequestsPerSecond),
			Burst: config.RateLimitBurst,
		},
	)))
	server.File("/favicon.ico", "webui/favicon.ico")
	server.File("/assets/style.css", "webui/assets/style.css")
	server.File("/assets/htmx.min.js", "webui/assets/htmx.min.js")
	server.File("/assets/sse.min.js", "webui/assets/sse.min.js")

	template.NewTemplateRenderer(server, "webui/*.html")

	webApp := &WebAppX{
		Config:    &config,
		Sessions:  session.NewManager(&config),
		Server:    server,
		LogCloser: logCloser,
	}

	return webApp
}

// resolveSession maps the incoming request to its session, minting a new candidate ID
// when the client has none. The cookie is always (re)written, even when the manager is
// at capacity, so a waiting client's later /wait SSE connection can claim the same ID
// once a slot frees up. expired reports whether the client presented a cookie for a
// session that no longer exists (it was purged for TTL/idle-timeout under capacity
// pressure) and got a brand new one instead -- worth telling them, since otherwise
// their board just silently resets with no explanation.
func (wx *WebAppX) resolveSession(c echo.Context) (sess *session.Session, ok bool, expired bool) {
	hadCookie := false
	id := ""
	if cookie, err := c.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		id = cookie.Value
		hadCookie = true
	}
	if id == "" {
		id = session.NewID()
	}

	sess, ok, existed := wx.Sessions.Claim(id)
	expired = hadCookie && ok && !existed

	// MaxAge reflects the session's actual remaining server-side TTL (from creation),
	// not a fixed value re-rolled on every request -- otherwise the cookie's client-
	// visible lifetime keeps looking freshly-reset forever, even as the session
	// approaches its real, unmoving eviction deadline. Falls back to the full TTL when
	// there's no session yet (the waiting-room case), matching the config's own default.
	maxAge := wx.Config.SessionTTLSeconds
	if ok {
		maxAge = int(wx.Sessions.SessionMaxAge(sess).Seconds())
	}

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	// c.Scheme() trusts the X-Forwarded-Proto header, which is only safe to rely on
	// behind a real reverse proxy (Config.TrustProxyHeaders) -- otherwise a direct
	// client could set that header itself and force Secure false on a real TLS
	// connection. Without a trusted proxy, fall back to checking whether TLS is
	// actually terminated in this process, which can't be spoofed by a header.
	secure := c.Request().TLS != nil
	if wx.Config.TrustProxyHeaders {
		secure = c.Scheme() == "https"
	}
	if secure {
		cookie.Secure = true
	}
	c.SetCookie(cookie)

	return sess, ok, expired
}

// baseState holds the keys every rendered page needs regardless of whether a client
// has a live session or is waiting for one, so gameState and waitState can't drift on
// them independently.
func (wx *WebAppX) baseState() map[string]interface{} {
	return map[string]interface{}{
		"SessionCount": wx.Sessions.Count(),
		"MaxSessions":  wx.Config.MaxSessions,
		"Version":      wx.Version,
	}
}

func (wx *WebAppX) gameState(sess *session.Session, expired bool) map[string]interface{} {
	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	state := wx.baseState()
	state["Config"] = configView{
		Dim:                     sess.Dim,
		Cheat:                   sess.Cheat,
		ToggleSequence:          sess.ToggleSequence,
		AvailableToggleSequence: wx.Config.AvailableToggleSequence,
	}
	state["Board"] = sess.Game.GetGrid()
	state["Solution"] = sess.Game.GetPossibleSolution()
	state["Moves"] = sess.Game.GetPreviousMoves()
	state["Win"] = sess.Game.CheckWin()
	state["Response"] = resp
	state["Waiting"] = false
	state["Expired"] = expired

	return state
}

func (wx *WebAppX) waitState() map[string]interface{} {
	state := wx.baseState()
	state["Waiting"] = true

	return state
}

// renderSession locks sess, snapshots its state (merged with resp), renders and unlocks.
func (wx *WebAppX) renderSession(c echo.Context, sess *session.Session, expired bool, resp map[string]interface{}) error {
	sess.Lock()
	state := utils.UpdateStateResponse(wx.gameState(sess, expired), resp)
	sess.Unlock()

	return c.Render(http.StatusOK, "index", state)
}

func (wx *WebAppX) InitHTMX(c echo.Context) error {
	sess, ok, expired := wx.resolveSession(c)
	if !ok {
		slog.Info("Client waiting for a session slot", utils.FuncAttrKey, utils.Caller())
		return c.Render(http.StatusOK, "index", wx.waitState())
	}

	// Debug, not Info: the session ID is the only value gating access to that session's
	// game state, so it shouldn't land in logs at a level that's likely to be enabled
	// (and read/retained) in a production deployment.
	slog.Debug(fmt.Sprintf("Serving session %s", sess.ID), utils.FuncAttrKey, utils.Caller())

	sess.Lock()
	state := wx.gameState(sess, expired)
	sess.Unlock()

	return c.Render(http.StatusOK, "index", state)
}

func (wx *WebAppX) Reset(c echo.Context) error {
	sess, ok, expired := wx.resolveSession(c)
	if !ok {
		return c.Render(http.StatusOK, "index", wx.waitState())
	}

	jsonMap := utils.ProcessRequestForm(c)
	resp := map[string]interface{}{"Status": "SUCCESS", "Error": ""}

	slog.Debug(fmt.Sprintf("Data received: %v", jsonMap), utils.FuncAttrKey, utils.Caller())

	dim, resp := utils.ParseDim(jsonMap, resp)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, expired, resp)
	}

	neighborhood, resp := utils.ParseNeighborhood(jsonMap, resp, wx.Config.AvailableToggleSequence)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, expired, resp)
	}

	cheat, resp := utils.ParseCheat(jsonMap, resp)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, expired, resp)
	}

	sess.Lock()

	sess.Dim = dim
	sess.ToggleSequence = utils.BuildToggleSequenceFromRequest(neighborhood, wx.Config.AvailableToggleSequence)
	sess.Cheat = cheat

	sess.Game = grid.NewGrid(dim, neighborhood)
	slog.Debug(fmt.Sprintf("Possible solution: %v", sess.Game.GetPossibleSolution()), utils.FuncAttrKey, utils.Caller())
	sess.Game.PrettyPrintGrid()

	state := utils.UpdateStateResponse(wx.gameState(sess, expired), resp)
	sess.Unlock()

	return c.Render(http.StatusOK, "index", state)
}

func (wx *WebAppX) RevertMove(c echo.Context) error {
	sess, ok, expired := wx.resolveSession(c)
	if !ok {
		return c.Render(http.StatusOK, "index", wx.waitState())
	}

	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	sess.Lock()
	defer sess.Unlock()

	moves := sess.Game.GetPreviousMoves()
	if moves == nil {
		const errMsg = "Not allowed: Nothing to revert to"
		resp["Status"] = "ERROR"
		resp["Error"] = errMsg

		slog.Info(errMsg, utils.FuncAttrKey, utils.Caller())

		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(sess, expired), resp))
	}

	sess.Game.Switch(moves[len(moves)-1])
	moves = moves[:len(moves)-1]
	sess.Game.SetPreviousMoves(moves)

	slog.Debug(fmt.Sprintf("Move History: %v", moves), utils.FuncAttrKey, utils.Caller())
	sess.Game.PrettyPrintGrid()

	return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(sess, expired), resp))
}

func (wx *WebAppX) Switch(c echo.Context) error {
	sess, ok, expired := wx.resolveSession(c)
	if !ok {
		return c.Render(http.StatusOK, "index", wx.waitState())
	}

	jsonMap := utils.ProcessRequestQuery(c)
	resp := map[string]interface{}{"Status": "SUCCESS", "Error": ""}

	slog.Debug(fmt.Sprintf("Data received: %v", jsonMap), utils.FuncAttrKey, utils.Caller())

	row, col, resp := utils.ParseRowCol(jsonMap, resp)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, expired, resp)
	}

	sess.Lock()
	defer sess.Unlock()

	// Bounds-checked here (rather than in ParseRowCol) since the valid range depends
	// on this session's current board size, which isn't known/lockable until now.
	if row < 0 || row >= sess.Game.Dim || col < 0 || col >= sess.Game.Dim {
		const errMsg = "Params error: row/col out of bounds for the current board"
		resp["Status"] = "ERROR"
		resp["Error"] = errMsg

		slog.Warn(errMsg, utils.FuncAttrKey, utils.Caller())

		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(sess, expired), resp))
	}

	pos := (sess.Game.Dim * row) + col

	sess.Game.Switch(pos)
	moves := sess.Game.GetPreviousMoves()
	moves = append(moves, pos)
	sess.Game.SetPreviousMoves(moves)

	slog.Debug(fmt.Sprintf("Move History: %v", moves), utils.FuncAttrKey, utils.Caller())
	sess.Game.PrettyPrintGrid()

	return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(sess, expired), resp))
}

// Wait serves an SSE stream for a client that couldn't get a session slot. It rechecks
// at SessionWaitCheckIntervalSeconds and, once a slot frees up for this client's ID,
// pushes a single "ready" event containing the rendered game fragment, then closes.
func (wx *WebAppX) Wait(c echo.Context) error {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	id := cookie.Value

	if wx.waitingConns.Add(1) > int32(wx.Config.MaxWaitingConnections) { //nolint:gosec // MaxWaitingConnections is validated >= 1 at startup, never near int32's range
		wx.waitingConns.Add(-1)
		slog.Warn("Wait -- rejected: too many concurrent waiting connections", utils.FuncAttrKey, utils.Caller())
		return c.NoContent(http.StatusServiceUnavailable)
	}
	defer wx.waitingConns.Add(-1)

	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.WriteHeader(http.StatusOK)

	interval := time.Duration(wx.Config.SessionWaitCheckIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ctx := c.Request().Context()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			sess, ok, _ := wx.Sessions.Claim(id)
			if !ok {
				continue
			}

			sess.Lock()
			state := wx.gameState(sess, false)
			sess.Unlock()

			var buf bytes.Buffer
			if err := c.Echo().Renderer.Render(&buf, "game", state, c); err != nil {
				return err
			}

			if err := writeSSEEvent(resp, "ready", buf.String()); err != nil {
				slog.Warn(fmt.Sprintf("Wait -- failed writing SSE event (client likely disconnected): %v", err), utils.FuncAttrKey, utils.Caller())
				return nil
			}
			resp.Flush()

			return nil
		}
	}
}

func writeSSEEvent(w io.Writer, event, data string) error {
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	for _, line := range strings.Split(data, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}
