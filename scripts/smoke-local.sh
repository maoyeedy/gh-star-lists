#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f go.mod ]]; then
  echo "error: scripts/smoke-local.sh must be run from the repository root" >&2
  exit 1
fi

GSL_SMOKE_REPLACE_EXISTING="${GSL_SMOKE_REPLACE_EXISTING:-0}"
GH_STAR_LISTS_REPLACE_EXTENSION="${GH_STAR_LISTS_REPLACE_EXTENSION:-0}"
GSL_SMOKE_COMMAND_TIMEOUT="${GSL_SMOKE_COMMAND_TIMEOUT:-60s}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --replace-existing-extension)
      GSL_SMOKE_REPLACE_EXISTING=1
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage: bash scripts/smoke-local.sh [--replace-existing-extension]

Runs local test/vet/build checks, verifies auth-free help/usage paths, installs
this repository as a gh extension, and checks gh star-lists --help.

By default, an existing extension provider conflict is non-destructive: the
script prints the original gh install stdout/stderr and exits with remediation
steps. To opt into replacing the local star-lists extension marker, use either:

  GH_STAR_LISTS_REPLACE_EXTENSION=1 bash scripts/smoke-local.sh
  GSL_SMOKE_REPLACE_EXISTING=1 bash scripts/smoke-local.sh
  bash scripts/smoke-local.sh --replace-existing-extension

Long-running commands are capped by GSL_SMOKE_COMMAND_TIMEOUT, defaulting to 60s.
Override it only for slow local environments, for example:

  GSL_SMOKE_COMMAND_TIMEOUT=120s bash scripts/smoke-local.sh
EOF
      exit 0
      ;;
    *)
      echo "error: unknown smoke-local option: $1" >&2
      echo "usage: bash scripts/smoke-local.sh [--replace-existing-extension]" >&2
      exit 2
      ;;
  esac
done

REPLACE_EXISTING=0
if [[ "$GSL_SMOKE_REPLACE_EXISTING" == "1" || "$GH_STAR_LISTS_REPLACE_EXTENSION" == "1" ]]; then
  REPLACE_EXISTING=1
fi

require_tool() {
  local tool="$1"
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "error: required tool '${tool}' is not available in PATH" >&2
    exit 1
  fi
}

GO_SEARCH_LOCATIONS=()
GO_EXE=""

discover_go() {
  GO_SEARCH_LOCATIONS=()

  if command -v go >/dev/null 2>&1; then
    GO_EXE="$(command -v go)"
    GO_SEARCH_LOCATIONS+=("${GO_EXE}")
    return 0
  fi

  local windows_home=""
  if [[ -n "${USERPROFILE:-}" ]]; then
    windows_home="${USERPROFILE//\\//}"
    if [[ "$windows_home" =~ ^([A-Za-z]):/(.*)$ ]]; then
      windows_home="/${BASH_REMATCH[1],}/${BASH_REMATCH[2]}"
    fi
  fi

  local candidates=(
    "${HOME:-}/scoop/shims/go"
    "${HOME:-}/scoop/shims/go.exe"
    "${windows_home:+${windows_home}/scoop/shims/go}"
    "${windows_home:+${windows_home}/scoop/shims/go.exe}"
    "/c/Users/jerkl/scoop/shims/go"
    "/c/Users/jerkl/scoop/shims/go.exe"
    "/mnt/c/Users/jerkl/scoop/shims/go.exe"
    "C:/Users/jerkl/scoop/shims/go"
    "C:/Users/jerkl/scoop/shims/go.exe"
    "${HOME:-}/scoop/apps/go/current/bin/go.exe"
    "${windows_home:+${windows_home}/scoop/apps/go/current/bin/go.exe}"
    "/c/Users/jerkl/scoop/apps/go/current/bin/go.exe"
    "/mnt/c/Users/jerkl/scoop/apps/go/current/bin/go.exe"
    "C:/Users/jerkl/scoop/apps/go/current/bin/go.exe"
  )
  local candidate
  for candidate in "${candidates[@]}"; do
    [[ -n "$candidate" ]] || continue
    GO_SEARCH_LOCATIONS+=("$candidate")
    if [[ -x "$candidate" ]]; then
      GO_EXE="$candidate"
      return 0
    fi
  done

  {
    echo "error: required Go executable was not found"
    echo "searched locations:"
    for candidate in "${GO_SEARCH_LOCATIONS[@]}"; do
      echo "  - ${candidate}"
    done
  } >&2
  return 1
}

