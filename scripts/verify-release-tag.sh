#!/usr/bin/env bash
set -euo pipefail

tag="${1:?usage: verify-release-tag.sh <tag> <expected-commit>}"
expected_commit="${2:?usage: verify-release-tag.sh <tag> <expected-commit>}"

local_commit="$(git rev-parse --verify "${tag}^{commit}" 2>/dev/null || true)"
if [[ -z "$local_commit" || "$local_commit" != "$expected_commit" ]]; then
  echo "local release tag $tag does not resolve to expected commit $expected_commit" >&2
  exit 1
fi

remote_refs="$(git ls-remote origin "refs/tags/$tag" "refs/tags/${tag}^{}")"
remote_direct=""
remote_peeled=""
while IFS=$'\t' read -r object ref; do
  case "$ref" in
    "refs/tags/$tag") remote_direct="$object" ;;
    "refs/tags/$tag^{}") remote_peeled="$object" ;;
  esac
done <<< "$remote_refs"
remote_commit="${remote_peeled:-$remote_direct}"
if [[ -z "$remote_commit" || "$remote_commit" != "$expected_commit" ]]; then
  echo "remote release tag $tag does not resolve to expected commit $expected_commit" >&2
  exit 1
fi
