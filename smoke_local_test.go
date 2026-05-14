package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSmokeLocalProviderConflictIsNonDestructiveByDefault(t *testing.T) {
	result := runSmokeLocalWithFakeTools(t, map[string]string{
		"SMOKE_GH_MODE": "provider-conflict",
	})

	if result.exitCode == 0 {
		t.Fatalf("smoke-local unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	assertContains(t, result.stderr, "smoke assertion failed: gh extension provider conflict: existing star-lists provider blocks local install")
	assertContains(t, result.stderr, "command: gh extension install . --force")
	assertContains(t, result.stderr, "exit code: 1")
	assertContains(t, result.stderr, "stdout:\nfake install stdout")
	assertContains(t, result.stderr, "stderr:\nalready an installed extension that provides the \"star-lists\" command")
	assertContains(t, result.stderr, "GH_STAR_LISTS_REPLACE_EXTENSION=1 bash scripts/smoke-local.sh")
	assertContains(t, result.stderr, "GSL_SMOKE_REPLACE_EXISTING=1 bash scripts/smoke-local.sh")
	assertContains(t, result.stderr, "bash scripts/smoke-local.sh --replace-existing-extension")
	assertNotContains(t, result.log, "gh extension remove star-lists")
}

func TestSmokeLocalProviderConflictIsDetectedFromStdoutToo(t *testing.T) {
	result := runSmokeLocalWithFakeTools(t, map[string]string{
		"SMOKE_GH_MODE": "provider-conflict-stdout",
	})

	if result.exitCode == 0 {
		t.Fatalf("smoke-local unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	assertContains(t, result.stderr, "gh extension provider conflict")
	assertContains(t, result.stderr, "stdout:\nalready installed: extension already provides star-lists")
	assertNotContains(t, result.log, "gh extension remove star-lists")
}

func TestSmokeLocalExplicitReplacementRemovesThenInstalls(t *testing.T) {
	result := runSmokeLocalWithFakeTools(t, map[string]string{
		"SMOKE_GH_MODE": "provider-conflict",
	}, "--replace-existing-extension")

	assertSmokeLocalReplacementSucceeded(t, result)
}

func TestSmokeLocalReplacementEnvAliasesRemoveThenInstall(t *testing.T) {
	tests := map[string]map[string]string{
		"roadmap GH alias": {
			"GH_STAR_LISTS_REPLACE_EXTENSION": "1",
		},
		"legacy GSL alias": {
			"GSL_SMOKE_REPLACE_EXISTING": "1",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			env["SMOKE_GH_MODE"] = "provider-conflict"
			result := runSmokeLocalWithFakeTools(t, env)

			assertSmokeLocalReplacementSucceeded(t, result)
		})
	}
}

func TestSmokeLocalReplacementEnvAliasesRequireExactOne(t *testing.T) {
	tests := map[string]map[string]string{
		"empty GH alias": {
			"GH_STAR_LISTS_REPLACE_EXTENSION": "",
		},
		"non-one GH alias": {
			"GH_STAR_LISTS_REPLACE_EXTENSION": "true",
		},
		"empty GSL alias": {
			"GSL_SMOKE_REPLACE_EXISTING": "",
		},
		"non-one GSL alias": {
			"GSL_SMOKE_REPLACE_EXISTING": "true",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			env["SMOKE_GH_MODE"] = "provider-conflict"
			result := runSmokeLocalWithFakeTools(t, env)

			if result.exitCode == 0 {
				t.Fatalf("smoke-local unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
			}
			assertContains(t, result.stderr, "gh extension provider conflict")
			assertNotContains(t, result.log, "gh extension remove star-lists")
		})
	}
}

func TestSmokeLocalDocumentsWSLScoopGoCandidates(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("scripts", "smoke-local.sh"))
	if err != nil {
		t.Fatalf("read smoke-local script: %v", err)
	}
	script := string(content)

	assertContains(t, script, "/mnt/c/Users/jerkl/scoop/shims/go.exe")
	assertContains(t, script, "/mnt/c/Users/jerkl/scoop/apps/go/current/bin/go.exe")
}

func TestSmokeLocalCmdBridgeQuotesDynamicValues(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("scripts", "smoke-local.sh"))
	if err != nil {
		t.Fatalf("read smoke-local script: %v", err)
	}
	script := string(content)

	assertContains(t, script, `quote_cmd_exe_arg() {`)
	assertContains(t, script, `quote_cmd_exe_set() {`)
	assertContains(t, script, `to_cmd_path() {`)
	assertContains(t, script, `printf '%s\r\n' "$(quote_cmd_exe_set GOCACHE "$(to_cmd_path "$GOCACHE")")"`)
	assertContains(t, script, `printf 'cd /d %s\r\n' "$(quote_cmd_exe_arg "$(to_cmd_path "$repo_root_windows")")"`)
	assertContains(t, script, `printf '%s' "$(quote_cmd_exe_arg "$(to_cmd_path "$GO_EXE_WINDOWS")")"`)
	assertContains(t, script, `printf ' %s' "$(quote_cmd_exe_arg "$arg")"`)
	assertNotContains(t, script, `set GOCACHE=${GOCACHE}&& cd /d ${repo_root_windows}&& ${GO_EXE_WINDOWS}`)
	assertNotContains(t, script, `set GOCACHE=$(quote_cmd_exe_arg "$GOCACHE")`)
	assertNotContains(t, script, `local cmd_line=`)
}

func TestSmokeLocalUsesOperationalCommandTimeout(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("scripts", "smoke-local.sh"))
	if err != nil {
		t.Fatalf("read smoke-local script: %v", err)
	}
	script := string(content)

	assertContains(t, script, `GSL_SMOKE_COMMAND_TIMEOUT="${GSL_SMOKE_COMMAND_TIMEOUT:-60s}"`)
	assertContains(t, script, `timeout "$GSL_SMOKE_COMMAND_TIMEOUT" "$@"`)
	assertContains(t, script, `GSL_SMOKE_COMMAND_TIMEOUT=120s bash scripts/smoke-local.sh`)
	assertNotContains(t, script, `timeout 15s "$@"`)
}

