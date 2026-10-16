#!/usr/bin/env bash
#
# deploy.sh — deploy the PHC Records API to Cloud Run.
#
# Uses PLAIN ENVIRONMENT VARIABLES via --env-vars-file. No Secret Manager.
#
# Usage:
#   PROJECT_ID=my-project ./deploy.sh
# or hard-code PROJECT_ID below.
#
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────
#  CONFIGURATION
#
#  A script runs in a FRESH shell: variables you exported in your terminal
#  are NOT visible here. So either hard-code the project, or pass it in:
#      PROJECT_ID=my-project ./deploy.sh
# ─────────────────────────────────────────────────────────────────────
PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-europe-west1}"
SERVICE="${SERVICE:-phc-api}"

# ─────────────────────────────────────────────────────────────────────
#  GUARD: refuse to run with an empty project.
#
#  This is the exact failure that produced
#      "The project property is set to the empty string, which is invalid."
#  gcloud accepts --project "" without complaining until it is too late, so
#  we check it ourselves and fail with an actionable message.
# ─────────────────────────────────────────────────────────────────────
if [ -z "$PROJECT_ID" ]; then
  echo "ERROR: PROJECT_ID is not set." >&2
  echo "" >&2
  echo "  A script runs in a fresh shell, so exporting it in your terminal" >&2
  echo "  does not reach this script. Run it as:" >&2
  echo "" >&2
  echo "      PROJECT_ID=your-project-id ./deploy.sh" >&2
  echo "" >&2
  echo "  Your available projects:" >&2
  gcloud projects list --format="table(projectId,name)" >&2 2>/dev/null || true
  exit 1
fi

# The active account matters just as much as the project when you have several.
# Checked AFTER the project-number guard below, because a numeric PROJECT_ID is
# the more specific and more actionable failure.
ACTIVE_ACCOUNT="$(gcloud config get-value account 2>/dev/null || true)"

# ─────────────────────────────────────────────────────────────────────
#  GUARD: a project NUMBER is not a project ID.
#
#  `gcloud run deploy --project` requires the string ID. A project number
#  (an all-digit value) fails with:
#      "The value of --project flag was set to Project number.
#       To use this command, set it to PROJECT ID instead."
#
#  TWO THINGS THAT LOOK LIKE THEY SHOULD RESOLVE IT, BUT DO NOT:
#
#   1. `gcloud config set project 843853811185` — succeeds, but stores the
#      NUMBER verbatim. It does not convert it. Verified empirically.
#   2. `gcloud projects describe <number>` — needs an active account AND
#      resourcemanager.projects.get on that project, so it fails for many
#      accounts with a permission error rather than returning the ID.
#
#  `gcloud projects list --filter` only needs resourcemanager.projects.list
#  on projects you already have access to, so it is the reliable lookup.
#  We do not call it automatically though: a silent fallback that returns
#  nothing would leave you with an empty project again. Fail loudly instead.
# ─────────────────────────────────────────────────────────────────────
if [[ "$PROJECT_ID" =~ ^[0-9]+$ ]]; then
  echo "ERROR: PROJECT_ID looks like a project NUMBER, not a project ID." >&2
  echo "" >&2
  echo "  Cloud Run's --project flag requires the string ID. A number cannot" >&2
  echo "  be substituted, and 'gcloud config set project <number>' does NOT" >&2
  echo "  convert it — it stores the number as-is." >&2
  echo "" >&2
  echo "  Find the ID with:" >&2
  echo "" >&2
  echo "      gcloud projects list --filter=\"projectNumber=${PROJECT_ID}\" \\" >&2
  echo "        --format=\"value(projectId)\"" >&2
  echo "" >&2
  echo "  Or see every project side by side:" >&2
  echo "" >&2
  echo "      gcloud projects list --format=\"table(projectNumber,projectId,name)\"" >&2
  echo "" >&2
  echo "  Then re-run:  PROJECT_ID=<the-string-id> ./deploy.sh" >&2
  echo "" >&2
  echo "  Attempting the lookup for you now…" >&2
  echo "" >&2

  RESOLVED="$(gcloud projects list \
    --filter="projectNumber=${PROJECT_ID}" \
    --format="value(projectId)" 2>/dev/null | head -1 || true)"

  if [ -n "$RESOLVED" ]; then
    echo "  Found it: ${RESOLVED}" >&2
    echo "" >&2
    echo "  Re-run with:" >&2
    echo "      PROJECT_ID=${RESOLVED} ./deploy.sh" >&2
  else
    echo "  Could not look it up (no active account, or the project belongs" >&2
    echo "  to a different account). Run 'gcloud auth list' to check, then use" >&2
    echo "  the commands above." >&2
  fi
  exit 1
fi

# ─────────────────────────────────────────────────────────────────────
#  GUARD: an active account must exist.
#
#  With several accounts authenticated, the ACTIVE one may not be the one
#  that can see this project. Checked here (after the number guard) so the
#  more specific error wins.
# ─────────────────────────────────────────────────────────────────────
if [ -z "$ACTIVE_ACCOUNT" ] || [ "$ACTIVE_ACCOUNT" = "(unset)" ]; then
  echo "ERROR: no active gcloud account." >&2
  echo "" >&2
  echo "  Your authenticated accounts:" >&2
  gcloud auth list --format="table(account,status)" >&2 2>/dev/null || true
  echo "" >&2
  echo "  Activate one with:" >&2
  echo "      gcloud config set account your-email@example.com" >&2
  echo "  or log in with:" >&2
  echo "      gcloud auth login" >&2
  exit 1
fi

