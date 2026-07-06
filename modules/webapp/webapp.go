package webapp

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

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

	// LogCloser releases the rotating log file's handle. Production code doesn't
	// need to call it (the process holds it for its whole lifetime), but callers
	// that need the log file removable afterward -- e.g. tests cleaning up a temp
	// directory -- should Close() it once done.
	LogCloser io.Closer
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
	config := utils.ParseJsonConfig(configPath)
	logCloser := utils.SetupLogging(&config)

	server := echo.New()
	server.Use(middleware.Recover())
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
// once a slot frees up.
func (wx *WebAppX) resolveSession(c echo.Context) (*session.Session, bool) {
	id := ""
	if cookie, err := c.Cookie(sessionCookieName); err == nil {
		id = cookie.Value
	}
	if id == "" {
		id = session.NewID()
	}

	sess, ok := wx.Sessions.Claim(id)

	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   wx.Config.SessionTTLSeconds,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return sess, ok
}

func (wx *WebAppX) gameState(sess *session.Session) map[string]interface{} {
	resp := map[string]interface{}{
		"Status": "SUCCESS",
		"Error":  "",
	}

	return map[string]interface{}{
		"Config": configView{
			Dim:                     sess.Dim,
			Cheat:                   sess.Cheat,
			ToggleSequence:          sess.ToggleSequence,
			AvailableToggleSequence: wx.Config.AvailableToggleSequence,
		},
		"Board":        sess.Game.GetGrid(),
		"Solution":     sess.Game.GetPossibleSolution(),
		"Moves":        sess.Game.GetPreviousMoves(),
		"Win":          sess.Game.CheckWin(),
		"Response":     resp,
		"Waiting":      false,
		"SessionCount": wx.Sessions.Count(),
		"MaxSessions":  wx.Config.MaxSessions,
	}
}

func (wx *WebAppX) waitState() map[string]interface{} {
	return map[string]interface{}{
		"Waiting":      true,
		"SessionCount": wx.Sessions.Count(),
		"MaxSessions":  wx.Config.MaxSessions,
	}
}

// renderSession locks sess, snapshots its state (merged with resp), renders and unlocks.
func (wx *WebAppX) renderSession(c echo.Context, sess *session.Session, resp map[string]interface{}) error {
	sess.Lock()
	state := utils.UpdateStateResponse(wx.gameState(sess), resp)
	sess.Unlock()

	return c.Render(http.StatusOK, "index", state)
}

func (wx *WebAppX) InitHTMX(c echo.Context) error {
	line := utils.Trace()

	sess, ok := wx.resolveSession(c)
	if !ok {
		log.Printf("%s Client waiting for a session slot", line)
		return c.Render(http.StatusOK, "index", wx.waitState())
	}

	log.Printf("%s Serving session %s", line, sess.ID)

	sess.Lock()
	state := wx.gameState(sess)
	sess.Unlock()

	return c.Render(http.StatusOK, "index", state)
}

func (wx *WebAppX) Reset(c echo.Context) error {
	line := utils.Trace()

	sess, ok := wx.resolveSession(c)
	if !ok {
		return c.Render(http.StatusOK, "index", wx.waitState())
	}

	jsonMap := utils.ProcessRequestForm(c)
	resp := map[string]interface{}{"Status": "SUCCESS", "Error": ""}

	log.Printf("%s Data recieved: %v\n", line, jsonMap)

	dim, resp := utils.ParseDim(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, resp)
	}

	neighborhood, resp := utils.ParseNeighborhood(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, resp)
	}

	cheat, resp := utils.ParseCheat(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, resp)
	}

	sess.Lock()

	sess.Dim = dim
	sess.ToggleSequence = utils.BuildToggleSequenceFromRequest(neighborhood, wx.Config.AvailableToggleSequence)
	sess.Cheat = cheat

	sess.Game = grid.NewGrid(dim, neighborhood)
	log.Printf("%s Possible solution: %v\n", line, sess.Game.GetPossibleSolution())
	sess.Game.PrettyPrintGrid()

	state := utils.UpdateStateResponse(wx.gameState(sess), resp)
	sess.Unlock()

	return c.Render(http.StatusOK, "index", state)
}

func (wx *WebAppX) RevertMove(c echo.Context) error {
	line := utils.Trace()

	sess, ok := wx.resolveSession(c)
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
		resp["Status"] = "ERROR"
		resp["Error"] = "Not allowed: Nothing to revert to"

		log.Printf("%s %s", line, resp["Error"])

		return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(sess), resp))
	}

	sess.Game.Switch(moves[len(moves)-1])
	moves = moves[:len(moves)-1]
	sess.Game.SetPreviousMoves(moves)

	log.Printf("%s Move History: %v\n", line, moves)
	sess.Game.PrettyPrintGrid()

	return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(sess), resp))
}

func (wx *WebAppX) Switch(c echo.Context) error {
	line := utils.Trace()

	sess, ok := wx.resolveSession(c)
	if !ok {
		return c.Render(http.StatusOK, "index", wx.waitState())
	}

	jsonMap := utils.ProcessRequestQuery(c)
	resp := map[string]interface{}{"Status": "SUCCESS", "Error": ""}

	log.Printf("%s Data recieved: %v\n", line, jsonMap)

	row, col, resp := utils.ParseRowCol(jsonMap, resp, line)
	if resp["Status"] == "ERROR" {
		return wx.renderSession(c, sess, resp)
	}

	sess.Lock()
	defer sess.Unlock()

	pos := (sess.Game.Dim * row) + col

	sess.Game.Switch(pos)
	moves := sess.Game.GetPreviousMoves()
	moves = append(moves, pos)
	sess.Game.SetPreviousMoves(moves)

	log.Printf("%s Move History: %v\n", line, moves)
	sess.Game.PrettyPrintGrid()

	return c.Render(http.StatusOK, "index", utils.UpdateStateResponse(wx.gameState(sess), resp))
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
			sess, ok := wx.Sessions.Claim(id)
			if !ok {
				continue
			}

			sess.Lock()
			state := wx.gameState(sess)
			sess.Unlock()

			var buf bytes.Buffer
			if err := c.Echo().Renderer.Render(&buf, "game", state, c); err != nil {
				return err
			}

			writeSSEEvent(resp, "ready", buf.String())
			resp.Flush()

			return nil
		}
	}
}

func writeSSEEvent(w io.Writer, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
