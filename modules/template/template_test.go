package template

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestRenderEscapesHTML is a regression test: the renderer must use html/template
// (which context-escapes output) rather than text/template (which does not), since
// rendered pages echo back user-supplied error text verbatim into the page.
func TestRenderEscapesHTML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "greeting.html")

	content := `{{ define "greeting" }}<textarea>{{ .Message }}</textarea>{{ end }}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp template: %v", err)
	}

	e := echo.New()
	NewTemplateRenderer(e, filepath.Join(dir, "*.html"))

	var buf bytes.Buffer
	data := map[string]interface{}{"Message": `</textarea><script>alert(1)</script>`}
	if err := e.Renderer.Render(&buf, "greeting", data, nil); err != nil {
		t.Fatalf("Render() returned an error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "<script>") {
		t.Fatalf("Render() did not escape unsafe content, got: %s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("Render() output does not contain the expected escaped form, got: %s", out)
	}
}

// TestRealTemplatesRenderWithoutError parses and executes the actual webui/*.html
// files (not throwaway ad-hoc templates), since Go's html/template only resolves a
// {{template "x"}} reference at Execute time -- a broken reference in a real file would
// still pass ParseGlob and only surface the first time a real request renders it. The
// integration tests in main_test.go do exercise these files too, but only indirectly
// through full HTTP requests; this catches the same class of bug faster and without a
// running server.
func TestRealTemplatesRenderWithoutError(t *testing.T) {
	e := echo.New()
	NewTemplateRenderer(e, filepath.Join("..", "..", "webui", "*.html"))

	data := map[string]interface{}{
		"SessionCount": 1,
		"MaxSessions":  10,
		"Version":      "test",
		"Waiting":      false,
		"Expired":      false,
		"Win":          false,
		"Board":        [][]int{{0, 1}, {1, 0}},
		"Solution":     []int{0, 1},
		"Moves":        []int{0},
		"Config": map[string]interface{}{
			"Dim":                     2,
			"Cheat":                   true,
			"ToggleSequence":          []bool{true, false, true},
			"AvailableToggleSequence": []int{0, 4, 8},
		},
		"Response": map[string]interface{}{"Status": "SUCCESS", "Error": ""},
	}

	for _, name := range []string{"index", "game", "waiting", "status-header", "help", "configuration", "trivia", "response", "grid"} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := e.Renderer.Render(&buf, name, data, nil); err != nil {
				t.Fatalf("rendering the real %q template failed: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Fatalf("rendering the real %q template produced no output", name)
			}
		})
	}
}

func TestRenderSubstitutesData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.html")

	content := `{{ define "hello" }}Hello, {{ .Name }}!{{ end }}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp template: %v", err)
	}

	e := echo.New()
	NewTemplateRenderer(e, filepath.Join(dir, "*.html"))

	var buf bytes.Buffer
	if err := e.Renderer.Render(&buf, "hello", map[string]interface{}{"Name": "goSwitch"}, nil); err != nil {
		t.Fatalf("Render() returned an error: %v", err)
	}

	if got, want := buf.String(), "Hello, goSwitch!"; got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}
