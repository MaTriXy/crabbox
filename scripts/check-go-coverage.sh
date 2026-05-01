#!/usr/bin/env bash
set -euo pipefail

threshold="${1:-85.0}"
profile="${TMPDIR:-/tmp}/crabbox-go-coverage.out"
core_profile="${TMPDIR:-/tmp}/crabbox-go-core-coverage.out"

go test ./... -covermode=atomic -coverprofile="$profile"

awk '
  NR == 1 {
    print
    next
  }
  $1 ~ /^github.com\/openclaw\/crabbox\/internal\/cli\/(bootstrap|claim|config|errors|flags|fmt|init|provider_labels|runlog|slug)\.go:/ {
    print
  }
' "$profile" >"$core_profile"

coverage="$(go tool cover -func="$core_profile" | awk '/^total:/ { sub(/%/, "", $3); print $3 }')"
awk -v coverage="$coverage" -v threshold="$threshold" 'BEGIN {
  if (coverage + 0 < threshold + 0) {
    printf "Go core coverage %.1f%% is below %.1f%%\n", coverage, threshold
    exit 1
  }
  printf "Go core coverage %.1f%% >= %.1f%%\n", coverage, threshold
}'
