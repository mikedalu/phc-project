#!/bin/bash
set -euo pipefail

# 1. Resolve repository directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
API_DIR="$ROOT_DIR/api"
ENV_FILE="$ROOT_DIR/.env.production"
YAML_FILE="/tmp/phc-env-vars.yaml"

cd "$API_DIR" || exit 1

if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: $ENV_FILE not found at $ENV_FILE"
    exit 1
fi

# Load variables (like PROJECT_ID)
set -a
. "$ENV_FILE"
set +a

# Strip comments and blank lines, then convert KEY=VALUE → KEY: "VALUE"
# Handles values with commas, spaces, and special characters safely.
grep -v '^\s*#' "$ENV_FILE" | grep -v '^\s*$' | while IFS='=' read -r key value; do
    # Remove surrounding quotes from the value if present
    value="${value%\"}"
    value="${value#\"}"
    printf '%s: "%s"\n' "$key" "$value"
done > "$YAML_FILE"

echo "── Generated env-vars YAML ──"
cat "$YAML_FILE"
echo "─────────────────────────────"

# 3. Deploy to Cloud Run
gcloud run deploy phc-api \
  --source . \
  --project "${PROJECT_ID:-senforge-emr}" \
  --region europe-west1 \
  --allow-unauthenticated \
  --max-instances 1 \
  --min-instances 0 \
  --memory 512Mi \
  --cpu 1 \
  --env-vars-file "$YAML_FILE"

