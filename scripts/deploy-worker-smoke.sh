#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CRABBOX_BIN="${CRABBOX_BIN:-$ROOT/bin/crabbox}"

profile_export() {
  local name="$1" file line value
  for file in "$HOME/.profile" "$HOME/.zprofile"; do
    [[ -r "$file" ]] || continue
    line="$(grep -E "^export ${name}=" "$file" | tail -n 1 || true)"
    [[ -n "$line" ]] || continue
    value="${line#export ${name}=}"
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"
    printf '%s' "$value"
    return 0
  done
  return 1
}

if [[ -z "${CRABBOX_CLOUDFLARE_API_TOKEN:-}" ]]; then
  CRABBOX_CLOUDFLARE_API_TOKEN="$(profile_export CRABBOX_CLOUDFLARE_API_TOKEN || true)"
fi
if [[ -z "${CRABBOX_CLOUDFLARE_ACCOUNT_ID:-}" ]]; then
  CRABBOX_CLOUDFLARE_ACCOUNT_ID="$(profile_export CRABBOX_CLOUDFLARE_ACCOUNT_ID || true)"
fi
if [[ -n "${CRABBOX_CLOUDFLARE_API_TOKEN:-}" ]]; then
  export CLOUDFLARE_API_TOKEN="$CRABBOX_CLOUDFLARE_API_TOKEN"
fi
if [[ -n "${CRABBOX_CLOUDFLARE_ACCOUNT_ID:-}" ]]; then
  export CLOUDFLARE_ACCOUNT_ID="$CRABBOX_CLOUDFLARE_ACCOUNT_ID"
fi

run() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
  "$@"
}

run npm --prefix "$ROOT/worker" run format:check
run npm --prefix "$ROOT/worker" run lint
run npm --prefix "$ROOT/worker" run check
run npm --prefix "$ROOT/worker" test
run npm --prefix "$ROOT/worker" run build
run npm --prefix "$ROOT/worker" run deploy

for url in \
  "https://crabbox.openclaw.ai/v1/health" \
  "https://crabbox-coordinator.services-91b.workers.dev/v1/health"; do
  run curl -fsS "$url"
  printf '\n'
done

if [[ "${CRABBOX_DEPLOY_SMOKE_AWS:-}" != "1" ]]; then
  printf 'deploy smoke complete; set CRABBOX_DEPLOY_SMOKE_AWS=1 for an opt-in AWS lease smoke\n'
  exit 0
fi

if [[ -z "${CRABBOX_LIVE_REPO:-}" ]]; then
  printf 'CRABBOX_LIVE_REPO is required for CRABBOX_DEPLOY_SMOKE_AWS=1\n' >&2
  exit 2
fi

if [[ ! -x "$CRABBOX_BIN" ]]; then
  printf 'CRABBOX_BIN is not executable: %s\n' "$CRABBOX_BIN" >&2
  exit 2
fi

log="$(mktemp)"
lease_id=""
cleanup() {
  if [[ -n "$lease_id" ]]; then
    (cd "$CRABBOX_LIVE_REPO" && "$CRABBOX_BIN" stop "$lease_id") || true
  fi
  rm -f "$log"
}
trap cleanup EXIT

(
  cd "$CRABBOX_LIVE_REPO"
  "$CRABBOX_BIN" warmup --provider aws --ttl 20m --idle-timeout 6m --reclaim --timing-json
) 2> >(tee "$log" >&2)

lease_id="$(
  node -e 'const fs=require("fs"); for (const line of fs.readFileSync(process.argv[1],"utf8").trim().split(/\n/).reverse()) { try { const json=JSON.parse(line); if (json.leaseId) { console.log(json.leaseId); process.exit(0); } } catch {} } process.exit(1);' "$log"
)"
printf 'aws deploy smoke lease=%s\n' "$lease_id"

(
  cd "$CRABBOX_LIVE_REPO"
  "$CRABBOX_BIN" run --id "$lease_id" --no-sync --timing-json -- uname -a
)

(
  cd "$CRABBOX_LIVE_REPO"
  "$CRABBOX_BIN" stop "$lease_id"
)
lease_id=""
