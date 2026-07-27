#!/usr/bin/env bash
#
# Creates the Honeycomb deploy marker for the Masterclass 2 demo.
#
# The marker has to sit at exactly the timestamp the seeder used, or the
# before/after boundary you draw queries around will not match the data.
# seed-canary-regression prints the right value and the exact command to run.
#
# Usage:
#   HONEYCOMB_API_KEY=<key> ./create-deploy-marker.sh <unix-timestamp> [dataset]
#
# The key needs "Manage Markers" (a v1 configuration key) — the send-events key
# the Collector uses is a different key with different permissions and will be
# rejected here.
#
# EU customers: export HONEYCOMB_API_ENDPOINT=https://api.eu1.honeycomb.io
#
# API reference: https://docs.honeycomb.io/api/markers/

set -euo pipefail

readonly DEFAULT_DATASET="sample-service"
readonly DEFAULT_ENDPOINT="https://api.honeycomb.io"

usage() {
  sed -n '3,20p' "$0" | sed 's|^# \{0,1\}||'
  exit 64
}

main() {
  if [[ $# -lt 1 || $# -gt 2 ]]; then
    usage
  fi

  local start_time="$1"
  local dataset="${2:-$DEFAULT_DATASET}"
  local endpoint="${HONEYCOMB_API_ENDPOINT:-$DEFAULT_ENDPOINT}"

  if [[ -z "${HONEYCOMB_API_KEY:-}" ]]; then
    echo "error: HONEYCOMB_API_KEY is not set." >&2
    echo "       Needs a v1 configuration key with Manage Markers permission." >&2
    exit 1
  fi

  if ! [[ "$start_time" =~ ^[0-9]+$ ]]; then
    echo "error: '$start_time' is not a Unix timestamp in seconds." >&2
    echo "       Run seed-canary-regression; it prints the value to use." >&2
    exit 1
  fi

  # Marker types are per-dataset colour keys in Honeycomb, so reusing "deploy"
  # keeps every deploy marker the same colour on the timeline.
  local body
  body=$(cat <<JSON
{
  "start_time": ${start_time},
  "message": "deploy 1.5.0 (canary)",
  "type": "deploy",
  "url": "https://github.com/honeycombio/o11y-eng-masterclass"
}
JSON
)

  local response http_code
  response=$(curl -sS -w '\n%{http_code}' \
    -X POST "${endpoint}/1/markers/${dataset}" \
    -H "X-Honeycomb-Team: ${HONEYCOMB_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body")

  http_code=$(tail -n1 <<<"$response")
  body=$(sed '$d' <<<"$response")

  if [[ "$http_code" != "200" && "$http_code" != "201" ]]; then
    echo "error: Honeycomb returned HTTP ${http_code}" >&2
    echo "$body" >&2
    case "$http_code" in
      401) echo "hint: key rejected — is this a configuration key, not a send-events key?" >&2 ;;
      404) echo "hint: dataset '${dataset}' not found. Seed it first, and check the slug." >&2 ;;
    esac
    exit 1
  fi

  echo "Created deploy marker in '${dataset}' at $(date -r "${start_time}" 2>/dev/null || echo "${start_time}")"
  echo "$body"
}

main "$@"
