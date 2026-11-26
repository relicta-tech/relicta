#!/usr/bin/env bash
set -euo pipefail

dist_dir="${1:-dist}"
if [[ ! -d "$dist_dir" ]]; then
  echo "snapshot directory does not exist: $dist_dir" >&2
  exit 2
fi

shopt -s nullglob
archives=("$dist_dir"/relicta_*.tar.gz "$dist_dir"/relicta_*.zip)
if [[ ${#archives[@]} -eq 0 ]]; then
  echo "no release archives found in $dist_dir" >&2
  exit 1
fi

for required in relicta_linux_amd64.tar.gz relicta_linux_arm64.tar.gz relicta_windows_amd64.zip; do
  if [[ ! -s "$dist_dir/$required" ]]; then
    echo "missing required release archive: $dist_dir/$required" >&2
    exit 1
  fi
done

temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT

for archive_path in "${archives[@]}"; do
  archive="${archive_path##*/}"
  case "$archive" in
    *_windows_*.zip)
      binary="${archive%.zip}"
      binary="${binary#relicta_}"
      binary="${binary//_/-}"
      unzip -p "$archive_path" relicta.exe > "$temp_dir/relicta-$binary.exe"
      ;;
    *.tar.gz)
      binary="${archive%.tar.gz}"
      binary="${binary#relicta_}"
      binary="${binary//_/-}"
      tar -xOf "$archive_path" relicta > "$temp_dir/relicta-$binary"
      ;;
  esac
done

python3 - "$temp_dir"/relicta-* <<'PY'
from pathlib import Path
import sys

markers = (b"frontend/index.html", b"frontend/assets/index-", b"frontend/assets/client-")
for path in sys.argv[1:]:
    binary = Path(path).read_bytes()
    missing = [marker.decode() for marker in markers if marker not in binary]
    if missing:
        raise SystemExit(f"{path}: missing embedded dashboard files: {missing}")
    print(f"{path}: embedded dashboard verified")
PY

sboms=()
for archive_path in "${archives[@]}"; do
  sboms+=("${archive_path##*/}.sbom.json")
done
for sbom in "${sboms[@]}"; do
  if [[ ! -s "$dist_dir/$sbom" ]]; then
    echo "missing release SBOM: $dist_dir/$sbom" >&2
    exit 1
  fi
done

python3 - "$dist_dir" "${sboms[@]}" <<'PY'
import json
from pathlib import Path
import sys

dist = Path(sys.argv[1])
for name in sys.argv[2:]:
    path = dist / name
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SystemExit(f"{path}: invalid SBOM JSON: {exc}")
    if not isinstance(document, dict) or not str(document.get("spdxVersion", "")).startswith("SPDX-"):
        raise SystemExit(f"{path}: expected an SPDX JSON document")
    print(f"{path}: SPDX document verified")
PY

archive_names=()
for archive_path in "${archives[@]}"; do
  archive_names+=("${archive_path##*/}")
done
python3 - "$dist_dir/checksums.txt" "$dist_dir" "${archive_names[@]}" <<'PY'
import hashlib
from pathlib import Path
import sys

manifest = Path(sys.argv[1])
root = Path(sys.argv[2]).resolve()
if not manifest.is_file() or manifest.stat().st_size == 0:
    raise SystemExit(f"missing checksum manifest: {manifest}")

entries = 0
seen = set()
required_artifacts = {
    "relicta_linux_amd64.tar.gz",
    "relicta_linux_arm64.tar.gz",
    "relicta_windows_amd64.zip",
}
for name in sys.argv[3:]:
    required_artifacts.add(name)
    required_artifacts.add(name + ".sbom.json")
verified_artifacts = set()
for line_number, line in enumerate(manifest.read_text(encoding="utf-8").splitlines(), 1):
    if not line.strip():
        continue
    parts = line.split(maxsplit=1)
    valid_digest = (
        len(parts) == 2
        and len(parts[0]) == 64
        and all(char in "0123456789abcdefABCDEF" for char in parts[0])
    )
    if not valid_digest:
        raise SystemExit(f"{manifest}:{line_number}: malformed SHA-256 entry")
    expected, name = parts
    name = name.lstrip("*")
    if name in seen:
        raise SystemExit(f"{manifest}:{line_number}: duplicate checksum entry for {name!r}")
    seen.add(name)
    path = (root / name).resolve()
    if root not in path.parents or not path.is_file():
        raise SystemExit(f"{manifest}:{line_number}: invalid or missing artifact {name!r}")
    sha256 = hashlib.sha256()
    with path.open("rb") as artifact:
        for chunk in iter(lambda: artifact.read(1024 * 1024), b""):
            sha256.update(chunk)
    digest = sha256.hexdigest()
    if digest.lower() != expected.lower():
        raise SystemExit(f"{manifest}:{line_number}: checksum mismatch for {name}")
    if name in required_artifacts:
        verified_artifacts.add(name)
    entries += 1

if entries == 0:
    raise SystemExit(f"{manifest}: no checksum entries")
missing_artifacts = sorted(required_artifacts - verified_artifacts)
if missing_artifacts:
    missing = ", ".join(missing_artifacts)
    raise SystemExit(f"{manifest}: missing checksum entries for release artifacts: {missing}")
print(f"{manifest}: {entries} checksum entries verified")
PY

published_formula="$temp_dir/relicta-published-formula.rb"
scripts/publish-homebrew-formula.sh --dry-run v0.0.0 "$dist_dir/checksums.txt" > "$published_formula"
python3 - "$published_formula" "$dist_dir/checksums.txt" <<'PY'
from pathlib import Path
import re
import sys

formula_path = Path(sys.argv[1])
formula = formula_path.read_text(encoding="utf-8")
manifest = Path(sys.argv[2])
checksums = {}
for line in manifest.read_text(encoding="utf-8").splitlines():
    parts = line.split(maxsplit=1)
    if len(parts) == 2:
        checksums[parts[1].lstrip("*")] = parts[0].lower()

if "depends_on :linux" not in formula or "relicta_darwin_" in formula:
    raise SystemExit(f"{formula_path}: expected an explicitly Linux-only formula")

for architecture in ("amd64", "arm64"):
    archive = f"relicta_linux_{architecture}.tar.gz"
    expected = checksums.get(archive)
    pattern = re.compile(
        rf'url "https://github\.com/relicta-tech/relicta/releases/download/v0\.0\.0/{re.escape(archive)}"\s+'
        rf'sha256 "([0-9a-f]{{64}})"'
    )
    match = pattern.search(formula)
    if expected is None or match is None or match.group(1) != expected:
        raise SystemExit(f"{formula_path}: URL or SHA-256 mismatch for {archive}")

print(f"{formula_path}: custom Homebrew formula matches the release checksums")
PY

echo "release snapshot validation passed: $dist_dir"