func TestSmokeLocalDetectsProviderConflictPatterns(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("scripts", "smoke-local.sh"))
	if err != nil {
		t.Fatalf("read smoke-local script: %v", err)
	}
	script := string(content)

	assertContains(t, script, `last_install_failure_is_provider_conflict() {`)
	assertContains(t, script, `last_output_contains 'already installed' && last_output_contains 'star-lists' && return 0`)
	assertContains(t, script, `last_output_contains 'extension already provides' && last_output_contains 'star-lists' && return 0`)
}

func TestSmokeLocalUnrelatedInstallFailureUsesNormalContext(t *testing.T) {
	result := runSmokeLocalWithFakeTools(t, map[string]string{
		"SMOKE_GH_MODE": "unrelated-install-failure",
	})

	if result.exitCode == 0 {
		t.Fatalf("smoke-local unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	assertContains(t, result.stderr, "smoke assertion failed: expected success")
	assertContains(t, result.stderr, "command: gh extension install . --force")
	assertContains(t, result.stderr, "exit code: 42")
	assertContains(t, result.stderr, "stdout:\nunrelated install stdout")
	assertContains(t, result.stderr, "stderr:\nunrelated install stderr")
	assertNotContains(t, result.stderr, "next steps:")
	assertNotContains(t, result.log, "gh extension remove star-lists")
}

func TestSmokeLocalSuccessPrintsManualLivePointerWithoutRunningLiveQueries(t *testing.T) {
	result := runSmokeLocalWithFakeTools(t, nil)

	if result.exitCode != 0 {
		t.Fatalf("smoke-local failed with exit %d\nstdout:\n%s\nstderr:\n%s\nlog:\n%s", result.exitCode, result.stdout, result.stderr, result.log)
	}
	assertContains(t, result.stdout, "local smoke readiness passed")
	assertContains(t, result.stdout, "Optional manual live checks: see README.md#optional-manual-live-verification")
	assertContains(t, result.stdout, "gh star-lists, gh star-lists --tsv, and gh star-lists repos <LIST_ID>")
	assertOrderedLog(t, result.log,
		"gh extension install . --force",
		"gh star-lists --help",
	)
	assertNotContains(t, result.log, "gh star-lists\n")
	assertNotContains(t, result.log, "gh star-lists --tsv")
	assertNotContains(t, result.log, "gh star-lists repos")
}

func TestSmokeLocalDoesNotPrintManualLivePointerWhenInstalledHelpFails(t *testing.T) {
	result := runSmokeLocalWithFakeTools(t, map[string]string{
		"SMOKE_GH_MODE": "installed-help-failure",
	})

	if result.exitCode == 0 {
		t.Fatalf("smoke-local unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	assertContains(t, result.stderr, "smoke assertion failed: expected exit 0")
	assertContains(t, result.stderr, "command: gh star-lists --help")
	assertNotContains(t, result.stdout, "local smoke readiness passed")
	assertNotContains(t, result.stdout, "Optional manual live checks")
	assertOrderedLog(t, result.log,
		"gh extension install . --force",
		"gh star-lists --help",
	)
	assertNotContains(t, result.log, "gh star-lists\n")
	assertNotContains(t, result.log, "gh star-lists --tsv")
	assertNotContains(t, result.log, "gh star-lists repos")
}

func assertSmokeLocalReplacementSucceeded(t *testing.T, result smokeLocalResult) {
	t.Helper()
	if result.exitCode != 0 {
		t.Fatalf("smoke-local failed with exit %d\nstdout:\n%s\nstderr:\n%s\nlog:\n%s", result.exitCode, result.stdout, result.stderr, result.log)
	}
	assertContains(t, result.stderr, "warning: existing gh star-lists extension marker blocked install; replacement was explicitly requested")
	assertContains(t, result.stderr, "initial install stdout:\nfake install stdout")
	assertContains(t, result.stderr, "initial install stderr:\nalready an installed extension that provides the \"star-lists\" command")
	assertOrderedLog(t, result.log,
		"gh extension install . --force",
		"gh extension remove star-lists",
		"gh extension install . --force",
		"gh star-lists --help",
	)
}

type smokeLocalResult struct {
	exitCode int
	stdout   string
	stderr   string
	log      string
}

func runSmokeLocalWithFakeTools(t *testing.T, env map[string]string, args ...string) smokeLocalResult {
	t.Helper()

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is required to exercise scripts/smoke-local.sh")
	}
	if runtime.GOOS == "windows" {
		if output, err := exec.Command(bashPath, "-lc", "uname -r").Output(); err == nil && strings.Contains(strings.ToLower(string(output)), "microsoft") {
			t.Skip("Windows WSL bash does not preserve the temp PATH fixture for smoke-local tests; use Git Bash or a Unix shell")
		}
	}

	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}

	workDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(workDir, "scripts"), 0o755); err != nil {
		t.Fatalf("create fake repo scripts dir: %v", err)
	}
	copyFile(t, "go.mod", filepath.Join(workDir, "go.mod"))
	copyFile(t, filepath.Join("scripts", "smoke-local.sh"), filepath.Join(workDir, "scripts", "smoke-local.sh"))

	logPath := filepath.Join(tempDir, "calls.log")
	statePath := filepath.Join(tempDir, "gh-state")
	writeExecutable(t, filepath.Join(binDir, "go"), fakeGoScript())
	writeExecutable(t, filepath.Join(binDir, "gh"), fakeGhScript())

	cmdArgs := append([]string{"scripts/smoke-local.sh"}, args...)
	cmd := exec.Command(bashPath, cmdArgs...)
	cmd.Dir = workDir
	cmd.Env = smokeLocalEnv(binDir, logPath, statePath, env)

	stdout, stderr, err := runCommandCapturingOutput(cmd)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run smoke-local: %v", err)
		}
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake command log: %v", err)
	}

	return smokeLocalResult{
		exitCode: exitCode,
		stdout:   stdout,
		stderr:   stderr,
		log:      string(logBytes),
	}
}

