#!/usr/bin/env bash
#
# verify-deploy.sh — prove the deployed API actually works.
#
# Usage:
#   ./scripts/verify-deploy.sh https://phc-api-xxxxx-ew.a.run.app
#
# Runs the five checks that matter, in order of how much they prove.
set -uo pipefail

API_URL="${1:-}"
if [ -z "$API_URL" ]; then
  echo "Usage: $0 <API_URL>" >&2
  echo "  e.g. $0 https://phc-api-abc123-ew.a.run.app" >&2
  exit 1
fi
API_URL="${API_URL%/}"   # strip a trailing slash

PASS=0
FAIL=0

# Print at most 300 chars of a response body, so a stray HTML error page from a
# proxy does not flood the terminal.
show() { printf '   %.300s\n' "$1"; }

ok()   { echo "  ✅ $1"; PASS=$((PASS+1)); }
bad()  { echo "  ❌ $1"; FAIL=$((FAIL+1)); }
warn() { echo "  ⚠️  $1"; }

echo "═══════════════════════════════════════════════════════════"
echo "  Verifying: $API_URL"
echo "═══════════════════════════════════════════════════════════"

# ─────────────────────────────────────────────────────────────
# 1. Liveness. Proves the container started and binds PORT.
#    Does NOT touch the database.
# ─────────────────────────────────────────────────────────────
echo
echo "1) /health — container is up and serving"
H="$(curl -sS --max-time 30 "$API_URL/health" 2>&1)"
show "$H"
if echo "$H" | grep -q '"status":"ok"'; then
  ok "container alive"
else
  bad "container not responding correctly"
fi

# ─────────────────────────────────────────────────────────────
# 2. Readiness. Proves Cloud Run -> pgx -> Neon works.
#    This is the one that catches a bad DATABASE_URL.
# ─────────────────────────────────────────────────────────────
echo
echo "2) /ready — database reachable"
R="$(curl -sS --max-time 30 "$API_URL/ready" 2>&1)"
show "$R"
if echo "$R" | grep -q '"database":"ok"'; then
  ok "Neon reachable from Cloud Run"
else
  bad "cannot reach Neon — check the DATABASE_URL env var (trailing newline? pooled endpoint?)"
fi

# ─────────────────────────────────────────────────────────────
# 3. THE REAL TEST. /meta runs an actual SQL query, so a green
#    tick here proves the whole chain, not just that a process
#    is listening.
# ─────────────────────────────────────────────────────────────
echo
echo "3) /api/v1/meta — real SQL round trip (THE walking skeleton)"
M="$(curl -sS --max-time 30 "$API_URL/api/v1/meta" 2>&1)"
show "$M"
if echo "$M" | grep -q '"facilityCount":1'; then
  ok "facilityCount = 1  →  Go -> pgx -> Neon -> back WORKS"
elif echo "$M" | grep -q '"facilityCount"'; then
  warn "responded, but facilityCount is not 1. Are the seed migrations applied?"
else
  bad "no usable /meta response"
fi

# ─────────────────────────────────────────────────────────────
# 4. The Neon pooler trap. Prepared statements fail only under
#    concurrency, so a single request would never reveal it.
# ─────────────────────────────────────────────────────────────
echo
echo "4) 30 concurrent requests — the Neon pooler / prepared-statement trap"
CODES="$(for _ in $(seq 1 30); do
  curl -sS -o /dev/null -w "%{http_code}\n" --max-time 30 "$API_URL/api/v1/meta" &
done; wait)"
N200="$(echo "$CODES" | grep -c '^200$')"
N500="$(echo "$CODES" | grep -c '^500$')"
echo "   200s: $N200 / 30   500s: $N500"
if [ "$N200" -eq 30 ]; then
  ok "no prepared-statement errors under concurrency"
elif [ "$N500" -gt 0 ]; then
  bad "$N500 server errors — if logs say 'prepared statement stmtcache_... already exists',"
  echo "       QueryExecModeExec is not taking effect. Tell me and I'll look."
else
  warn "mixed results: $(echo "$CODES" | sort | uniq -c | tr '\n' ' ')"
fi

# ─────────────────────────────────────────────────────────────
# 5. CORS. Two halves, and the SECOND is the one that matters:
#    a config that merely fails to crash is not evidence it works.
# ─────────────────────────────────────────────────────────────
echo
echo "5) CORS — allowed origin, then a hostile one"
export PAGES_URL="${PAGES_URL:-}"
if [ -n "$PAGES_URL" ]; then
  A="$(curl -sS -o /dev/null -D - --max-time 20 -X OPTIONS "$API_URL/api/v1/meta" \
        -H "Origin: $PAGES_URL" -H "Access-Control-Request-Method: GET" 2>&1)"
  if echo "$A" | grep -qi "access-control-allow-origin: *$PAGES_URL"; then
    ok "allowed origin accepted: $PAGES_URL"
  else
    bad "$PAGES_URL is NOT allowed. Run the Step 8 CORS update. Got:"
    echo "$A" | grep -iE "^HTTP/|access-control" | sed 's/^/       /'
  fi
else
  warn "PAGES_URL not set — skipping the allowed-origin half"
  echo "       Re-run as: PAGES_URL=https://you.pages.dev $0 $API_URL"
fi

# A hostile origin must be REJECTED. This is the meaningful assertion.
B="$(curl -sS -o /dev/null -D - --max-time 20 -X OPTIONS "$API_URL/api/v1/meta" \
      -H "Origin: https://evil.example" -H "Access-Control-Request-Method: GET" 2>&1)"
BCODE="$(echo "$B" | grep -i '^HTTP/' | tail -1 | awk '{print $2}')"
if echo "$B" | grep -qi "access-control-allow-origin"; then
  bad "a hostile origin received an allow-origin header — CORS is too permissive"
else
  ok "hostile origin rejected (HTTP ${BCODE:-?}, no allow-origin header)"
fi

# ─────────────────────────────────────────────────────────────
echo
echo "═══════════════════════════════════════════════════════════"
echo "  $PASS passed, $FAIL failed"
echo "═══════════════════════════════════════════════════════════"

if [ "$FAIL" -eq 0 ]; then
  echo
  echo "Sprint 0 is done. Next: Cloudflare Pages, then re-run with PAGES_URL set."
  echo
  echo "Cloudflare Pages settings:"
  echo "  Build command:             pnpm build"
  echo "  Build output directory:    dist"
  echo "  Root directory:            web"
  echo "  NODE_VERSION               22          <- REQUIRED, Pages defaults to 18"
  echo "  VITE_API_URL               ${API_URL}/api/v1"
  echo "  VITE_API_ORIGIN            'self' ${API_URL}"
fi
exit "$FAIL"
