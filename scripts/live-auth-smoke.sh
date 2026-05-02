#!/usr/bin/env bash
set -euo pipefail

if [[ "${CRABBOX_LIVE:-}" != "1" ]]; then
  echo "set CRABBOX_LIVE=1 to run live coordinator auth smoke tests" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cb="${CRABBOX_BIN:-$root/bin/crabbox}"
coord="${CRABBOX_AUTH_SMOKE_COORDINATOR:-${CRABBOX_COORDINATOR:-https://crabbox-access.openclaw.ai}}"
config_path="$("$cb" config path)"

need_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 2
  fi
}

need_tool curl
need_tool jq
need_tool ruby

config_value() {
  local key_path="$1"
  ruby -ryaml -e '
    value = ARGV[1].split(".").reduce(YAML.load_file(ARGV[0])) do |memo, key|
      memo.is_a?(Hash) ? memo[key] : nil
    end
    exit 3 if value.nil? || value.to_s.empty?
    print value
  ' "$config_path" "$key_path"
}

curl_quote() {
  ruby -e 'print ARGV[0].inspect' "$1"
}

write_curl_config() {
  local token="$1"
  local url="$2"
  local output="$3"
  shift 3
  : >"$output"
  chmod 0600 "$output"
  {
    printf 'url = %s\n' "$(curl_quote "$url")"
    printf 'request = "GET"\n'
    printf 'connect-timeout = "10"\n'
    printf 'max-time = "300"\n'
    printf 'silent\n'
    printf 'show-error\n'
    printf 'location\n'
    printf 'output = "-"\n'
    printf 'write-out = "\\n%%{http_code}"\n'
    printf 'header = %s\n' "$(curl_quote "Authorization: Bearer $token")"
    if [[ -n "${access_client_id:-}" && -n "${access_client_secret:-}" ]]; then
      printf 'header = %s\n' "$(curl_quote "CF-Access-Client-Id: $access_client_id")"
      printf 'header = %s\n' "$(curl_quote "CF-Access-Client-Secret: $access_client_secret")"
    fi
    for header in "$@"; do
      printf 'header = %s\n' "$(curl_quote "$header")"
    done
  } >>"$output"
}

request_json() {
  local token="$1"
  local path="$2"
  local body_file="$3"
  shift 3
  local cfg
  cfg="$(mktemp)"
  write_curl_config "$token" "${coord%/}$path" "$cfg" "$@"
  local response
  response="$(curl --config "$cfg")"
  rm -f "$cfg"
  local status="${response##*$'\n'}"
  local body="${response%$'\n'*}"
  printf '%s' "$body" >"$body_file"
  printf '%s' "$status"
}

shared_token="$(config_value broker.token)"
admin_token="$(config_value broker.adminToken)"
access_client_id="${CRABBOX_ACCESS_CLIENT_ID:-$(config_value broker.access.clientId 2>/dev/null || true)}"
access_client_secret="${CRABBOX_ACCESS_CLIENT_SECRET:-$(config_value broker.access.clientSecret 2>/dev/null || true)}"
owner="${CRABBOX_OWNER:-$(git config user.email 2>/dev/null || true)}"
owner="${owner:-crabbox-auth-smoke@example.invalid}"
org="${CRABBOX_ORG:-openclaw}"

if [[ "$coord" == *"crabbox-access.openclaw.ai"* ]]; then
  no_access_code="$(curl -sS -o /dev/null -w '%{http_code}' "${coord%/}/v1/health")"
  if [[ "$no_access_code" != "403" ]]; then
    echo "failed no-access edge check: HTTP $no_access_code" >&2
    exit 1
  fi
  echo "ok no-access edge denied http=403"
fi

whoami="$(env -u CRABBOX_COORDINATOR_TOKEN CRABBOX_COORDINATOR="$coord" "$cb" whoami --json)"
printf '%s\n' "$whoami" | jq -e '.auth == "bearer" and (.owner | length > 0) and (.org | length > 0)' >/dev/null
echo "ok shared token whoami owner=$(printf '%s\n' "$whoami" | jq -r '.owner') org=$(printf '%s\n' "$whoami" | jq -r '.org')"

body="$(mktemp)"
trap 'rm -f "$body"' EXIT

status="$(request_json "$shared_token" "/v1/whoami" "$body" \
  "X-Crabbox-Owner: $owner" \
  "X-Crabbox-Org: $org" \
  "cf-access-authenticated-user-email: spoof@example.invalid")"
if [[ "$status" != "200" ]]; then
  echo "failed raw Access spoof check: HTTP $status body=$(cat "$body")" >&2
  exit 1
fi
spoof_owner="$(jq -r '.owner' "$body")"
if [[ "$spoof_owner" == "spoof@example.invalid" ]]; then
  echo "failed raw Access spoof check: spoofed owner accepted" >&2
  exit 1
fi
echo "ok raw Access identity spoof ignored owner=$spoof_owner"

status="$(request_json "$shared_token" "/v1/admin/leases?limit=1" "$body" \
  "X-Crabbox-Owner: $owner" \
  "X-Crabbox-Org: $org")"
if [[ "$status" != "403" ]]; then
  echo "failed shared-token admin denial: HTTP $status body=$(cat "$body")" >&2
  exit 1
fi
jq -e '.message == "admin token required"' "$body" >/dev/null
echo "ok shared token denied for admin http=403"

status="$(request_json "$admin_token" "/v1/admin/leases?limit=1" "$body" \
  "X-Crabbox-Owner: $owner" \
  "X-Crabbox-Org: $org")"
if [[ "$status" != "200" ]]; then
  echo "failed admin-token admin check: HTTP $status body=$(cat "$body")" >&2
  exit 1
fi
jq -e '.leases | type == "array"' "$body" >/dev/null
echo "ok admin token accepted for admin leases=$(jq '.leases | length' "$body")"
