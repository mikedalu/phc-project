#!/bin/bash

# 1. Navigate to the API folder
cd "/home/agent-mike/deepseek-harness/Default workspace/api" || exit 1

# 2. Check and parse the standard .env file into a comma-separated list
ENV_FILE="../.env.production"
if [ -f "$ENV_FILE" ]; then
    # Removes comments, empty lines, and joins lines with commas
    ENV_VARS=$(grep -v '^#' "$ENV_FILE" | grep -v '^$' | xargs | tr ' ' ',')
else
    echo "ERROR: $ENV_FILE not found."
    exit 1
fi

# 3. Deploy to Cloud Run using --update-env-vars instead
gcloud run deploy phc-api \
  --source . \
  --project "$PROJECT_ID" \
  --region europe-west1 \
  --allow-unauthenticated \
  --max-instances 1 \
  --min-instances 0 \
  --memory 512Mi \
  --cpu 1 \
  --update-env-vars "$ENV_VARS"
