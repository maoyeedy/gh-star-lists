package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/cli/go-gh/v2/pkg/jq"
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

func bold(enabled bool) func(string) string  { return ansiStyle(enabled, "1") }
func faint(enabled bool) func(string) string { return ansiStyle(enabled, "2") }

func writeJSONSliceWithOptions[T any](w io.Writer, options Options, data []T) error {
	if options.JQ == "" {
		return writeJSONSlice(w, data)
	}
	if data == nil {
		data = make([]T, 0)
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return jq.Evaluate(bytes.NewReader(jsonData), w, options.JQ)
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

func shortAge(value string, now time.Time) string {
	if value == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if parsed.After(now) {
		return parsed.Format("2006-01-02")
	}
	duration := now.Sub(parsed)
	switch {
	case duration < time.Minute:
		return "now"
	case duration < time.Hour:
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	case duration < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	case duration < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(duration.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(duration.Hours()/(24*365)))
	}
}
