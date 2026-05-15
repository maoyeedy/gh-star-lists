#!/bin/bash
# gh-star-lists-consolidate: Batch move repos between star lists
#
# Usage:
#   bash scripts/move-repos.sh <source_list_json> <target_list_id>
#
# Input: JSON array of {nameWithOwner: "owner/repo"} from gh star-lists repos --json
# Reads from stdin. Example:
#   gh star-lists repos "<list-name>" --json | bash scripts/move-repos.sh "<target_id>"
#
# Output: prints each move result. Exits non-zero on first failure.

set -euo pipefail

TARGET_LIST_ID="${1:?Usage: move-repos.sh <target_list_id>}"
REPOS_JSON="$(cat)"

echo "$REPOS_JSON" | jq -c '.[]' | while read -r repo; do
  NAME_WITH_OWNER=$(echo "$repo" | jq -r '.nameWithOwner')
  OWNER="${NAME_WITH_OWNER%%/*}"
  NAME="${NAME_WITH_OWNER#*/}"

  echo "  Resolving $NAME_WITH_OWNER ..."

  REPO_ID=$(gh api graphql \
    -f query="query{r:repository(owner:\"$OWNER\",name:\"$NAME\"){id}}" \
    --jq '.data.r.id')

  gh api graphql --jq '.' \
    -f query="mutation{updateUserListsForItem(input:{itemId:\"$REPO_ID\",listIds:[\"$TARGET_LIST_ID\"]}){lists{id name}}}" \
    > /dev/null

  echo "    Moved $NAME_WITH_OWNER"
done
