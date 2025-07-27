#!/bin/bash

set -euo pipefail

# cleanup .tmp directory
if [ -d .tmp ]; then
  echo "Cleaning up old .tmp directory..."
  rm -rf .tmp
fi

mkdir -p .tmp

echo "Fetch all items (unfiltered)..."
gh project item-list 70 --owner fleetdm --limit 1000 --format json > .tmp/items.json

echo "Fetch comments for filtered issues..."
jq '
  .items
  | map(select(
      .content != null
      and .content.type == "Issue"
      and (
        (.status != "Done")
        or (.sprint == null)
        or (.sprint != null and .sprint.title == "Sprint 44")
      )
    ))
    | .[].content.number
' .tmp/items.json |
while read -r issue; do
  echo "Fetching comments for issue #$issue"
  gh issue view "$issue" --repo fleetdm/fleet --json comments > ".tmp/comments-${issue}.json"
done