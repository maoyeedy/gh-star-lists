package format

import (
	"fmt"
	"time"
)

func bold(enabled bool) func(string) string {
	if !enabled {
		return nil
	}
	return func(s string) string {
		return "\x1b[1m" + s + "\x1b[0m"
	}
}

func faint(enabled bool) func(string) string {
	if !enabled {
		return nil
	}
	return func(s string) string {
		return "\x1b[2m" + s + "\x1b[0m"
	}
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
