package format

import "fmt"

// OutputMode selects how command results are written to stdout.
type OutputMode string

const (
	OutputHuman OutputMode = "human"
	OutputTSV   OutputMode = "tsv"
	OutputJSON  OutputMode = "json"
)

// SelectOutputMode validates mutually exclusive output flags and returns the
// corresponding output mode. The zero-value false/false selection is human.
func SelectOutputMode(jsonFlag, tsvFlag bool) (OutputMode, error) {
	if jsonFlag && tsvFlag {
		return "", fmt.Errorf("cannot combine --json and --tsv")
	}
	if jsonFlag {
		return OutputJSON, nil
	}
	if tsvFlag {
		return OutputTSV, nil
	}
	return OutputHuman, nil
}
