package format

import "fmt"

// OutputMode selects how command results are written to stdout.
type OutputMode string

const (
	OutputHuman    OutputMode = "human"
	OutputPlain    OutputMode = "plain"
	OutputTSV      OutputMode = "tsv"
	OutputFZF      OutputMode = "fzf"
	OutputJSON     OutputMode = "json"
	OutputTemplate OutputMode = "template"
)

// SelectOutputMode validates mutually exclusive output flags and returns the
// corresponding output mode. The zero-value false/false/false/false selection is human.
func SelectOutputMode(jsonFlag, tsvFlag, plainFlag, fzfFlag bool) (OutputMode, error) {
	selected := 0
	for _, flag := range []bool{jsonFlag, tsvFlag, plainFlag, fzfFlag} {
		if flag {
			selected++
		}
	}
	if selected > 1 {
		return "", fmt.Errorf("cannot combine --plain, --json, --tsv, and --fzf")
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
	if fzfFlag {
		return OutputFZF, nil
	}
	return OutputHuman, nil
}
