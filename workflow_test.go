package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorkflowFilesAreValidYAML(t *testing.T) {
	for _, path := range []string{
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
	} {
		t.Run(path, func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read workflow: %v", err)
			}

			var parsed map[string]any
			if err := yaml.Unmarshal(content, &parsed); err != nil {
				t.Fatalf("workflow is malformed YAML: %v", err)
			}

			for _, key := range []string{"name", "on", "jobs"} {
				if _, ok := parsed[key]; !ok {
					t.Fatalf("workflow missing top-level %q key", key)
				}
			}
		})
	}
}

func TestCIWorkflowLaunchabilityContract(t *testing.T) {
	content := readWorkflow(t, ".github/workflows/ci.yml")

	assertContains(t, content, "push:")
	assertContains(t, content, "pull_request:")
	assertContains(t, content, "go-version-file: go.mod")
	assertContains(t, content, "go test ./...")
	assertContains(t, content, "go vet ./...")
	assertContains(t, content, "go build")
}

func TestReleaseWorkflowPrecompileContract(t *testing.T) {
	content := readWorkflow(t, ".github/workflows/release.yml")

	assertContains(t, content, "contents: write")
	assertContains(t, content, "v*")
	assertContains(t, content, "cli/gh-extension-precompile@v2")
	assertContains(t, content, "go_version_file: go.mod")
	assertNotContains(t, content, "secrets.")
}

func readWorkflow(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	return string(content)
}

func assertContains(t *testing.T, content, want string) {
	t.Helper()

	if !strings.Contains(content, want) {
		t.Fatalf("workflow missing %q", want)
	}
}

func assertNotContains(t *testing.T, content, unwanted string) {
	t.Helper()

	if strings.Contains(content, unwanted) {
		t.Fatalf("workflow unexpectedly contains %q", unwanted)
	}
}
