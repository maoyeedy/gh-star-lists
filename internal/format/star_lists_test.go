package format_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteStarListsTSVUsesLegacyFieldOrder(t *testing.T) {
	t.Parallel()

	lists := []githubapi.StarList{
		{Name: "Go Tools", Description: "CLI helpers", LastAddedAt: "2024-05-01T12:00:00Z", ID: "UL_1"},
		{Name: "No Description", LastAddedAt: "2024-05-02T12:00:00Z", ID: "UL_2"},
	}

	var out bytes.Buffer
	if err := format.WriteStarLists(&out, format.OutputTSV, lists); err != nil {
		t.Fatalf("WriteStarLists returned unexpected error: %v", err)
	}

	want := "Go Tools\tCLI helpers\t2024-05-01T12:00:00Z\tUL_1\n" +
		"No Description\t\t2024-05-02T12:00:00Z\tUL_2\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteStarLists TSV output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteStarListsJSONUsesLowerCamelCaseArray(t *testing.T) {
	t.Parallel()

	lists := []githubapi.StarList{
		{Name: "Go Tools", Description: "CLI helpers", LastAddedAt: "2024-05-01T12:00:00Z", ID: "UL_1"},
	}

	var out bytes.Buffer
	if err := format.WriteStarLists(&out, format.OutputJSON, lists); err != nil {
		t.Fatalf("WriteStarLists returned unexpected error: %v", err)
	}

	want := "[{\"name\":\"Go Tools\",\"description\":\"CLI helpers\",\"lastAddedAt\":\"2024-05-01T12:00:00Z\",\"id\":\"UL_1\"}]\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteStarLists JSON output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteStarListsEmptyOutputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode format.OutputMode
		want string
	}{
		{name: "json", mode: format.OutputJSON, want: "[]\n"},
		{name: "tsv", mode: format.OutputTSV, want: ""},
		{name: "human", mode: format.OutputHuman, want: "No Star Lists found.\n"},
		{name: "plain", mode: format.OutputPlain, want: "No Star Lists found.\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			if err := format.WriteStarLists(&out, tt.mode, nil); err != nil {
				t.Fatalf("WriteStarLists returned unexpected error: %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Fatalf("WriteStarLists empty %s output = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestWriteStarListsHumanIsDeterministic(t *testing.T) {
	t.Parallel()

	lists := []githubapi.StarList{
		{Name: "Go Tools", Description: "CLI helpers", LastAddedAt: "2024-05-01T12:00:00Z", ID: "UL_1"},
		{Name: "No Optional Fields", ID: "UL_2"},
	}

	var out bytes.Buffer
	options := format.Options{Mode: format.OutputHuman, Width: 120, Now: time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)}
	if err := format.WriteStarListsWithOptions(&out, options, lists); err != nil {
		t.Fatalf("WriteStarLists returned unexpected error: %v", err)
	}

	want := "NAME                ADDED   ID\n" +
		"Go Tools            2y ago  UL_1\n" +
		"No Optional Fields  -       UL_2\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteStarLists human output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteStarListsPlainPreservesDetailedOutput(t *testing.T) {
	t.Parallel()

	lists := []githubapi.StarList{
		{Name: "Go Tools", Description: "CLI helpers", LastAddedAt: "2024-05-01T12:00:00Z", ID: "UL_1"},
		{Name: "No Optional Fields", ID: "UL_2"},
	}

	var out bytes.Buffer
	if err := format.WriteStarLists(&out, format.OutputPlain, lists); err != nil {
		t.Fatalf("WriteStarLists returned unexpected error: %v", err)
	}

	want := "Go Tools\n" +
		"  Description: CLI helpers\n" +
		"  Last added: 2024-05-01T12:00:00Z\n" +
		"  ID: UL_1\n" +
		"No Optional Fields\n" +
		"  ID: UL_2\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteStarLists human output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteStarListsHumanColorIsOptIn(t *testing.T) {
	t.Parallel()

	lists := []githubapi.StarList{{Name: "Go Tools", LastAddedAt: "2024-05-01T12:00:00Z", ID: "UL_1"}}
	options := format.Options{Mode: format.OutputHuman, Width: 120, Now: time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)}

	var plain bytes.Buffer
	if err := format.WriteStarListsWithOptions(&plain, options, lists); err != nil {
		t.Fatalf("WriteStarLists returned unexpected error: %v", err)
	}
	if strings.Contains(plain.String(), "\x1b[") {
		t.Fatalf("WriteStarLists color-disabled output contains ANSI: %q", plain.String())
	}

	options.Color = true
	var colored bytes.Buffer
	if err := format.WriteStarListsWithOptions(&colored, options, lists); err != nil {
		t.Fatalf("WriteStarLists returned unexpected error: %v", err)
	}
	got := colored.String()
	if !strings.Contains(got, "\x1b[1mNAME") || !strings.Contains(got, "\x1b[2mUL_1") {
		t.Fatalf("WriteStarLists color-enabled output missing expected ANSI styling: %q", got)
	}
}

func TestWriteStarListsHumanDateFallbacks(t *testing.T) {
	t.Parallel()

	lists := []githubapi.StarList{
		{Name: "Bad Date", LastAddedAt: "not-a-date", ID: "UL_bad"},
		{Name: "Future", LastAddedAt: "2026-06-01T00:00:00Z", ID: "UL_future"},
		{Name: "Missing", ID: "UL_missing"},
	}

	var out bytes.Buffer
	options := format.Options{Mode: format.OutputHuman, Width: 120, Now: time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)}
	if err := format.WriteStarListsWithOptions(&out, options, lists); err != nil {
		t.Fatalf("WriteStarLists returned unexpected error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"not-a-date", "2026-06-01", "-"} {
		if !strings.Contains(got, want) {
			t.Fatalf("WriteStarLists date fallback output %q missing %q", got, want)
		}
	}
}

func TestWriteStarListsReturnsWriterErrors(t *testing.T) {
	t.Parallel()

	lists := []githubapi.StarList{{Name: "Go Tools", ID: "UL_1"}}
	if err := format.WriteStarLists(errWriter{}, format.OutputHuman, lists); err == nil {
		t.Fatal("WriteStarLists human returned nil error for failing writer")
	}
	if err := format.WriteStarLists(errWriter{}, format.OutputTSV, lists); err == nil {
		t.Fatal("WriteStarLists TSV returned nil error for failing writer")
	}
	if err := format.WriteStarLists(errWriter{}, format.OutputJSON, lists); err == nil {
		t.Fatal("WriteStarLists JSON returned nil error for failing writer")
	}
	if err := format.WriteStarLists(errWriter{}, format.OutputPlain, lists); err == nil {
		t.Fatal("WriteStarLists plain returned nil error for failing writer")
	}
}
