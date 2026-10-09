#!/usr/bin/env bash
# Checks that VERSION holds a semantic version (MAJOR.MINOR.PATCH) whose tag
# v<VERSION> is not on origin yet and is greater than every released version.
# With BASE_REF set (a pull request's target branch), it must also be greater
# than that branch's VERSION, so two pull requests cannot claim the same one.
# Writes version=<VERSION> to $GITHUB_OUTPUT when run in a workflow.
set -euo pipefail

version=$(tr -d '[:space:]' < VERSION)

if ! [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error file=VERSION::VERSION must be MAJOR.MINOR.PATCH, got '$version'"
  exit 1
fi

tags=$(git ls-remote --tags --refs origin 'v*' | sed 's#.*refs/tags/##')

if grep -qxF "v$version" <<< "$tags"; then
  echo "::error file=VERSION::v$version is already released; bump VERSION"
  exit 1
fi

latest=$(grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' <<< "$tags" | sort -V | tail -n 1 || true)

if [[ -n $latest && $(printf '%s\n' "${latest#v}" "$version" | sort -V | tail -n 1) != "$version" ]]; then
  echo "::error file=VERSION::VERSION $version is lower than the latest release $latest"
  exit 1
fi

if [[ -n ${BASE_REF:-} ]]; then
  git fetch -q --depth=1 origin "$BASE_REF"
  base_version=$(git show FETCH_HEAD:VERSION 2> /dev/null | tr -d '[:space:]' || true)
  if [[ -n $base_version && ( $base_version == "$version" || $(printf '%s
' "$base_version" "$version" | sort -V | tail -n 1) != "$version" ) ]]; then
    echo "::error file=VERSION::VERSION $version must be greater than $base_version on $BASE_REF; bump VERSION"
    exit 1
  fi
fi

echo "Next version: v$version (latest release: ${latest:-none})"
echo "version=$version" >> "${GITHUB_OUTPUT:-/dev/null}"
