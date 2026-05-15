---
name: gh-star-lists-consolidate
description: >
  Consolidate GitHub Star Lists by merging small, overlapping, or empty lists
  into broader categories. Trigger when user says: clean up, merge, consolidate,
  reorganize, compact, reduce, or "too many" lists. Also when they mention
  reaching list limits, fragmentation, or wanting to restructure their lists.
  DESTRUCTIVE workflow — moves repos and deletes emptied lists. Always show
  plan first and ask approval before any mutations.
---

# gh-star-lists-consolidate

Consolidate GitHub Star Lists by intelligently grouping similar lists and
merging small/empty ones. Uses `gh` CLI with `user` OAuth scope.

## Workflow

```
Discover → Analyze → Show Plan → Ask Approval → Execute → Report
```

## Phase 1: Discover

```bash
gh star-lists --json
```

Also run the analysis script for structured data:

```bash
gh star-lists --json | python scripts/analyze-lists.py
```

## Phase 2: Analyze

Use your judgment to classify each list. Look at the data holistically:

### Patterns to detect

| Pattern | Description |
|---------|-------------|
| **Prefix groups** | Lists sharing a prefix (e.g. `Foo-A`, `Foo-B`, `Foo-C`) are almost certainly fragments of the same topic. |
| **Semantic overlap** | Lists with different prefixes but overlapping keywords (e.g. one named `Graphics`, another `Shader`, another `VFX`). The `word_clusters` field in the analysis output highlights these. |
| **Tiny lists** | Lists with 1-2 repos are usually over-split. They can likely fold into a broader sibling. |
| **Empty lists** | 0 repos — delete candidates. |
| **Solitary lists** | Lists that don't group with any other. Keep these as-is. |

### How to decide the target

When merging list A into list B:

- **The larger list should absorb the smaller**, not the reverse. A big list named `Render` should absorb a tiny `Render-Features`, not the other way.
- **The more generic name should absorb the more specific.** `Games` absorbs `RPG-Tools`, `Network` absorbs `WebSocket`. The narrower term is a subset of the broader.
- **When in doubt about semantics**, read the actual repo names inside tiny lists (`gh star-lists repos <list-name> --json | jq '.[].nameWithOwner'`) to understand what they contain, then decide.
- **If two lists are the same size and same level of specificity**, keep both unless they're clearly redundant.

There is no fixed merge map. Every user's list taxonomy is different. Apply common sense.

## Phase 3: Show Plan + Ask Approval

Print the full plan in a table:

```
| Source | Repos | Action | Target |
|--------|-------|--------|--------|
| Foo-A  | 2     | MERGE  | Foo    |
| Bar    | 0     | DELETE | —      |
| Baz    | 12    | KEEP   | —      |
```

Include a summary: **X lists → Y lists** (Z deletions, N repos moved).

Then **wait for the user to explicitly approve**. Do not proceed without approval.

## Phase 4: Execute

### Step A: Check auth scope

Mutations require the `user` OAuth scope:

```bash
gh auth status 2>&1
```

Look for `'user'` in the token scopes list. If missing:

```bash
gh auth refresh --scopes user
```

The user must complete browser-based OAuth before you can proceed.

### Step B: Move repos

For each source list being merged, get its repos and move them one at a time.

Resolve a repo's GraphQL node ID:

```bash
OWNER="repo-owner"
NAME="repo-name"
REPO_ID=$(gh api graphql \
  -f query="query{r:repository(owner:\"$OWNER\",name:\"$NAME\"){id}}" \
  --jq '.data.r.id')
```

Move the repo to the target list (this REPLACES all list memberships):

```bash
gh api graphql --jq '.' \
  -f query="mutation{updateUserListsForItem(input:{itemId:\"$REPO_ID\",listIds:[\"$TARGET_LIST_ID\"]}){lists{id name}}}" \
  > /dev/null
```

The bundled script handles this end-to-end for a batch:

```bash
gh star-lists repos "<source-list>" --json | bash scripts/move-repos.sh "<TARGET_LIST_ID>"
```

### Step C: Verify and delete

After all repos from a source list have been moved, confirm it's empty:

```bash
gh star-lists repos "<source-list>" --json | jq length
```

Then delete:

```bash
gh api graphql --jq '.' \
  -f query="mutation{deleteUserList(input:{listId:\"$LIST_ID\"}){clientMutationId}}" \
  > /dev/null
```

Only delete lists you verified are empty.

## Phase 5: Report

Print the final state:

```bash
gh star-lists --json | jq -r '["List","Repos"],["----","-----"],(.[] | [.name, (.repoCount|tostring)]) | @tsv'
```

Summarize:
- How many lists were deleted
- How many repos were relocated
- How many lists remain

## Guardrails

- **Plan first, execute second.** No mutation calls before user approval.
- **Verify emptiness before delete.** Always check `repoCount == 0` first.
- **Scope check before mutations.** Don't attempt `updateUserListsForItem` without `user` scope — it will fail with `INSUFFICIENT_SCOPES`.
- **`updateUserListsForItem` replaces, not appends.** Calling it with `listIds: ["target"]` removes the repo from ALL other lists. This is fine for consolidation (you're deliberately moving repos), but be aware of the semantics.
- **One repo can only be in one target.** If a merge plan puts the same repo in two target lists, the second mutation will remove it from the first.