# Final check: the ID must be reachable by this account.
# Uses `projects list` rather than `describe` for the reason above.
if ! gcloud projects list --filter="projectId=${PROJECT_ID}" \
     --format="value(projectId)" 2>/dev/null | grep -qx "$PROJECT_ID"; then
  echo "ERROR: project '$PROJECT_ID' is not reachable by $ACTIVE_ACCOUNT." >&2
  echo "  Your projects:" >&2
  gcloud projects list --format="table(projectNumber,projectId,name)" >&2 2>/dev/null || true
  exit 1
fi

# ─────────────────────────────────────────────────────────────────────
#  LOCATE THE REPO
#
#  Resolved from this script's own location, so it works no matter which
#  directory you invoke it from. Expects to live in scripts/.
# ─────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
ENV_PROD="$REPO_ROOT/.env.production"
API_DIR="$REPO_ROOT/api"

echo "═══════════════════════════════════════════════════════════"
echo "  Account:   $ACTIVE_ACCOUNT"
echo "  Project:   $PROJECT_ID"
echo "  Region:    $REGION"
echo "  Service:   $SERVICE"
echo "  Repo root: $REPO_ROOT"
echo "═══════════════════════════════════════════════════════════"
echo

# ─────────────────────────────────────────────────────────────────────
#  LOAD LOCAL .env TO GET THE SECRETS
# ─────────────────────────────────────────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: $ENV_FILE not found." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

# GUARD: if the URLs are single-quoted or unquoted with "&" in them, sourcing
# can silently leave these empty. Check before deploying something broken.
for var in DATABASE_URL JWT_SECRET REFERRAL_TOKEN_ENCRYPTION_KEY; do
  value="${!var:-}"
  if [ -z "$value" ]; then
    echo "ERROR: $var is empty after loading $ENV_FILE" >&2
    echo "  Are the URLs double-quoted in .env? See docs/00-ARCHITECTURE.md §3.7" >&2
    exit 1
  fi
  echo "  loaded $var (${#value} chars)"
done
echo

# ─────────────────────────────────────────────────────────────────────
#  BUILD THE PRODUCTION ENV FILE
#
#  --env-vars-file REPLACES the entire environment, so this file must be
#  complete. It is gitignored, and it keeps secrets out of the command line
#  and therefore out of your shell history.
# ─────────────────────────────────────────────────────────────────────
echo ">> writing $ENV_PROD"

grep -v '^MIGRATION_DATABASE_URL=' "$ENV_FILE" > "$ENV_PROD"

sed -i \
  -e 's|^APP_ENV=.*|APP_ENV=production|' \
  -e 's|^LOG_LEVEL=.*|LOG_LEVEL=info|' \
  -e 's|^PORT=.*|PORT=8080|' \
  -e 's|^TRUST_PROXY=.*|TRUST_PROXY=true|' \
  -e 's|^ALLOWED_ORIGINS=.*|ALLOWED_ORIGINS=http://localhost:5173|' \
  "$ENV_PROD"

# These keys have no production meaning; make sure they did not survive.
if grep -q '^MIGRATION_DATABASE_URL=' "$ENV_PROD"; then
  echo "ERROR: MIGRATION_DATABASE_URL leaked into $ENV_PROD" >&2
  exit 1
fi
if ! grep -q '^APP_ENV=production' "$ENV_PROD"; then
  echo "ERROR: APP_ENV was not overridden to production in $ENV_PROD" >&2
  exit 1
fi

# Never commit this file. Fail loudly rather than leak a credential.
if command -v git >/dev/null 2>&1 && git -C "$REPO_ROOT" check-ignore -q "$ENV_PROD"; then
  echo "   ignored by git ✓"
else
  echo "ERROR: $ENV_PROD is NOT gitignored. Refusing to continue." >&2
  exit 1
fi
echo

# ─────────────────────────────────────────────────────────────────────
#  ENABLE APIS (idempotent)
# ─────────────────────────────────────────────────────────────────────
echo ">> enabling APIs"
gcloud services enable \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  --project "$PROJECT_ID" --quiet
echo

# ─────────────────────────────────────────────────────────────────────
#  DEPLOY
#
#  --source . requires running from api/, because it uploads the CURRENT
#  DIRECTORY and needs Dockerfile + go.mod at its top level.
# ─────────────────────────────────────────────────────────────────────
echo ">> deploying (this builds in Cloud Build, expect 2-5 minutes)"
cd "$API_DIR"

gcloud run deploy "$SERVICE" \
  --source . \
  --project "$PROJECT_ID" \
  --region "$REGION" \
  --allow-unauthenticated \
  --max-instances 5 \
  --min-instances 0 \
  --memory 512Mi \
  --cpu 1 \
  --env-vars-file "$ENV_PROD"
echo

# ─────────────────────────────────────────────────────────────────────
#  VERIFY
# ─────────────────────────────────────────────────────────────────────
API_URL="$(gcloud run services describe "$SERVICE" \
  --project "$PROJECT_ID" --region "$REGION" --format="value(status.url)")"

echo "═══════════════════════════════════════════════════════════"
echo "  Service URL: $API_URL"
echo "═══════════════════════════════════════════════════════════"
echo

echo ">> /health"
curl -sS --max-time 30 "$API_URL/health" || true
echo; echo

echo ">> /ready   (this pings Neon)"
curl -sS --max-time 30 "$API_URL/ready" || true
echo; echo

echo ">> /api/v1/meta   (facilityCount MUST be 1)"
curl -sS --max-time 30 "$API_URL/api/v1/meta" || true
echo; echo

echo "───────────────────────────────────────────────────────────"
echo "Next: add this to Cloudflare Pages as VITE_API_URL:"
echo "    ${API_URL}/api/v1"
echo "and as VITE_API_ORIGIN:"
echo "    'self' ${API_URL}"
echo "───────────────────────────────────────────────────────────"
