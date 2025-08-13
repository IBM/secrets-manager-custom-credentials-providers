#!/bin/bash

if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <client_id> <client_secret> <redirect_uri> <code>"
  exit 1
fi

CLIENT_ID="$1"
CLIENT_SECRET="$2"
REDIRECT_URI="$3"
CODE="$4"

response=$(curl -s -X POST https://slack.com/api/oauth.v2.access \
  -d client_id="$CLIENT_ID" \
  -d client_secret="$CLIENT_SECRET" \
  -d code="$CODE" \
  -d redirect_uri="$REDIRECT_URI")

echo "Slack Response:"
echo "$response" | jq

OK=$(echo "$response" | jq -r '.ok')
if [ "$OK" == "true" ]; then
  ACCESS_TOKEN=$(echo "$response" | jq -r '.authed_user.access_token')
  REFRESH_TOKEN=$(echo "$response" | jq -r '.authed_user.refresh_token')

  echo ""
  echo "Access Token: $ACCESS_TOKEN"
  echo "Refresh Token: $REFRESH_TOKEN"
  echo "Credential for secrets manager: {\"client_id\": \"$CLIENT_ID\", \"client_secret\": \"$CLIENT_SECRET\", \"refresh_token\": \"$REFRESH_TOKEN\"}"
else
  echo ""
  echo "Failed to exchange code. Error:"
  echo "$response" | jq -r '.error'
fi
