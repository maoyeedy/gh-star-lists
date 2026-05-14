package format

import "fmt"

// OutputMode selects how command results are written to stdout.
type OutputMode string

const (
	OutputHuman OutputMode = "human"
	OutputPlain OutputMode = "plain"
	OutputTSV   OutputMode = "tsv"
	OutputJSON  OutputMode = "json"
)

// SelectOutputMode validates mutually exclusive output flags and returns the
// corresponding output mode. The zero-value false/false selection is human.
func SelectOutputMode(jsonFlag, tsvFlag, plainFlag bool) (OutputMode, error) {
	selected := 0
	for _, flag := range []bool{jsonFlag, tsvFlag, plainFlag} {
		if flag {
			selected++
		}
	}
	if selected > 1 {
		return "", fmt.Errorf("cannot combine --plain, --json, and --tsv")
	}
	if jsonFlag {
		return OutputJSON, nil
	}
	if tsvFlag {
		return OutputTSV, nil
	}
	if plainFlag {
		return OutputPlain, nil
	}
	return OutputHuman, nil
}
