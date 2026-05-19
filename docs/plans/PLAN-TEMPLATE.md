# Plan: TUI v1.X — Theme

## Goal

One paragraph describing what this version achieves.

**Prerequisite:** Shipped version this builds on (e.g. "TUI v1.3 shipped with ...").

## Scope

What's in and out of scope. Use an "In \| Out" table for balanced scope, or a
deferred-items bullet list if the boundary is mostly about pushing things out.

**In** | **Out**
---|---
Item A | Item X
Item B | Item Y

— or —

- Item A — in scope
- Item B — in scope
- Item X — deferred to v1.N+1

## Current state

Optional — describe the situation before this plan. Omit if the problem is
obvious from context or the work items are self-explanatory.

- Current limitation one
- Current limitation two

## Design

Optional — describe the desired behavior change or implementation approach when
there are multiple valid options or non-trivial tradeoffs. Omit for
straightforward work.

- Behavior change one
- Tradeoff or approach decision

## Work items

Implementation steps. Lead with rationale when needed; follow with concrete
bullets.

### W1 — Short title

- Implementation detail one
- Implementation detail two

### W2 — Short title

- ...

## Test additions

| Test | What it covers |
|------|----------------|
| Example test | What scenario it validates |

Or use a bullet list for simpler additions.

## Verification

```
make check
make ascii-check
```

Manual smoke:

```
go run . browse
# Checklist of behaviors to verify interactively.
```
