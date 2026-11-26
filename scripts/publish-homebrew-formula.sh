#!/usr/bin/env bash
set -euo pipefail

dry_run=false
if [[ "${1:-}" == "--dry-run" ]]; then
  dry_run=true
  shift
fi

release_tag="${1:?usage: publish-homebrew-formula.sh [--dry-run] <release-tag> [checksum-file]}"
checksum_file="${2:-dist/checksums.txt}"

if [[ ! "$release_tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "release tag must be a stable semantic version prefixed with v: $release_tag" >&2
  exit 1
fi
if [[ ! -s "$checksum_file" ]]; then
  echo "checksum manifest is missing or empty: $checksum_file" >&2
  exit 1
fi

version="${release_tag#v}"

checksum_for() {
  local asset="$1"
  local -a matches=()
  while IFS= read -r checksum; do
    matches+=("$checksum")
  done < <(awk -v asset="$asset" '$2 == asset { print $1 }' "$checksum_file")
  if [[ "${#matches[@]}" -ne 1 || ! "${matches[0]}" =~ ^[[:xdigit:]]{64}$ ]]; then
    echo "expected one SHA-256 entry for $asset in $checksum_file" >&2
    return 1
  fi
  printf '%s' "${matches[0]}"
}

amd64_sha="$(checksum_for relicta_linux_amd64.tar.gz)"
arm64_sha="$(checksum_for relicta_linux_arm64.tar.gz)"

formula_file="$(mktemp)"
trap 'rm -f "$formula_file"' EXIT
cat >"$formula_file" <<FORMULA
# typed: strict
# frozen_string_literal: true

class Relicta < Formula
  desc "Release governance for AI-native software teams"
  homepage "https://github.com/relicta-tech/relicta"
  version "$version"
  license "MIT"
  depends_on :linux

  on_linux do
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/relicta-tech/relicta/releases/download/$release_tag/relicta_linux_amd64.tar.gz"
      sha256 "$amd64_sha"
    end
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/relicta-tech/relicta/releases/download/$release_tag/relicta_linux_arm64.tar.gz"
      sha256 "$arm64_sha"
    end
  end

  def install
    bin.install "relicta"
  end

  test do
    system "#{bin}/relicta", "version"
  end
end
FORMULA

if [[ "$dry_run" == true ]]; then
  cat "$formula_file"
  exit 0
fi

repository="relicta-tech/homebrew-tap"
formula_path="Formula/relicta.rb"
existing_sha="$(gh api "repos/$repository/contents/$formula_path" --jq .sha)"
content="$(base64 <"$formula_file" | tr -d '\n')"
response="$(gh api --method PUT "repos/$repository/contents/$formula_path" \
  --field message="Update Relicta formula to $version" \
  --field content="$content" \
  --field sha="$existing_sha")"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  commit_url="$(jq -er '.commit.html_url | strings | select(length > 0)' <<<"$response")"
  {
    echo "## Homebrew formula updated"
    echo
    echo "Relicta $version Linux formula: [$commit_url]($commit_url)"
  } >>"$GITHUB_STEP_SUMMARY"
fi
