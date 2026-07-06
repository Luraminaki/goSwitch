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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

func TestRenderSubstitutesData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.html")

	content := `{{ define "hello" }}Hello, {{ .Name }}!{{ end }}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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
