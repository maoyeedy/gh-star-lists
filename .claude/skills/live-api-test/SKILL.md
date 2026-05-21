---
name: live-api-test
description: ""
---

# Live API Test

Manually invoked only. Validates every user-facing command against the real GitHub API, fixes failures, and restores state. The full acceptance criteria live in `GOAL.md` at the repo root — read it before starting.

## Step 1: Discover commands

Run `go run . --help` and `go run . <command> --help` for every subcommand to build your own complete list of commands and flags to exercise. Do not rely on a hardcoded list. Include aliases where they differ meaningfully.

## Step 2: Determine test order

Plan the sequence yourself based on what you discover. The only ordering rule: **read-only commands before write commands, write commands before destructive ones.** Within each tier, order however makes sense for state flow (e.g. `create` before `update` before `delete`).

## Step 3: Test rules

**Fixtures** (create if missing):
- Repo: `HyDE-Project/HyDE`
- Persistent list: `Theme`
- Ephemeral lists: prefix with `__test`, e.g. `__test__`; always deleted before you finish.

**Token scope** — check with `gh auth status` before the first mutation. All write commands (`add`, `remove`, `move`, `copy`, `merge`, `unstar`, `create`, `edit`, `delete`) require the `user` scope. Add it with `gh auth refresh -s user` if missing.

**Cleanup after each write command** — restore state before moving to the next command. Specific cases:
- After `remove`: re-add the repo to restore the list.
- After `move`: move it back.
- After `unstar`: re-add to Theme to restore list membership (`unstar` removes the star; `add` restores list membership but does not re-star — note this gap).
- After `merge`: the source list is deleted by design; clean up any repos unexpectedly added to the target.
- Delete all `__test*` lists before declaring done.

**`browse`** is TTY-only. Verify it exits with a clear error (not a panic) when stdout is not a TTY: `go run . browse 2>&1 | head -5`. A non-zero exit with a readable message counts as passing.

**Dry-run flag** — where a command supports `--dry-run`, run with it first to verify the plan output before running for real.

## Step 4: Fix failures

If a command fails, diagnose before patching. For any GitHub GraphQL API question, use Context7:
- Library ID: `/cli/go-gh` — covers GraphQL executor, field availability, pagination, auth.

After any code fix, run `make lint && make check` before continuing tests.

## Step 5: Done criteria

- Every discovered command exited 0 (or exited non-zero with a clear, expected message for TTY-only commands).
- `Theme` list contains the same repos it had before the run started.
- No `__test*` lists remain.
- `make lint && make check` passes.
