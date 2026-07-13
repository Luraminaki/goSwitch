package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	utils "goSwitch/modules/utils"
	webapp "goSwitch/modules/webapp"
)

// shutdownTimeout bounds how long in-flight requests get to finish once a
// shutdown signal arrives, before the server is forced closed.
//
// This is best-effort for an open /wait SSE connection specifically: webapp.Wait only
// returns early on its own request context being canceled (the client disconnecting) or
// on a session becoming available at its SessionWaitCheckIntervalSeconds ticker -- it
// does not watch this shutdown deadline itself, and Echo's graceful Shutdown does not
// cancel in-flight request contexts, only stop accepting new ones and wait for existing
// ones to finish. So if SessionWaitCheckIntervalSeconds is configured larger than
// shutdownTimeout, a currently-waiting client can keep its connection open past
// shutdownTimeout (Shutdown then just returns a logged error while the process exits
// anyway, which closes the socket at the OS level). Keeping
// SessionWaitCheckIntervalSeconds well under this value avoids relying on that.
const shutdownTimeout = 10 * time.Second

// defaultVersion is shown if the embedded VERSION file is ever empty, so the frontend
// badge never silently renders as a bare "v".
const defaultVersion = "dev"

//go:embed VERSION
var rawVersion string

// version is embedded from the repo-root VERSION file -- the single source of truth
// bumped by hand at release time; the CHANGELOG and the in-app badge both trace back
// to it instead of each carrying their own copy of the number. Trimmed once here
// (embedded files commonly end in a trailing newline) so nothing downstream has to
// repeat that or risk reading the untrimmed rawVersion directly.
var version = trimmedVersion()

func trimmedVersion() string {
	v := strings.TrimSpace(rawVersion)
	if v == "" {
		return defaultVersion
	}
	return v
}

func main() {
	wx := webapp.NewWebApp("./config.json")
	wx.Version = version

	wx.Server.POST("/reset", wx.Reset)
	wx.Server.POST("/switch", wx.Switch)
	wx.Server.POST("/revert", wx.RevertMove)
	wx.Server.GET("/wait", wx.Wait)
	wx.Server.GET("/", wx.InitHTMX)

	// Buffered so the goroutine can always send, whether main() is still waiting on it
	// (a Start failure) or has already moved on to a normal signal-triggered shutdown.
	serveErr := make(chan error, 1)
	go func() {
		err := wx.Server.Start(":" + wx.Config.Port)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Routed through slog (not wx.Server.Logger.Fatal/Error) so both a Start failure
	// and a Shutdown error land in the same rotating log file operators actually watch,
	// instead of only on stdout -- and, critically, so a Start failure still reaches the
	// cleanup below rather than Logger.Fatal's immediate os.Exit skipping it entirely.
	startFailed := false
	select {
	case <-ctx.Done():
		slog.Info("Shutting down...", utils.FuncAttrKey, utils.Caller())
	case err := <-serveErr:
		startFailed = err != nil
		if startFailed {
			slog.Error(fmt.Sprintf("server failed to start: %v", err), utils.FuncAttrKey, utils.Caller())
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := wx.Server.Shutdown(shutdownCtx); err != nil {
		slog.Error(fmt.Sprintf("error during shutdown: %v", err), utils.FuncAttrKey, utils.Caller())
	}

	if err := wx.LogCloser.Close(); err != nil {
		slog.Error(fmt.Sprintf("failed to close log file: %v", err), utils.FuncAttrKey, utils.Caller())
	}

	if startFailed {
		os.Exit(1)
	}
}