func smokeLocalEnv(binDir, logPath, statePath string, extra map[string]string) []string {
	env := os.Environ()
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SMOKE_LOG="+filepath.ToSlash(logPath),
		"SMOKE_GH_STATE="+filepath.ToSlash(statePath),
	)
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}

func runCommandCapturingOutput(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	if err := os.WriteFile(destination, content, 0o755); err != nil {
		t.Fatalf("write %s: %v", destination, err)
	}
}

func fakeGoScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail
echo "go $*" >> "$SMOKE_LOG"
case "$1" in
  test|vet)
    exit 0
    ;;
  build)
    cat > gh-star-lists.exe <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  "--help")
    printf 'gh star-lists v0.1\n\nUsage:\n  gh star-lists [list] [--json|--tsv]\n\nCommands:\n  repos <LIST_ID>\n'
    exit 0
    ;;
  "repos")
    printf 'missing list id for repos\n\nUsage:\n' >&2
    exit 2
    ;;
  "list --json --tsv")
    printf 'cannot combine --json and --tsv\n\nUsage:\n' >&2
    exit 2
    ;;
  "stars")
    printf 'unknown command "stars"\n\nUsage:\n' >&2
    exit 2
    ;;
  *)
    printf 'unexpected fake gh-star-lists invocation: %s\n' "$*" >&2
    exit 99
    ;;
