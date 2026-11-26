#!/usr/bin/env bash
set -euo pipefail

image="${1:?usage: scripts/smoke-container.sh IMAGE}"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/relicta-container-smoke.XXXXXX")"
container_name="relicta-smoke-${RANDOM}-${$}"
api_key="relicta-smoke-only-${RANDOM}-${$}"

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT

chmod 0777 "$work_dir"
cd "$work_dir"
git init -q
git config user.email smoke@example.invalid
git config user.name "Relicta Container Smoke"
printf 'module example.invalid/relicta-smoke\n\ngo 1.26\n' > go.mod
printf 'package main\n\nfunc main() {}\n' > main.go
git add go.mod main.go
git commit -qm 'chore: initialize smoke repository'
git -c tag.gpgsign=false tag -a -m 'v0.1.0' v0.1.0
printf '\n// smoke feature\n' >> main.go
git add main.go
git commit -qm 'feat: add smoke capability'

docker_run=(
  docker run --rm --read-only
  --tmpfs '/tmp:rw,noexec,nosuid,size=64m'
  --cap-drop=ALL
  --security-opt=no-new-privileges
  --memory=512m --memory-swap=512m --cpus=2 --pids-limit=100
  -v "$work_dir:/workspace" -w /workspace
)

echo "Checking init in restricted container"
"${docker_run[@]}" "$image" init

echo "Checking release planning"
plan_output="$("${docker_run[@]}" "$image" plan --ci --json --no-ai)"
printf '%s\n' "$plan_output" | python3 -c '
import json, sys
plan = json.load(sys.stdin)
if plan.get("next_version") != "0.2.0":
    raise SystemExit(f"expected next_version 0.2.0, got {plan.get('next_version')!r}")
if plan.get("summary", {}).get("features") != 1:
    raise SystemExit("expected exactly one planned feature")
print("planned version 0.2.0 from one feature commit")
'

echo "Checking health"
health_output="$("${docker_run[@]}" "$image" health --json)"
printf '%s\n' "$health_output" | python3 -c '
import json, sys
health = json.load(sys.stdin)
status = health.get("status")
if status != "healthy":
    raise SystemExit(f"expected healthy status, got {status!r}")
print("health status healthy")
'

echo "Checking authenticated dashboard server"
docker run -d --name "$container_name" --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --cap-drop=ALL --security-opt=no-new-privileges \
  --memory=512m --memory-swap=512m --cpus=2 --pids-limit=100 \
  -p 127.0.0.1::8080 -v "$work_dir:/workspace" -w /workspace \
  --entrypoint /app/relicta "$image" \
  server --api-key "$api_key" --address 0.0.0.0:8080 >/dev/null

host_port="$(docker port "$container_name" 8080/tcp | awk -F: 'NF {print $NF; exit}')"
if [[ -z "$host_port" ]]; then
  echo "container did not publish a host port" >&2
  docker logs "$container_name" >&2 || true
  exit 1
fi
base_url="http://127.0.0.1:$host_port"

ready=false
for _ in {1..30}; do
  if curl --fail --silent "$base_url/healthz" >/dev/null; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  echo "dashboard server did not become healthy" >&2
  docker logs "$container_name" >&2 || true
  exit 1
fi

dashboard="$(curl --fail --silent --show-error "$base_url/")"
if [[ "$dashboard" != *"<html"* ]]; then
  echo "embedded dashboard response did not contain HTML" >&2
  exit 1
fi

unauthenticated_status="$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/releases/")"
if [[ "$unauthenticated_status" != 401 ]]; then
  echo "expected unauthenticated API status 401, got $unauthenticated_status" >&2
  exit 1
fi

authenticated_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  --header "Authorization: Bearer $api_key" "$base_url/api/v1/releases/")"
if [[ "$authenticated_status" != 200 ]]; then
  echo "expected authenticated API status 200, got $authenticated_status" >&2
  exit 1
fi

health_status=""
for _ in {1..30}; do
  health_status="$(docker inspect --format '{{.State.Health.Status}}' "$container_name")"
  [[ "$health_status" == healthy ]] && break
  sleep 1
done
if [[ "$health_status" != healthy ]]; then
  echo "container healthcheck did not become healthy (status: $health_status)" >&2
  docker logs "$container_name" >&2 || true
  exit 1
fi

echo "container smoke passed: dashboard served, API auth enforced, healthcheck healthy"
