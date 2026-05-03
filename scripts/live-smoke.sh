#!/usr/bin/env bash
set -euo pipefail

if [[ "${CRABBOX_LIVE:-}" != "1" ]]; then
  echo "set CRABBOX_LIVE=1 to run live provider smoke tests" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cb="${CRABBOX_BIN:-$root/bin/crabbox}"
repo="${CRABBOX_LIVE_REPO:-$PWD}"
providers=",${CRABBOX_LIVE_PROVIDERS-aws,hetzner},"

run_in_repo() {
  (cd "$repo" && "$@")
}

has_provider() {
  [[ "$providers" == *",$1,"* ]]
}

extract_lease() {
  rg -o 'cbx_[a-f0-9]{12}' | head -1
}

extract_slug() {
  sed -n 's/.*slug=\([^ ]*\).*/\1/p' | rg -v '^-$' | head -1
}

stop_lease() {
  local id="$1"
  local slug="${2:-}"
  if [[ -n "$slug" ]]; then
    run_in_repo "$cb" stop "$slug" || run_in_repo "$cb" stop "$id" || true
  else
    run_in_repo "$cb" stop "$id" || true
  fi
}

provider_smoke() {
  local provider="$1"
  shift
  local lease=""
  local slug=""
  cleanup() {
    if [[ -n "$lease" ]]; then
      stop_lease "$lease" "$slug"
    fi
  }
  trap cleanup RETURN

  local out
  out="$(run_in_repo "$cb" warmup --provider "$provider" "$@" 2>&1)"
  printf '%s\n' "$out"
  lease="$(printf '%s\n' "$out" | extract_lease)"
  slug="$(printf '%s\n' "$out" | extract_slug)"
  test -n "$lease"
  test -n "$slug"

  run_in_repo "$cb" status --id "$slug" --wait --wait-timeout 90s
  run_in_repo "$cb" inspect --id "$slug" --json | jq '{id,slug,provider,state,serverType,host,ready,lastTouchedAt,expiresAt}'
  run_in_repo "$cb" ssh --id "$slug"
  run_in_repo "$cb" cache stats --id "$slug" --json | jq 'if type=="array" then {items:length,kinds:[.[].kind]} else {keys:keys} end'

  local runout
  runout="$(run_in_repo "$cb" run --id "$slug" --shell -- 'test -f package.json && printf crabbox-live-ok && printf " pwd=%s\n" "$PWD"' 2>&1)"
  printf '%s\n' "$runout"
  local runid
  runid="$(printf '%s\n' "$runout" | rg -o 'run_[a-f0-9]{12}' | tail -1 || true)"
  run_in_repo "$cb" history --lease "$lease" --limit 5
  if [[ -n "$runid" ]]; then
    run_in_repo "$cb" logs "$runid" | tail -80
  fi
  stop_lease "$lease" "$slug"
  lease=""
}

blacksmith_smoke() {
  run_in_repo "$cb" list --provider blacksmith-testbox --json | jq '.[0] // empty'
  run_in_repo "$cb" run \
    --provider blacksmith-testbox \
    --blacksmith-org "${CRABBOX_BLACKSMITH_ORG:-openclaw}" \
    --blacksmith-workflow "${CRABBOX_BLACKSMITH_WORKFLOW:-.github/workflows/ci-check-testbox.yml}" \
    --blacksmith-job "${CRABBOX_BLACKSMITH_JOB:-check}" \
    --blacksmith-ref "${CRABBOX_BLACKSMITH_REF:-main}" \
    --idle-timeout "${CRABBOX_BLACKSMITH_IDLE_TIMEOUT:-10m}" \
    --shell -- 'echo blacksmith-crabbox-ok && pwd'
}

run_in_repo "$cb" whoami --json
run_in_repo "$cb" doctor
run_in_repo "$cb" sync-plan | sed -n '1,80p'

if has_provider aws; then
  provider_smoke aws --type "${CRABBOX_LIVE_AWS_TYPE:-t3.small}" --ttl 15m --idle-timeout 5m
fi

if has_provider hetzner; then
  provider_smoke hetzner --class "${CRABBOX_LIVE_HETZNER_CLASS:-standard}" --ttl 15m --idle-timeout 2m
fi

if has_provider blacksmith-testbox; then
  blacksmith_smoke
fi

run_in_repo "$cb" admin leases --state active --json | jq 'length'
