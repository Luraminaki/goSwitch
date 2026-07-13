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

// TestPrettyHandlerWithAttrsBindsAttrs is a regression test: WithAttrs used to be a
// silent no-op, so a future slog.Default().With(...) call would drop its attrs on the
// floor instead of surfacing them in the log line.
func TestPrettyHandlerWithAttrsBindsAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := newPrettyHandler(&buf, "testapp", slog.LevelDebug)

	bound := h.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if bound == h {
		t.Fatal("WithAttrs() with a non-empty slice should return a distinct handler")
	}

	logger := slog.New(bound)
	logger.Info("hello", FuncAttrKey, "test")

	out := buf.String()
	if !strings.Contains(out, "k=v") {
		t.Errorf("log line %q does not contain the bound attr k=v", out)
	}

	// The original handler must be untouched (WithAttrs returns a modified copy).
	buf.Reset()
	slog.New(h).Info("hello", FuncAttrKey, "test")
	if strings.Contains(buf.String(), "k=v") {
		t.Errorf("original handler should not have picked up the bound attr, got: %s", buf.String())
	}
}

// TestPrettyHandlerWithGroupQualifiesKeys is a regression test for the same silent-drop
// class of bug as TestPrettyHandlerWithAttrsBindsAttrs, but for WithGroup: attrs passed
// at the call site under an open group must come out group-qualified (group.key), not
// vanish or appear unqualified.
func TestPrettyHandlerWithGroupQualifiesKeys(t *testing.T) {
	var buf bytes.Buffer
	h := newPrettyHandler(&buf, "testapp", slog.LevelDebug)

	grouped := h.WithGroup("req")
	if grouped == h {
		t.Fatal("WithGroup() with a non-empty name should return a distinct handler")
	}

	slog.New(grouped).Info("hello", FuncAttrKey, "test", "id", "42")

	out := buf.String()
	if !strings.Contains(out, "req.id=42") {
		t.Errorf("log line %q does not contain the group-qualified attr req.id=42", out)
	}
}

func TestPrettyHandlerWithEmptyAttrsOrGroupAreNoOps(t *testing.T) {
	var buf bytes.Buffer
	h := newPrettyHandler(&buf, "testapp", slog.LevelDebug)

	if h.WithAttrs(nil) != h {
		t.Error("WithAttrs(nil) should return the same handler (nothing to bind)")
	}
	if h.WithGroup("") != h {
		t.Error(`WithGroup("") should return the same handler (no group to open)`)
	}
	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Enabled() should return true for a level at or above the handler's threshold")
	}
}

// TestSetupLoggingWritesToConfiguredFile is a direct (non-integration) test of
// SetupLogging itself: it should create/rotate LogFilePath, honor LogLevel, and return
// an io.Closer that actually releases the file handle.
func TestSetupLoggingWritesToConfiguredFile(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/test.log"

	config := &Config{
		LogFilePath:   logPath,
		LogMaxSizeMB:  5,
		LogMaxBackups: 3,
		LogLevel:      "WARN",
	}

	prev := slog.Default()
	closer := SetupLogging(config)
	defer slog.SetDefault(prev)

	slog.Info("this should be suppressed by LogLevel=WARN")
	slog.Warn("this should reach the file")

	if err := closer.Close(); err != nil {
		t.Fatalf("Close() on the returned io.Closer failed: %v", err)
	}

	data, err := os.ReadFile(logPath) //nolint:gosec // logPath is built from t.TempDir() in this test, not attacker-controlled
	if err != nil {
		t.Fatalf("SetupLogging did not create the configured log file: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "this should be suppressed") {
		t.Errorf("log file contains a DEBUG/INFO line despite LogLevel=WARN, got: %s", content)
	}
	if !strings.Contains(content, "this should reach the file") {
		t.Errorf("log file is missing the expected WARN line, got: %s", content)
	}
	if !strings.Contains(content, "["+loggerName+"]") {
		t.Errorf("log file lines should carry the app's logger name, got: %s", content)
	}
}