step() {
  echo "==> $*"
}

step "Checking required tools"
discover_go || exit 1
step "Using Go executable: ${GO_EXE}"
require_tool gh

repo_root="$(pwd)"
if command -v pwd >/dev/null 2>&1 && pwd -W >/dev/null 2>&1; then
  repo_root_windows="$(pwd -W)"
  export GOCACHE="${repo_root_windows}\\.gsd\\go-cache"
elif [[ "$GO_EXE" == *.exe && "$repo_root" =~ ^/mnt/([A-Za-z])/(.*)$ ]]; then
  repo_root_windows="${BASH_REMATCH[1]^^}:/${BASH_REMATCH[2]}"
  export GOCACHE="${repo_root_windows}/.gsd/go-cache"
elif [[ "$GO_EXE" == *.exe && "$repo_root" =~ ^/([A-Za-z])/(.*)$ ]]; then
  repo_root_windows="${BASH_REMATCH[1]^^}:/${BASH_REMATCH[2]}"
  export GOCACHE="${repo_root_windows}/.gsd/go-cache"
else
  export GOCACHE="${repo_root}/.gsd/go-cache"
fi

GO_VIA_CMD=0
GO_EXE_WINDOWS="$GO_EXE"
if [[ "$GO_EXE" == *.exe && -n "${repo_root_windows:-}" && -x /mnt/c/Windows/System32/cmd.exe && ( "$repo_root" =~ ^/mnt/[A-Za-z]/ || "$repo_root" =~ ^/[A-Za-z]/ ) ]]; then
  GO_VIA_CMD=1
  if [[ "$GO_EXE" =~ ^/mnt/([A-Za-z])/(.*)$ ]]; then
    GO_EXE_WINDOWS="${BASH_REMATCH[1]^^}:/${BASH_REMATCH[2]}"
  elif [[ "$GO_EXE" =~ ^/([A-Za-z])/(.*)$ ]]; then
    GO_EXE_WINDOWS="${BASH_REMATCH[1]^^}:/${BASH_REMATCH[2]}"
  fi
fi

LAST_CMD=""
LAST_EXIT=0
LAST_STDOUT=""
LAST_STDERR=""

quote_cmd() {
  local rendered=""
  local arg
  for arg in "$@"; do
    printf -v rendered '%s%q ' "$rendered" "$arg"
  done
  printf '%s' "${rendered% }"
}

quote_cmd_exe_arg() {
  local arg="${1//\"/\"\"}"
  printf '"%s"' "$arg"
}

quote_cmd_exe_set() {
  local name="$1"
  local value="${2//\"/\"\"}"
  printf 'set "%s=%s"' "$name" "$value"
}

to_cmd_path() {
  local path="$1"
  printf '%s' "${path//\//\\}"
}

run_capture() {
  local stdout_file stderr_file
  stdout_file="$(mktemp)"
  stderr_file="$(mktemp)"
  LAST_CMD="$(quote_cmd "$@")"

  set +e
  if command -v timeout >/dev/null 2>&1; then
    timeout "$GSL_SMOKE_COMMAND_TIMEOUT" "$@" >"$stdout_file" 2>"$stderr_file"
  else
    "$@" >"$stdout_file" 2>"$stderr_file"
  fi
  LAST_EXIT=$?
  set -e

  LAST_STDOUT="$(<"$stdout_file")"
  LAST_STDERR="$(<"$stderr_file")"
  rm -f "$stdout_file" "$stderr_file"
}

