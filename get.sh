#!/bin/bash

set -euo pipefail

# Support CLI args or fallback to environment variables
SPRINT_NAME="${1:-${SPRINT_NAME:-}}"
PROJECT_ID="${2:-${PROJECT_ID:-}}"

# Ensure both are set
: "${SPRINT_NAME:?Must provide SPRINT_NAME as argument or environment variable}"
: "${PROJECT_ID:?Must provide PROJECT_ID as argument or environment variable}"

echo "Using sprint: $SPRINT_NAME"
echo "Using project ID: $PROJECT_ID"

# cleanup .tmp directory
if [ -d .tmp ]; then
  echo "Cleaning up old .tmp directory..."
  rm -rf .tmp
fi

mkdir -p .tmp

echo "Fetch all items (unfiltered)..."
gh project item-list "$PROJECT_ID" --owner fleetdm --limit 1000 --format json > .tmp/items.json

echo "Fetch comments for filtered issues..."
jq --arg sprint "$SPRINT_NAME" '
  .items
  | map(select(
      .content != null
      and .content.type == "Issue"
      and (
        (.status != "Done")
        or (.sprint == null)
        or (.sprint != null and .sprint.title == $sprint)
      )
    ))
    | .[].content.number
' .tmp/items.json |
while read -r issue; do
  echo "Fetching comments for issue #$issue"
  gh issue view "$issue" --repo fleetdm/fleet --json comments > ".tmp/comments-${issue}.json"
done
