package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cli/go-gh/v2/pkg/template"
)

func ansiStyle(enabled bool, code string) func(string) string {
	if !enabled {
		return nil
	}
	return func(s string) string {
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
}

func Bold(enabled bool) func(string) string      { return ansiStyle(enabled, "1") }
func Underline(enabled bool) func(string) string { return ansiStyle(enabled, "4") }
func Red(enabled bool) func(string) string       { return ansiStyle(enabled, "31") }
func Green(enabled bool) func(string) string     { return ansiStyle(enabled, "32") }
func Yellow(enabled bool) func(string) string    { return ansiStyle(enabled, "33") }
func Cyan(enabled bool) func(string) string      { return ansiStyle(enabled, "36") }
func Faint(enabled bool) func(string) string     { return ansiStyle(enabled, "2") }

// FormatNameWithOwner formats "owner/repo" with the owner and "/" in faint
// and the repo name in bold. Returns plain text when color is false.
func FormatNameWithOwner(nameWithOwner string, color bool) string {
	if !color {
		return nameWithOwner
	}
	owner, repo, ok := strings.Cut(nameWithOwner, "/")
	if !ok {
		return Bold(true)(nameWithOwner)
	}
	return Faint(true)(owner) + Faint(true)("/") + Bold(true)(repo)
}

func writeJSONSlice[T any](w io.Writer, data []T) error {
	if data == nil {
		data = make([]T, 0)
	}
	return json.NewEncoder(w).Encode(data)
}

func writeTemplate[T any](w io.Writer, options Options, data []T) error {
	if data == nil {
		data = make([]T, 0)
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("template marshal error: %w", err)
	}
	t := template.New(w, options.Width, options.Color)
	if err := t.Parse(options.Template); err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}
	if err := t.Execute(bytes.NewReader(jsonData)); err != nil {
		return fmt.Errorf("template execute error: %w", err)
	}
	return t.Flush()
}