print_failure_context() {
  local reason="$1"
  {
    echo "smoke assertion failed: ${reason}"
    echo "command: ${LAST_CMD}"
    echo "exit code: ${LAST_EXIT}"
    echo "stdout:"
    if [[ -n "$LAST_STDOUT" ]]; then
      printf '%s\n' "$LAST_STDOUT"
    else
      echo "<empty>"
    fi
    echo "stderr:"
    if [[ -n "$LAST_STDERR" ]]; then
      printf '%s\n' "$LAST_STDERR"
    else
      echo "<empty>"
    fi
  } >&2
}

assert_exit() {
  local expected="$1"
  if [[ "$LAST_EXIT" -ne "$expected" ]]; then
    print_failure_context "expected exit ${expected}"
    exit 1
  fi
}

assert_stdout_contains() {
  local expected="$1"
  if [[ "$LAST_STDOUT" != *"$expected"* ]]; then
    print_failure_context "stdout missing snippet: ${expected}"
    exit 1
  fi
}

assert_stderr_contains() {
  local expected="$1"
  if [[ "$LAST_STDERR" != *"$expected"* ]]; then
    print_failure_context "stderr missing snippet: ${expected}"
    exit 1
  fi
}

assert_stdout_empty() {
  if [[ -n "$LAST_STDOUT" ]]; then
    print_failure_context "expected empty stdout"
    exit 1
  fi
}

assert_stderr_empty() {
  if [[ -n "$LAST_STDERR" ]]; then
    print_failure_context "expected empty stderr"
    exit 1
  fi
}

assert_success() {
  step "Running $(quote_cmd "$@")"
  run_capture "$@"
  if [[ "$LAST_EXIT" -ne 0 ]]; then
    print_failure_context "expected success"
    exit 1
  fi
  echo "ok: ${LAST_CMD}"
}

last_output_contains() {
  local expected="$1"
  [[ "$LAST_STDOUT" == *"$expected"* || "$LAST_STDERR" == *"$expected"* ]]
}

last_install_failure_is_provider_conflict() {
  [[ "$LAST_EXIT" -ne 0 ]] || return 1

  last_output_contains 'already an installed extension that provides the "star-lists" command' && return 0
  last_output_contains 'already installed' && last_output_contains 'star-lists' && return 0
  last_output_contains 'extension already provides' && last_output_contains 'star-lists' && return 0

  return 1
}

run_go_capture() {
  if [[ "$GO_VIA_CMD" == "1" ]]; then
    local cmd_file=".gsd/smoke-go-${BASHPID:-$$}.cmd"
    local cmd_file_windows
    cmd_file_windows="$(to_cmd_path "${repo_root_windows}/${cmd_file}")"
    {
      printf '@echo off\r\n'
      printf '%s\r\n' "$(quote_cmd_exe_set GOCACHE "$(to_cmd_path "$GOCACHE")")"
      printf 'cd /d %s\r\n' "$(quote_cmd_exe_arg "$(to_cmd_path "$repo_root_windows")")"
      printf '%s' "$(quote_cmd_exe_arg "$(to_cmd_path "$GO_EXE_WINDOWS")")"
      local arg
      for arg in "$@"; do
        printf ' %s' "$(quote_cmd_exe_arg "$arg")"
      done
      printf '\r\n'
    } >"$cmd_file"
    run_capture /mnt/c/Windows/System32/cmd.exe /C "$cmd_file_windows"
    rm -f "$cmd_file"
    LAST_CMD="$(quote_cmd "$GO_EXE" "$@")"
  else
    run_capture "$GO_EXE" "$@"
  fi
}

