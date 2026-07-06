package utils

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"DEBUG", slog.LevelDebug, false},
		{"debug", slog.LevelDebug, false},
		{"INFO", slog.LevelInfo, false},
		{"WARN", slog.LevelWarn, false},
		{"WARNING", slog.LevelWarn, false},
		{"ERROR", slog.LevelError, false},
		{"VERBOSE", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseLogLevel(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseLogLevel(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestPrettyHandlerFormat regression-checks the line shape the user asked for:
// "[%(asctime)s] [%(process)s] [%(name)s] [%(levelname)s]: %(funcName)s -- %(message)s".
//
// funcName comes from an explicit Caller() call passed as the FuncAttrKey attribute,
// not from slog.Record.PC: PC's capture depth is calibrated for the exact
// slog.Info/Warn/etc call shape, and inlining of those tiny wrapper functions has
// been observed (in this exact toolchain) to throw it off, landing on an internal
// log/slog frame instead of the real caller -- this is a regression test for that.
func TestPrettyHandlerFormat(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(newPrettyHandler(&buf, "testapp", slog.LevelDebug)))
	defer slog.SetDefault(prev)

	slog.Info("hello world", FuncAttrKey, Caller())

	out := buf.String()

	pattern := `^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}\] \[\d+\] \[testapp\] \[INFO\]: (\S+) -- hello world\n$`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(out)
	if match == nil {
		t.Fatalf("log line %q does not match expected format %q", out, pattern)
	}

	if !strings.Contains(match[1], "TestPrettyHandlerFormat") {
		t.Errorf("funcName = %q, expected it to identify the calling test function", match[1])
	}

	if !strings.Contains(out, "["+strconv.Itoa(os.Getpid())+"]") {
		t.Errorf("log line %q does not contain the process's own PID", out)
	}
}

func TestPrettyHandlerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(newPrettyHandler(&buf, "testapp", slog.LevelWarn)))
	defer slog.SetDefault(prev)

	slog.Debug("debug message")
	slog.Info("info message")
	slog.Warn("warn message")
	slog.Error("error message")

	out := buf.String()

	if strings.Contains(out, "debug message") || strings.Contains(out, "info message") {
		t.Errorf("handler with level=WARN should suppress DEBUG/INFO, got: %s", out)
	}
	if !strings.Contains(out, "warn message") || !strings.Contains(out, "error message") {
		t.Errorf("handler with level=WARN should pass through WARN/ERROR, got: %s", out)
	}
}

func TestPrettyHandlerWithAttrsAndGroupAreNoOps(t *testing.T) {
	var buf bytes.Buffer
	h := newPrettyHandler(&buf, "testapp", slog.LevelDebug)

	if h.WithAttrs([]slog.Attr{slog.String("k", "v")}) != h {
		t.Error("WithAttrs() should return the same handler (attrs aren't used)")
	}
	if h.WithGroup("g") != h {
		t.Error("WithGroup() should return the same handler (groups aren't used)")
	}
	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Enabled() should return true for a level at or above the handler's threshold")
	}
}
