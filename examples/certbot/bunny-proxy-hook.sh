#!/usr/bin/env bash
# certbot DNS-01 hook for bunny-api-proxy.
#
# Adds the _acme-challenge TXT record through the proxy with a scoped token
# (auth) and removes exactly that record again (cleanup). The token only
# needs add_record and delete_record for TXT in the zone.
#
# Usage:
#   export BUNNY_PROXY_URL=https://proxy.example.net    # your proxy
#   export BUNNY_PROXY_TOKEN=...                         # scoped token
#   certbot certonly --manual --preferred-challenges dns \
#     --manual-auth-hook    "/path/to/bunny-proxy-hook.sh auth" \
#     --manual-cleanup-hook "/path/to/bunny-proxy-hook.sh cleanup" \
#     -d example.com -d '*.example.com'
#
# Optional:
#   BUNNY_PROXY_PROPAGATION_SECONDS  wait after adding the record (default 30)
#   BUNNY_PROXY_ZONE                 the zone (e.g. example.com) if it cannot
#                                    be found from the domain
#
# Needs bash, curl and python3 (certbot itself runs on Python).
# certbot passes the domain in CERTBOT_DOMAIN and the challenge in
# CERTBOT_VALIDATION; what the auth hook prints becomes CERTBOT_AUTH_OUTPUT
# for the cleanup hook, which is how cleanup knows which record to delete.

set -euo pipefail

fail() { echo "bunny-proxy-hook: $*" >&2; exit 1; }

[ -n "${BUNNY_PROXY_URL:-}" ] || fail "BUNNY_PROXY_URL is not set (the proxy's URL)"
[ -n "${BUNNY_PROXY_TOKEN:-}" ] || fail "BUNNY_PROXY_TOKEN is not set (a scoped token)"
[ -n "${CERTBOT_DOMAIN:-}" ] || fail "CERTBOT_DOMAIN is not set; run this from certbot"
base="${BUNNY_PROXY_URL%/}"
domain="${CERTBOT_DOMAIN#\*.}"
domain="${domain%.}"

# api METHOD PATH [BODY]: prints the response body; fails on HTTP >= 400.
api() {
  local method=$1 path=$2 body=${3:-} out status
  local args=(-sS -X "$method" -H "AccessKey: $BUNNY_PROXY_TOKEN" -w $'\n%{http_code}')
  [ -n "$body" ] && args+=(-H "Content-Type: application/json" --data-binary "$body")
  out=$(curl "${args[@]}" "$base$path") || fail "cannot reach $base"
  status=${out##*$'\n'}
  out=${out%$'\n'*}
  if [ "$status" -ge 400 ]; then
    fail "$method $path: HTTP $status: $out"
  fi
  printf '%s' "$out"
}

# json EXPR: evaluates a Python expression on the JSON from stdin (as d).
json() { python3 -c "import json,sys; d=json.load(sys.stdin); v=$1; print('' if v is None else v)"; }

# find_zone: prints "<zone id> <zone domain>" for the domain, trying the
# domain and then each parent (www.a.example.com, a.example.com, ...).
find_zone() {
  local candidate=${BUNNY_PROXY_ZONE:-$domain} id
  while [[ $candidate == *.* ]]; do
    id=$(api GET "/dnszone?search=$candidate" | ZONE="$candidate" \
      json 'next((z["Id"] for z in d.get("Items", []) if z["Domain"].lower() == __import__("os").environ["ZONE"].lower()), None)')
    if [ -n "$id" ]; then
      echo "$id $candidate"
      return
    fi
    [ -n "${BUNNY_PROXY_ZONE:-}" ] && break
    candidate=${candidate#*.}
  done
  fail "no zone for $domain that this token may use (set BUNNY_PROXY_ZONE, or give the token a permission for the zone)"
}

case "${1:-}" in
auth)
  [ -n "${CERTBOT_VALIDATION:-}" ] || fail "CERTBOT_VALIDATION is not set"
  read -r zone_id zone < <(find_zone)
  # The record name relative to the zone: _acme-challenge[.sub].
  name="_acme-challenge"
  if [ "$domain" != "$zone" ]; then
    name="_acme-challenge.${domain%."$zone"}"
  fi
  body=$(NAME="$name" VALUE="$CERTBOT_VALIDATION" python3 -c \
    'import json,os; print(json.dumps({"Type": 3, "Name": os.environ["NAME"], "Value": os.environ["VALUE"], "Ttl": 120}))')
  record_id=$(api PUT "/dnszone/$zone_id/records" "$body" | json 'd["Id"]')
  [ -n "$record_id" ] || fail "the proxy did not return a record ID"
  echo "bunny-proxy-hook: added TXT $name in $zone (record $record_id)" >&2
  sleep "${BUNNY_PROXY_PROPAGATION_SECONDS:-30}"
  # certbot gives this to the cleanup hook as CERTBOT_AUTH_OUTPUT.
  echo "$zone_id $record_id"
  ;;
cleanup)
  read -r zone_id record_id <<<"${CERTBOT_AUTH_OUTPUT:-}" || true
  [ -n "${zone_id:-}" ] && [ -n "${record_id:-}" ] ||
    fail "CERTBOT_AUTH_OUTPUT does not name the record to delete (got '${CERTBOT_AUTH_OUTPUT:-}')"
  api DELETE "/dnszone/$zone_id/records/$record_id" >/dev/null
  echo "bunny-proxy-hook: removed record $record_id" >&2
  ;;
*)
  fail "usage: $0 auth|cleanup (run from certbot's --manual-auth-hook / --manual-cleanup-hook)"
  ;;
esac
