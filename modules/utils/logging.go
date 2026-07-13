package utils

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

// loggerName identifies this app in every log line, mirroring Python's %(name)s
// (there's only one process-wide logger here, so a fixed name is enough).
const loggerName = "goSwitch"

// FuncAttrKey is the slog attribute key call sites use to report their own function
// name, e.g. slog.Info(msg, utils.FuncAttrKey, utils.Caller()). slog.Record.PC is
// deliberately not used for this: its capture depth is calibrated for the exact
// slog.Info/Warn/etc call shape, and inlining of those tiny wrapper functions has
// been observed to throw it off, landing on an internal log/slog frame instead of
// the real caller.
const FuncAttrKey = "func"

// Caller returns the calling function's fully-qualified name, for passing as the
// "func" attribute to slog calls, e.g. slog.Info(msg, "func", utils.Caller()).
func Caller() string {
	pc := make([]uintptr, 1)
	n := runtime.Callers(2, pc) // skip [Callers, Caller itself] -> Caller's own caller
	if n == 0 {
		return "?"
	}

	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame.Function
}

// prettyHandler renders slog records as:
//
//	[2006-01-02 15:04:05,000] [pid] [name] [LEVEL]: funcName -- message
//
// matching the Python `logging` format
// "[%(asctime)s] [%(process)s] [%(name)s] [%(levelname)s]: %(funcName)s -- %(message)s".
type prettyHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	name  string
	pid   int
	level slog.Level

	// attrs are bound via WithAttrs (e.g. slog.Default().With(...)); their keys are
	// already group-qualified at bind time. groupPrefix is the dot-joined chain of
	// WithGroup names still open, applied to attrs passed directly to a log call.
	attrs       []slog.Attr
	groupPrefix string
}

func newPrettyHandler(w io.Writer, name string, level slog.Level) *prettyHandler {
	return &prettyHandler{
		mu:    &sync.Mutex{},
		w:     w,
		name:  name,
		pid:   os.Getpid(),
		level: level,
	}
}

func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// qualifyKey prefixes key with any still-open WithGroup names, matching slog's group
// convention (e.g. group "req" + key "id" -> "req.id").
func qualifyKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func formatAttr(a slog.Attr) string {
	return a.Key + "=" + a.Value.String()
}

// Handle renders the fixed "funcName -- message" line the rest of this package relies
// on, then appends any bound (WithAttrs) or call-site attrs other than FuncAttrKey as
// trailing "key=value" pairs -- so a future slog.Default().With(...)/WithGroup(...)
// call surfaces its attrs instead of silently vanishing, without changing the line
// shape for the common case (only FuncAttrKey) that the rest of this codebase uses.
func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	funcName := "?"

	parts := make([]string, 0, len(h.attrs)+r.NumAttrs())
	for _, a := range h.attrs {
		parts = append(parts, formatAttr(a))
	}

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == FuncAttrKey {
			funcName = a.Value.String()
			return true
		}
		parts = append(parts, formatAttr(slog.Attr{Key: qualifyKey(h.groupPrefix, a.Key), Value: a.Value}))
		return true
	})

	line := fmt.Sprintf("[%s,%03d] [%d] [%s] [%s]: %s -- %s",
		r.Time.Format("2006-01-02 15:04:05"),
		r.Time.Nanosecond()/1e6,
		h.pid,
		h.name,
		r.Level.String(),
		funcName,
		r.Message,
	)
	if len(parts) > 0 {
		line += " " + strings.Join(parts, " ")
	}
	line += "\n"

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := io.WriteString(h.w, line)
	return err
}

func (h *prettyHandler) WithAttrs(as []slog.Attr) slog.Handler {
	if len(as) == 0 {
		return h
	}

	qualified := make([]slog.Attr, 0, len(as))
	for _, a := range as {
		qualified = append(qualified, slog.Attr{Key: qualifyKey(h.groupPrefix, a.Key), Value: a.Value})
	}

	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), qualified...)
	return &next
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	next := *h
	next.groupPrefix = qualifyKey(h.groupPrefix, name)
	return &next
}

// ParseLogLevel maps a config string (case-insensitive) to a slog.Level.
func ParseLogLevel(s string) (slog.Level, error) {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q (want DEBUG, INFO, WARN, or ERROR)", s)
	}
}

// SetupLogging directs slog's default logger to both stdout and a size/count-bounded
// rotating file (per config.LogFilePath/LogMaxSizeMB/LogMaxBackups/LogLevel), so logs
// stay visible in the console while also persisting to disk. The returned io.Closer
// releases the log file's handle; callers that need the log file removable afterward
// (e.g. tests cleaning up a temp directory) should Close() it once done.
//
// SetupLogging replaces the process-wide slog default logger, so real production usage
// calls it exactly once (from main). A caller that needs to call it again -- e.g. a test
// reconfiguring the config between cases -- must Close() the io.Closer from the previous
// call first, or that earlier rotator's open file handle leaks.
func SetupLogging(config *Config) io.Closer {
	rotator := &lumberjack.Logger{
		Filename:   config.LogFilePath,
		MaxSize:    config.LogMaxSizeMB,
		MaxBackups: config.LogMaxBackups,
	}

	// Already validated at config-load time (validateConfig), so this can't fail here.
	level, _ := ParseLogLevel(config.LogLevel)

	out := io.MultiWriter(os.Stdout, rotator)
	slog.SetDefault(slog.New(newPrettyHandler(out, loggerName, level)))

	return rotator
}
