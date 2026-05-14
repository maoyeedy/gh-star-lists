package format_test

import (
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/format"
)

func TestSelectOutputMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		jsonFlag  bool
		tsvFlag   bool
		plainFlag bool
		want      format.OutputMode
		wantErr   bool
	}{
		{name: "defaults to human", want: format.OutputHuman},
		{name: "selects json", jsonFlag: true, want: format.OutputJSON},
		{name: "selects tsv", tsvFlag: true, want: format.OutputTSV},
		{name: "selects plain", plainFlag: true, want: format.OutputPlain},
		{name: "rejects conflict", jsonFlag: true, tsvFlag: true, wantErr: true},
		{name: "rejects plain conflict", jsonFlag: true, plainFlag: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := format.SelectOutputMode(tt.jsonFlag, tt.tsvFlag, tt.plainFlag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SelectOutputMode(%v, %v, %v) returned nil error", tt.jsonFlag, tt.tsvFlag, tt.plainFlag)
				}
				return
			}
			if err != nil {
				t.Fatalf("SelectOutputMode(%v, %v, %v) returned unexpected error: %v", tt.jsonFlag, tt.tsvFlag, tt.plainFlag, err)
			}
			if got != tt.want {
				t.Fatalf("SelectOutputMode(%v, %v, %v) = %q, want %q", tt.jsonFlag, tt.tsvFlag, tt.plainFlag, got, tt.want)
			}
		})
	}
}
