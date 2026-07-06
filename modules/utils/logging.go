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

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	funcName := "?"
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == FuncAttrKey {
			funcName = a.Value.String()
			return false
		}
		return true
	})

	line := fmt.Sprintf("[%s,%03d] [%d] [%s] [%s]: %s -- %s\n",
		r.Time.Format("2006-01-02 15:04:05"),
		r.Time.Nanosecond()/1e6,
		h.pid,
		h.name,
		r.Level.String(),
		funcName,
		r.Message,
	)

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := io.WriteString(h.w, line)
	return err
}

func (h *prettyHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *prettyHandler) WithGroup(_ string) slog.Handler {
	return h
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
