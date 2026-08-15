#!/usr/bin/env bash
set -euo pipefail

base_url="${MOSOO_SMOKE_BASE_URL:?MOSOO_SMOKE_BASE_URL is required}"
agent_id="${MOSOO_SMOKE_AGENT_ID:?MOSOO_SMOKE_AGENT_ID is required}"
api_token="${MOSOO_SMOKE_API_TOKEN:?MOSOO_SMOKE_API_TOKEN is required}"
user_id="${MOSOO_SMOKE_USER_ID:?MOSOO_SMOKE_USER_ID is required}"
environment="${MOSOO_SMOKE_ENVIRONMENT:?MOSOO_SMOKE_ENVIRONMENT is required}"

environment_key="$(printf '%s' "$environment" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
case "$environment_key" in
	dev|dev-*|development|development-*|stage|stage-*|staging|staging-*|preview|preview-*|test|test-*|testing|testing-*|qa|qa-*|sandbox|sandbox-*|nonprod|nonprod-*|nonproduction|nonproduction-*) ;;
	*)
		echo "refusing Public Thread smoke: MOSOO_SMOKE_ENVIRONMENT must explicitly identify a non-production deployment" >&2
		exit 1
		;;
esac

base_url="${base_url%/}"
smoke_host="$(MOSOO_SMOKE_URL_VALUE="$base_url" python3 -c '
import os
from urllib.parse import urlsplit

value = urlsplit(os.environ["MOSOO_SMOKE_URL_VALUE"])
if value.scheme.lower() != "https" or not value.hostname or value.username or value.password:
    raise SystemExit("MOSOO_SMOKE_BASE_URL must be an HTTPS URL without credentials")
print(value.hostname.lower().rstrip("."))
')"
case "$smoke_host" in
	cloud.mosoo.ai|api.mosoo.ai|try.mosoo.ai|mosoo.ai)
		echo "refusing to run Public Thread smoke against a production Mosoo host" >&2
		exit 1
		;;
esac
case "$base_url" in
	https://*) ;;
	*)
		echo "MOSOO_SMOKE_BASE_URL must be an HTTPS non-production deployment" >&2
		exit 1
		;;
esac

case "$base_url" in
	*/api/v1) api_base="$base_url" ;;
	*) api_base="$base_url/api/v1" ;;
esac

tmp_dir="$(mktemp -d)"
body_file="$tmp_dir/create.json"
response_file="$tmp_dir/response.json"
thread_id=""

cleanup() {
	if [ -n "$thread_id" ]; then
		curl -sS --connect-timeout 10 --max-time 30 -X DELETE \
			-H "Authorization: Bearer $api_token" \
			"$api_base/threads/$thread_id" >/dev/null || true
	fi
	rm -rf "$tmp_dir"
}
trap cleanup EXIT

MOSOO_SMOKE_USER_ID_VALUE="$user_id" MOSOO_SMOKE_BODY_FILE="$body_file" python3 -c '
import json
import os
from pathlib import Path

Path(os.environ["MOSOO_SMOKE_BODY_FILE"]).write_text(
    json.dumps({"userId": os.environ["MOSOO_SMOKE_USER_ID_VALUE"]}, separators=(",", ":")) + "\n",
    encoding="utf-8",
)
'

status="$(curl -sS --connect-timeout 10 --max-time 30 -o "$response_file" -w '%{http_code}' \
	-X POST \
	-H "Authorization: Bearer $api_token" \
	-H "Content-Type: application/json" \
	--data-binary "@$body_file" \
	"$api_base/agents/$agent_id/threads")"
case "$status" in
	2??) ;;
	*)
		echo "Public Thread smoke create failed with HTTP $status" >&2
		cat "$response_file" >&2
		exit 1
		;;
esac

thread_id="$(python3 -c '
import json
import sys

value = json.load(open(sys.argv[1], encoding="utf-8"))
thread_id = value.get("thread", {}).get("id")
if not isinstance(thread_id, str) or not thread_id.strip():
    raise SystemExit("create response did not contain thread.id")
print(thread_id)
' "$response_file")"

echo "non-production Public Thread smoke passed for $environment ($thread_id)"
