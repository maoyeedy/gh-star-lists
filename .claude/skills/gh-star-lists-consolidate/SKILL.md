---
name: gh-star-lists-consolidate
description: >
  Consolidate GitHub Star Lists by merging small, overlapping, or empty lists
  into broader categories. Trigger: clean up, merge, consolidate, reorganize,
  compact, reduce, "too many" lists, list limits, fragmentation, restructure
  requests. DESTRUCTIVE - moves repos, deletes emptied lists. Show plan first,
  ask approval before mutations.
---

# gh-star-lists-consolidate

Consolidate GitHub Star Lists by grouping similar lists, merging small/empty ones. Uses `gh` CLI with `user` OAuth scope.

## Workflow

```
Discover → Analyze → Show Plan → Ask Approval → Execute → Report
```

## Phase 1: Discover

```bash
gh star-lists --json
```

Also run analysis script for structured data:

```bash
gh star-lists --json | python scripts/analyze-lists.py
```

## Phase 2: Analyze

Judge each list holistically.

### Patterns to detect

| Pattern | Description |
|---------|-------------|
| **Prefix groups** | Lists sharing prefix (e.g. `Foo-A`, `Foo-B`, `Foo-C`) likely fragments of same topic. |
| **Semantic overlap** | Different prefixes, overlapping keywords (e.g. `Graphics`, `Shader`, `VFX`). `word_clusters` field highlights these. |
| **Tiny lists** | 1-2 repos usually over-split. Fold into broader sibling. |
| **Empty lists** | 0 repos - delete candidates. |
| **Solitary lists** | Don't group with any other. Keep as-is. |

### How to decide target

- **Larger list absorbs smaller.** `Render` absorbs `Render-Features`, not reverse.
- **Generic name absorbs specific.** `Games` absorbs `RPG-Tools`, `Network` absorbs `WebSocket`. Narrower is subset of broader.
- **When doubt semantics**, read repo names inside tiny lists (`gh star-lists repos <list-name> --json | jq '.[].nameWithOwner'`) to understand, then decide.
- **If same size and specificity**, keep both unless clearly redundant.

No fixed merge map. Each user's taxonomy different. Apply common sense.

## Phase 3: Show Plan + Ask Approval

Print full plan in table:

```
| Source | Repos | Action | Target |
|--------|-------|--------|--------|
| Foo-A  | 2     | MERGE  | Foo    |
| Bar    | 0     | DELETE | -      |
| Baz    | 12    | KEEP   | -      |
```

Summary: **X lists → Y lists** (Z deletions, N repos moved).

Wait for user explicit approval. No proceed without.

## Phase 4: Execute

### Step A: Check auth scope

Mutations require `user` OAuth scope:

```bash
gh auth status 2>&1
```

Look for `'user'` in token scopes. If missing:

```bash
gh auth refresh --scopes user
```

User must complete browser-based OAuth before proceed.

### Step B: Move repos

For each source list, get repos and move one at a time.

Resolve repo GraphQL node ID:

```bash
OWNER="repo-owner"
NAME="repo-name"
REPO_ID=$(gh api graphql \
  -f query="query{r:repository(owner:\"$OWNER\",name:\"$NAME\"){id}}" \
  --jq '.data.r.id')
```

Move repo to target list (REPLACES all list memberships):

```bash
gh api graphql --jq '.' \
  -f query="mutation{updateUserListsForItem(input:{itemId:\"$REPO_ID\",listIds:[\"$TARGET_LIST_ID\"]}){lists{id name}}}" \
  > /dev/null
```

Bundled script handles batch end-to-end:

```bash
gh star-lists repos "<source-list>" --json | bash scripts/move-repos.sh "<TARGET_LIST_ID>"
```

### Step C: Verify and delete

After all repos moved, confirm empty:

```bash
gh star-lists repos "<source-list>" --json | jq length
```

Then delete:

```bash
gh api graphql --jq '.' \
  -f query="mutation{deleteUserList(input:{listId:\"$LIST_ID\"}){clientMutationId}}" \
  > /dev/null
```

Only delete verified empty lists.

## Phase 5: Report

Print final state:

```bash
gh star-lists --json | jq -r '["List","Repos"],["----","-----"],(.[] | [.name, (.repoCount|tostring)]) | @tsv'
```

Summarize:
- Lists deleted
- Repos relocated
- Lists remain

## Guardrails

- **Plan first, execute second.** No mutations before approval.
- **Verify empty before delete.** Check `repoCount == 0` first.
- **Scope check before mutations.** `updateUserListsForItem` without `user` scope fails with `INSUFFICIENT_SCOPES`.
- **`updateUserListsForItem` replaces, not appends.** `listIds: ["target"]` removes repo from ALL other lists. Fine for consolidation (deliberately moving repos), but beware semantics.
- **One repo, one target.** If merge plan puts same repo in two targets, second mutation removes from first.