esac
EOF
    chmod +x gh-star-lists.exe
    exit 0
    ;;
  *)
    printf 'unexpected fake go invocation: %s\n' "$*" >&2
    exit 99
    ;;
esac
`
}

func fakeGhScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail
echo "gh $*" >> "$SMOKE_LOG"
mode="${SMOKE_GH_MODE:-success}"
case "$*" in
  "extension install . --force")
    if [[ "$mode" == "provider-conflict" && ! -f "$SMOKE_GH_STATE" ]]; then
      echo 'fake install stdout'
      echo 'already an installed extension that provides the "star-lists" command' >&2
      exit 1
    fi
    if [[ "$mode" == "provider-conflict-stdout" && ! -f "$SMOKE_GH_STATE" ]]; then
      echo 'already installed: extension already provides star-lists'
      exit 1
    fi
    if [[ "$mode" == "unrelated-install-failure" ]]; then
      echo 'unrelated install stdout'
      echo 'unrelated install stderr' >&2
      exit 42
    fi
    echo installed > "$SMOKE_GH_STATE"
    exit 0
    ;;
  "extension remove star-lists")
    echo removed > "$SMOKE_GH_STATE"
    exit 0
    ;;
  "star-lists --help")
    if [[ "$mode" == "installed-help-failure" ]]; then
      echo 'installed help failed' >&2
      exit 13
    fi
    if [[ ! -x ./gh-star-lists ]]; then
      echo 'missing extensionless gh-star-lists executable' >&2
      exit 14
    fi
    printf 'Usage:\n  gh star-lists [list]\n\nCommands:\n  repos <LIST_ID>\n'
    exit 0
    ;;
  *)
    printf 'unexpected fake gh invocation: %s\n' "$*" >&2
    exit 99
    ;;
esac
`
}

func assertOrderedLog(t *testing.T, log string, entries ...string) {
	t.Helper()
	position := 0
	for _, entry := range entries {
		index := strings.Index(log[position:], entry)
		if index < 0 {
			t.Fatalf("log missing %q after byte %d\nlog:\n%s", entry, position, log)
		}
		position += index + len(entry)
	}
}
