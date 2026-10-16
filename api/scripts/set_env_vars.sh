#!/bin/bash

# 1. Get the absolute directory where THIS script lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 2. Point to the .env file located one level up from the script
ENV_PATH="$SCRIPT_DIR/../../.env"

# 3. Check if the file exists, then load it
if [ -f "$ENV_PATH" ]; then
    # Automatically export all variables loaded from the file
    set -a
    . "$ENV_PATH"
    set +a
else
    echo "ERROR: .env file not found at $ENV_PATH"
    exit 1
fi

# # ── DATABASE_URL: the POOLED endpoint (host contains "-pooler") ──
# printf '%s' "$DATABASE_URL" | gcloud secrets create phc-database-url \
#   --data-file=- --replication-policy=automatic --project "$PROJECT_ID"

# # ── JWT_SECRET: already set in .env ──
# printf '%s' "$JWT_SECRET" | gcloud secrets create phc-jwt-secret \
#   --data-file=- --replication-policy=automatic --project "$PROJECT_ID"

# # ── REFERRAL_TOKEN_ENCRYPTION_KEY: already set in .env ──
# printf '%s' "$REFERRAL_TOKEN_ENCRYPTION_KEY" | gcloud secrets create phc-referral-key \
#   --data-file=- --replication-policy=automatic --project "$PROJECT_ID"
