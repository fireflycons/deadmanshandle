#!/usr/bin/env bash
# Decides whether the push to master in GITHUB_SHA is released: not if the
# pull request merged as that commit has [no release] (any case) in its
# title. Without a pull request (a direct push), the commit message
# (HEAD_MESSAGE) is checked instead.
# Writes release=true|false to $GITHUB_OUTPUT when run in a workflow.
set -euo pipefail

title=$(gh api "repos/$GITHUB_REPOSITORY/commits/$GITHUB_SHA/pulls" --jq '.[0].title // ""')
if [[ -n $title ]]; then
  source="the pull request's title, \"$title\""
else
  title=${HEAD_MESSAGE:-}
  source="the commit message (no pull request found)"
fi

shopt -s nocasematch
if [[ $title == *'[no release]'* ]]; then
  release=false
  summary="No release: [no release] is in $source"
else
  release=true
  summary="Release: [no release] is not in $source"
fi

echo "$summary"
echo "$summary" >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
echo "release=$release" >> "${GITHUB_OUTPUT:-/dev/null}"