assert_go_success() {
  step "Running $(quote_cmd "$GO_EXE" "$@")"
  run_go_capture "$@"
  if [[ "$LAST_EXIT" -ne 0 ]]; then
    print_failure_context "expected success"
    exit 1
  fi
  echo "ok: ${LAST_CMD}"
}

assert_usage_error() {
  local expected_message="$1"
  shift
  step "Checking usage error for $(quote_cmd "$@")"
  run_capture "$@"
  assert_exit 2
  assert_stdout_empty
  assert_stderr_contains "$expected_message"
  assert_stderr_contains "Usage:"
  echo "ok: ${LAST_CMD}"
}

assert_go_success test ./...
assert_go_success vet ./...
assert_go_success build

if [[ -x ./gh-star-lists.exe ]]; then
  binary="./gh-star-lists.exe"
  if [[ ! -x ./gh-star-lists ]]; then
    cat > ./gh-star-lists <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "${script_dir}/gh-star-lists.exe" "$@"
EOF
    chmod +x ./gh-star-lists
  fi
elif [[ -x ./gh-star-lists ]]; then
  binary="./gh-star-lists"
else
  echo "error: built binary not found at ./gh-star-lists or ./gh-star-lists.exe" >&2
  exit 1
fi

step "Checking local binary help output"
run_capture "$binary" --help
assert_exit 0
assert_stdout_contains "gh star-lists v0.1"
assert_stdout_contains "Usage:"
assert_stdout_contains "Commands:"
assert_stdout_contains "repos <LIST_ID>"
assert_stderr_empty
echo "ok: ${LAST_CMD}"

assert_usage_error "missing list id for repos" "$binary" repos
assert_usage_error "cannot combine --json and --tsv" "$binary" list --json --tsv
assert_usage_error "unknown command \"stars\"" "$binary" stars

# Roadmap final-assembly proof: gh extension install . --force && gh star-lists --help
step "Installing local gh extension"
run_capture gh extension install . --force
if last_install_failure_is_provider_conflict; then
  if [[ "$REPLACE_EXISTING" == "1" ]]; then
    {
      echo "warning: existing gh star-lists extension marker blocked install; replacement was explicitly requested"
      echo "initial install stdout:"
      if [[ -n "$LAST_STDOUT" ]]; then
        printf '%s\n' "$LAST_STDOUT"
      else
        echo "<empty>"
      fi
      echo "initial install stderr:"
      if [[ -n "$LAST_STDERR" ]]; then
        printf '%s\n' "$LAST_STDERR"
      else
        echo "<empty>"
      fi
    } >&2
    assert_success gh extension remove star-lists
    assert_success gh extension install . --force
  else
    print_failure_context "gh extension provider conflict: existing star-lists provider blocks local install"
    {
      echo "next steps:"
      echo "  - Keep the existing extension: do nothing; local smoke stops before changing it."
      echo "  - Replace it for this local smoke run only if that is intended:"
      echo "      GH_STAR_LISTS_REPLACE_EXTENSION=1 bash scripts/smoke-local.sh"
      echo "      GSL_SMOKE_REPLACE_EXISTING=1 bash scripts/smoke-local.sh"
      echo "      bash scripts/smoke-local.sh --replace-existing-extension"
      echo "  - The replacement path runs: gh extension remove star-lists"
    } >&2
    exit 1
  fi
elif [[ "$LAST_EXIT" -ne 0 ]]; then
  print_failure_context "expected success"
  exit 1
else
  echo "ok: ${LAST_CMD}"
fi

step "Checking installed gh extension help output"
run_capture gh star-lists --help
assert_exit 0
assert_stdout_contains "Usage:"
assert_stdout_contains "repos <LIST_ID>"
assert_stderr_empty
echo "ok: ${LAST_CMD}"

echo "local smoke readiness passed"
echo "Optional manual live checks: see README.md#optional-manual-live-verification"
echo "Run only if your authenticated account has Star Lists: gh star-lists, gh star-lists --tsv, and gh star-lists repos <LIST_ID>"
